package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

func (s *Server) handleListClaims(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	q := r.URL.Query()

	f := store.ClaimFilter{
		StoreID:   u.StoreID,
		MachineID: q.Get("machineId"),
		Cursor:    q.Get("cursor"),
	}
	for _, raw := range q["status"] {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			st, err := domain.ParseStatus(part)
			if err != nil {
				badRequest(w, "알 수 없는 상태입니다: "+part)
				return
			}
			f.Statuses = append(f.Statuses, st)
		}
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			badRequest(w, "limit은 숫자여야 합니다.")
			return
		}
		f.Limit = n
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			badRequest(w, "from은 RFC3339 형식이어야 합니다.")
			return
		}
		f.From = t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			badRequest(w, "to는 RFC3339 형식이어야 합니다.")
			return
		}
		f.To = t
	}

	claims, next, err := s.store.ListClaims(r.Context(), f)
	if err != nil {
		internalError(w, r, err, "신고 목록 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"claims": claims, "nextCursor": next})
}

// claimDetailResponse는 사장님 상세 화면이 받는 정보다.
// 계좌와 전화번호가 평문으로 나가고, 이 조회는 감사 로그에 남는다.
type claimDetailResponse struct {
	ID           string              `json:"id"`
	ReceiptCode  string              `json:"receiptCode"`
	MachineID    string              `json:"machineId"`
	MachineLabel string              `json:"machineLabel"`
	MachineCode  string              `json:"machineCode"`
	IssueType    domain.IssueType    `json:"issueType"`
	IssueLabel   string              `json:"issueLabel"`
	AmountKRW    int                 `json:"amountKrw"`
	Description  string              `json:"description"`
	Status       domain.Status       `json:"status"`
	StatusLabel  string              `json:"statusLabel"`
	RiskScore    int                 `json:"riskScore"`
	RiskReasons  []domain.RiskReason `json:"riskReasons"`

	Phone     string `json:"phone"`
	BankCode  string `json:"bankCode"`
	BankName  string `json:"bankName"`
	AccountNo string `json:"accountNo"`
	Holder    string `json:"holder"`

	// 통화 버튼을 주요 동작으로 띄울지. 고액 건은 전화로 확인하는 게
	// 사진 몇 장보다 확실하다.
	CallRecommended bool `json:"callRecommended"`

	PhotoIDs []string        `json:"photoIds"`
	Events   []eventResponse `json:"events"`

	PhoneClaims30d    int `json:"phoneClaims30d"`
	PhonePaidTotal30d int `json:"phonePaidTotal30d"`

	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt"`
	PaidAt     *time.Time `json:"paidAt"`

	AllowedTransitions []string `json:"allowedTransitions"`
}

type eventResponse struct {
	ActorKind string    `json:"actorKind"`
	Action    string    `json:"action"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Note      string    `json:"note"`
	At        time.Time `json:"at"`
}

func (s *Server) handleClaimDetail(w http.ResponseWriter, r *http.Request) {
	a := accessOf(r.Context())

	d, err := s.store.ClaimByID(r.Context(), a.storeID, r.PathValue("id"), a.actor)
	if errors.Is(err, store.ErrNotFound) {
		// 남의 매장 건도 404다. 403을 주면 "그 ID는 존재한다"가 새어 나간다.
		notFound(w, "신고 건을 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "신고 상세 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, s.claimDetailResponse(d))
}

func (s *Server) claimDetailResponse(d store.ClaimDetail) claimDetailResponse {
	bankName := d.BankCode
	if b, ok := payout.BankByCode(d.BankCode); ok {
		bankName = b.Name
	}

	photoIDs := make([]string, 0, len(d.Photos))
	for _, p := range d.Photos {
		photoIDs = append(photoIDs, p.ID)
	}

	events := make([]eventResponse, 0, len(d.Events))
	for _, e := range d.Events {
		events = append(events, eventResponse{
			ActorKind: e.Actor.Kind, Action: e.Action,
			From: string(e.From), To: string(e.To), Note: e.Note, At: e.At,
		})
	}

	c := domain.Claim{Status: d.Status}
	allowed := []string{}
	for _, st := range domain.AllStatuses() {
		if c.CanTransition(st) {
			allowed = append(allowed, string(st))
		}
	}

	return claimDetailResponse{
		ID: d.ID, ReceiptCode: receiptCode(d.ID),
		MachineID: d.MachineID, MachineLabel: d.MachineLabel, MachineCode: d.MachineCode,
		IssueType: d.IssueType, IssueLabel: d.IssueType.Label(),
		AmountKRW: d.AmountKRW, Description: d.Description,
		Status: d.Status, StatusLabel: d.Status.Label(),
		RiskScore: d.RiskScore, RiskReasons: d.RiskReasons,
		Phone: d.Phone, BankCode: d.BankCode, BankName: bankName,
		AccountNo: d.Account, Holder: d.Holder,
		CallRecommended:    domain.RequiresPhoto(d.AmountKRW, s.policy),
		PhotoIDs:           photoIDs,
		Events:             events,
		PhoneClaims30d:     d.PhoneClaims30d,
		PhonePaidTotal30d:  d.PhonePaidTotal30d,
		CreatedAt:          d.CreatedAt,
		ResolvedAt:         nilTime(d.ResolvedAt),
		PaidAt:             nilTime(d.PaidAt),
		AllowedTransitions: allowed,
	}
}

func nilTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type transitionRequest struct {
	Note   string `json:"note"`
	Method string `json:"method"` // mark-paid 전용: deeplink | manual
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, domain.StatusApproved, nil)
}

func (s *Server) handleReject(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, domain.StatusRejected, nil)
}

func (s *Server) handleMarkPaid(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, domain.StatusPaid, func(c *domain.Claim, req transitionRequest) error {
		// 어떤 경로로 보냈는지 반드시 기록한다. 나중에 "이 건 진짜 보냈나"를
		// 확인할 때 근거가 되고, 지급대행 연동 후에는 대사 대상이 된다.
		switch req.Method {
		case domain.PayoutDeeplink, domain.PayoutManual:
			c.PayoutMethod = req.Method
			return nil
		default:
			return errors.New("송금 방법을 알려주세요 (deeplink 또는 manual)")
		}
	})
}

func (s *Server) transition(w http.ResponseWriter, r *http.Request, to domain.Status,
	extra func(*domain.Claim, transitionRequest) error,
) {
	a := accessOf(r.Context())

	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	d, err := s.store.ClaimByID(r.Context(), a.storeID, r.PathValue("id"), domain.Actor{})
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "신고 건을 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "신고 조회 실패")
		return
	}

	// 상태 전이는 반드시 도메인을 거친다. 핸들러가 상태를 직접 쓰면
	// 허용 표를 우회하게 되고, 그 순간 paid 를 되돌릴 수 있게 된다.
	c := d.Claim
	if extra != nil {
		if err := extra(&c, req); err != nil {
			badRequest(w, err.Error())
			return
		}
	}

	ev, err := c.Transition(to, a.actor, strings.TrimSpace(req.Note))
	if err != nil {
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
		return
	}

	if err := s.store.ApplyTransition(r.Context(), a.storeID, &c, ev); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusConflict, "stale",
				"다른 곳에서 이미 처리된 건입니다. 새로고침 후 다시 확인해주세요.")
			return
		}
		internalError(w, r, err, "상태 변경 실패")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id": c.ID, "status": c.Status, "statusLabel": c.Status.Label(),
	})
}

// payoutLinksResponse는 송금 시트가 받는 정보다.
type payoutLinksResponse struct {
	Links []payout.Link `json:"links"`
	// 딥링크가 없거나 PC에서 열었을 때를 위한 수동 경로. 항상 함께 준다.
	BankName  string `json:"bankName"`
	AccountNo string `json:"accountNo"`
	Holder    string `json:"holder"`
	AmountKRW int    `json:"amountKrw"`
	CopyText  string `json:"copyText"`
}

func (s *Server) handlePayoutLinks(w http.ResponseWriter, r *http.Request) {
	a := accessOf(r.Context())

	d, err := s.store.ClaimByID(r.Context(), a.storeID, r.PathValue("id"), a.actor)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "신고 건을 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "신고 조회 실패")
		return
	}
	if d.Status != domain.StatusApproved {
		writeError(w, http.StatusConflict, "not_approved", "승인된 건만 송금할 수 있습니다.")
		return
	}

	links, err := s.payout.Links(payout.Request{
		BankCode: d.BankCode, AccountNo: d.Account, Holder: d.Holder, AmountKRW: d.AmountKRW,
	})
	if err != nil {
		// 딥링크를 못 만들어도 수동 경로는 줘야 한다. 사장님이 계좌를
		// 복사해서 직접 보내면 되고, 그게 이 화면의 최소 기능이다.
		links = []payout.Link{}
	}

	bankName := d.BankCode
	if b, ok := payout.BankByCode(d.BankCode); ok {
		bankName = b.Name
	}

	writeJSON(w, http.StatusOK, payoutLinksResponse{
		Links: links, BankName: bankName, AccountNo: d.Account,
		Holder: d.Holder, AmountKRW: d.AmountKRW,
		CopyText: bankName + " " + d.Account + " " + d.Holder,
	})
}

// handlePhoto는 첨부 사진을 내려준다.
//
// 경로가 /claims/{id}/photos/{photoId} 인 이유: claim id 가 경로에 있어야
// 링크 토큰을 그 건에 묶어 검증할 수 있다. 사진 id 만으로는 어느 건의
// 것인지 모르므로 토큰을 확인할 방법이 없다.
func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	a := accessOf(r.Context())

	d, err := s.store.ClaimByID(r.Context(), a.storeID, r.PathValue("id"), domain.Actor{})
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "사진을 찾을 수 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "신고 조회 실패")
		return
	}

	photoID := r.PathValue("photoId")
	var key string
	for _, p := range d.Photos {
		if p.ID == photoID {
			key = p.ObjectKey
			break
		}
	}
	if key == "" {
		notFound(w, "사진을 찾을 수 없습니다.")
		return
	}

	rc, contentType, err := s.media.Open(r.Context(), key)
	if err != nil {
		notFound(w, "사진 파일이 없습니다. 보존 기간이 지났을 수 있습니다.")
		return
	}
	defer rc.Close() //nolint:errcheck

	w.Header().Set("Content-Type", contentType)
	// 개인정보가 담긴 이미지다. 중간 캐시에 남으면 안 된다.
	w.Header().Set("Cache-Control", "private, no-store")
	// 브라우저가 내용을 다시 추측해서 실행 가능한 타입으로 해석하지 않게 한다.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, rc); err != nil {
		// 헤더는 이미 나갔다. 사장님이 새로고침하면 된다.
		return
	}
}
