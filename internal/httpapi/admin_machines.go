package httpapi

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

type machineItem struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Label      string `json:"label"`
	Location   string `json:"location"`
	Active     bool   `json:"active"`
	Claims24h  int    `json:"claims24h"`
	Claims7d   int    `json:"claims7d"`
	OpenClaims int    `json:"openClaims"`
	Daily7d    []int  `json:"daily7d"`
	QRTarget   string `json:"qrTarget"`
	NeedsCheck bool   `json:"needsCheck"`
}

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	list, err := s.store.ListMachines(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "기계 목록 조회 실패")
		return
	}

	out := make([]machineItem, 0, len(list))
	for _, m := range list {
		out = append(out, machineItem{
			ID: m.ID, Code: m.Code, Label: m.Label, Location: m.Location, Active: m.Active,
			Claims24h: m.Claims24h, Claims7d: m.Claims7d, OpenClaims: m.OpenClaims,
			Daily7d:  m.Daily7d,
			QRTarget: s.qrTarget(m.Code),
			// 목록에서 바로 보이는 점검 신호. 알림을 놓쳤어도 여기서 잡힌다.
			NeedsCheck: m.Claims24h >= s.cfg.Policy.MachineAlertCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"machines": out})
}

func (s *Server) qrTarget(code string) string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/r/" + code
}

type machineRequest struct {
	Label    string `json:"label"`
	Location string `json:"location"`
	Active   *bool  `json:"active"`
}

func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req machineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if n := len([]rune(req.Label)); n < 1 || n > 50 {
		badRequest(w, "기계 이름을 입력해주세요 (50자 이내).")
		return
	}

	m, err := s.store.CreateMachine(r.Context(), u.StoreID, req.Label, strings.TrimSpace(req.Location))
	if err != nil {
		internalError(w, r, err, "기계 생성 실패")
		return
	}
	writeJSON(w, http.StatusCreated, machineItem{
		ID: m.ID, Code: m.Code, Label: m.Label, Location: m.Location,
		Active: m.Active, Daily7d: make([]int, 7), QRTarget: s.qrTarget(m.Code),
	})
}

func (s *Server) handleUpdateMachine(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	id := r.PathValue("id")

	current, err := s.store.MachineByID(r.Context(), u.StoreID, id)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "기계를 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "기계 조회 실패")
		return
	}

	var req machineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	// 보내지 않은 필드는 그대로 둔다. PATCH 이므로 부분 수정이다.
	label := current.Label
	if v := strings.TrimSpace(req.Label); v != "" {
		if len([]rune(v)) > 50 {
			badRequest(w, "기계 이름은 50자 이내여야 합니다.")
			return
		}
		label = v
	}
	location := current.Location
	if req.Location != "" {
		location = strings.TrimSpace(req.Location)
	}
	active := current.Active
	if req.Active != nil {
		active = *req.Active
	}

	if err := s.store.UpdateMachine(r.Context(), u.StoreID, id, label, location, active); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFound(w, "기계를 찾을 수 없습니다.")
			return
		}
		internalError(w, r, err, "기계 수정 실패")
		return
	}

	writeJSON(w, http.StatusOK, machineItem{
		ID: id, Code: current.Code, Label: label, Location: location,
		Active: active, Daily7d: make([]int, 7), QRTarget: s.qrTarget(current.Code),
	})
}

// handleMachineQR은 인쇄용 QR PNG를 내려준다.
func (s *Server) handleMachineQR(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	m, err := s.store.MachineByID(r.Context(), u.StoreID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "기계를 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "기계 조회 실패")
		return
	}

	// 오류 정정 수준 High. 스티커가 긁히거나 젖어도 읽혀야 한다 —
	// 기계에 붙은 스티커는 험하게 쓰인다.
	png, err := qrcode.Encode(s.qrTarget(m.Code), qrcode.High, 512)
	if err != nil {
		internalError(w, r, err, "QR 생성 실패")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", `inline; filename="qr-`+m.Code+`.png"`)
	w.Write(png) //nolint:errcheck
}

func (s *Server) handleListContacts(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	contacts, err := s.store.ListContacts(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "연락처 목록 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
}

type contactStatusRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
	Kind   string `json:"kind"` // phone | account. 생략하면 phone
}

func (s *Server) handleSetContactStatus(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	hash, err := hex.DecodeString(r.PathValue("hash"))
	if err != nil || len(hash) != 32 {
		badRequest(w, "연락처 식별자가 올바르지 않습니다.")
		return
	}

	var req contactStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	if _, err := domain.ParseContactStatus(req.Status); err != nil {
		badRequest(w, "상태는 normal, watch, blocked 중 하나여야 합니다.")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "phone"
	}

	if err := s.store.SetContactStatus(r.Context(), u.StoreID, hash, kind,
		req.Status, strings.TrimSpace(req.Note), u.ID); err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

func (s *Server) handleDailyStats(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	q := r.URL.Query()

	// 기본은 최근 30일. 대시보드가 파라미터 없이 바로 그릴 수 있어야 한다.
	to := s.now()
	from := to.AddDate(0, 0, -29)

	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			badRequest(w, "from은 YYYY-MM-DD 형식이어야 합니다.")
			return
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			badRequest(w, "to는 YYYY-MM-DD 형식이어야 합니다.")
			return
		}
		to = t
	}
	if to.Before(from) {
		badRequest(w, "종료일이 시작일보다 빠릅니다.")
		return
	}
	// 기간 상한이 없으면 누군가 10년치를 요청해 DB를 붙든다.
	if to.Sub(from) > 366*24*time.Hour {
		badRequest(w, "조회 기간은 1년 이내여야 합니다.")
		return
	}

	stats, err := s.store.DailyStats(r.Context(), u.StoreID, from, to)
	if err != nil {
		internalError(w, r, err, "일별 통계 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": stats})
}
