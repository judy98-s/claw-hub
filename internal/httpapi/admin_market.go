package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

// contactsPerDay는 한 매장이 하루에 열어볼 수 있는 번호 수다.
//
// 거래하러 온 사장님은 하루에 몇 건이면 충분하다. 이 선을 넘는 건
// 목록을 훑으며 번호를 긁는 쪽이다. 상한에 걸려도 목록은 계속 보인다 —
// 장터를 못 쓰게 만드는 게 아니라 수집을 막는 게 목적이다.
const contactsPerDay = 30

func (s *Server) handleListMarket(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	q := r.URL.Query()

	f := store.ListingFilter{
		ViewerStoreID: u.StoreID,
		RegionCode:    strings.TrimSpace(q.Get("region")),
		Now:           s.nowKST(),
	}
	if f.RegionCode != "" {
		if _, ok := domain.RegionByCode(f.RegionCode); !ok {
			badRequest(w, "지역을 다시 선택해주세요.")
			return
		}
	}
	if raw := strings.TrimSpace(q.Get("kind")); raw != "" {
		kind, err := domain.ParseListingKind(raw)
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		f.Kind = kind
	}

	list, err := s.store.ListListings(r.Context(), f)
	if err != nil {
		internalError(w, r, err, "장터 목록 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"listings": list,
		"regions":  domain.Regions(),
	})
}

// handleMyListings는 내 글을 전부 보여준다. 내린 글과 만료된 글도 함께다.
func (s *Server) handleMyListings(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	list, err := s.store.ListListings(r.Context(), store.ListingFilter{
		ViewerStoreID: u.StoreID, MineOnly: true, Now: s.nowKST(),
	})
	if err != nil {
		internalError(w, r, err, "내 글 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": list})
}

func (s *Server) handleMarketDetail(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	l, err := s.store.ListingByID(r.Context(), r.PathValue("id"), u.StoreID, s.nowKST())
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이미 내려간 글입니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "장터 글 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

type createListingRequest struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Qty          int    `json:"qty"`
	UnitPriceKrw int    `json:"unitPriceKrw"`
	Note         string `json:"note"`
}

func (s *Server) handleCreateListing(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req createListingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	// 게시 조건을 먼저 본다. 입력을 다 받아놓고 마지막에 막으면,
	// 사장님은 적은 걸 다 날리고 설정 화면으로 간다.
	st, err := s.store.StoreByID(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "매장 조회 실패")
		return
	}
	if !st.CanPostListing() {
		writeError(w, http.StatusConflict, "store_incomplete",
			"장터에 글을 쓰려면 설정에서 매장 지역과 사업자등록번호를 먼저 등록해주세요.")
		return
	}

	kind, err := domain.ParseListingKind(req.Kind)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	in := domain.ListingInput{
		Name: strings.TrimSpace(req.Name), Kind: kind,
		Qty: req.Qty, UnitPriceKRW: req.UnitPriceKrw,
		Note: strings.TrimSpace(req.Note),
	}
	if err := domain.ValidateListing(in); err != nil {
		badRequest(w, err.Error())
		return
	}

	l, err := s.store.CreateListing(r.Context(), store.CreateListingInput{
		StoreID: u.StoreID, UserID: u.ID,
		Name: in.Name, Kind: in.Kind, Qty: in.Qty,
		UnitPriceKRW: in.UnitPriceKRW, Note: in.Note,
		RegionCode: st.RegionCode, RegionDetail: st.RegionDetail,
		Now: s.nowKST(),
	})
	if err != nil {
		internalError(w, r, err, "장터 글 저장 실패")
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleCloseListing(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	err := s.store.CloseListing(r.Context(), r.PathValue("id"), u.StoreID)
	if errors.Is(err, store.ErrNotFound) {
		// 남의 글이든 이미 내려간 글이든 같은 응답이다. 구분해서 알려주면
		// "그 글은 존재하며 내 것이 아니다"가 새어 나간다.
		notFound(w, "내릴 수 있는 글이 아닙니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "장터 글 내리기 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "closed"})
}

// contactResponse는 연락처다. 이 응답으로만 번호가 나간다.
type contactResponse struct {
	StoreName string `json:"storeName"`
	Phone     string `json:"phone"`
	BizNo     string `json:"bizNo"`
	// RemainingToday는 오늘 몇 번 더 볼 수 있는지다. 상한에 갑자기
	// 걸리는 것보다 남은 횟수를 아는 게 낫다.
	RemainingToday int `json:"remainingToday"`
}

func (s *Server) handleListingContact(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	now := s.nowKST()

	l, err := s.store.ListingByID(r.Context(), r.PathValue("id"), u.StoreID, now)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이미 내려간 글입니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "장터 글 조회 실패")
		return
	}
	if l.Mine {
		// 자기 번호를 자기가 열람할 이유가 없고, 그 기록이 남으면
		// 감사 로그가 쓰레기가 된다.
		writeError(w, http.StatusForbidden, "own_listing", "내 글입니다.")
		return
	}

	used, err := s.store.ContactsTodayFor(r.Context(), u.StoreID, now)
	if err != nil {
		internalError(w, r, err, "연락처 열람 집계 실패")
		return
	}
	if used >= contactsPerDay {
		writeError(w, http.StatusTooManyRequests, "contact_limit",
			"오늘 연락처를 너무 많이 열어보셨습니다. 내일 다시 시도해주세요.")
		return
	}

	c, err := s.store.ContactFor(r.Context(), l.ID, u.StoreID, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이미 내려간 글입니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "연락처 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, contactResponse{
		StoreName: c.StoreName, Phone: c.Phone,
		BizNo:          domain.FormatBizNo(c.BizNo),
		RemainingToday: contactsPerDay - used - 1,
	})
}

func (s *Server) handleReportListing(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	if len([]rune(req.Reason)) > 300 {
		badRequest(w, "신고 사유가 너무 깁니다. 300자 이내로 적어주세요.")
		return
	}

	l, err := s.store.ListingByID(r.Context(), r.PathValue("id"), u.StoreID, s.nowKST())
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이미 내려간 글입니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "장터 글 조회 실패")
		return
	}
	if l.Mine {
		badRequest(w, "내 글은 신고할 수 없습니다. 내리시려면 '내리기'를 눌러주세요.")
		return
	}

	removed, err := s.store.ReportListing(r.Context(), l.ID, u.StoreID, req.Reason)
	if err != nil {
		internalError(w, r, err, "신고 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}
