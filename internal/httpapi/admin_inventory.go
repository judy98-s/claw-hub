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
	"github.com/judy98-s/claw-hub/internal/store"
)

// inventoryResponse는 재고 화면이 한 번에 받는 전부다.
//
// 목록과 요약을 따로 내려받게 하면 요청이 두 번 나가고, 그 사이에 등록이
// 끼면 "12종"과 13줄짜리 목록이 같이 뜬다.
type inventoryResponse struct {
	Summary store.InventorySummary `json:"summary"`
	Items   []store.InventoryRow   `json:"items"`
	// SpentFrom/To는 요약의 지출이 어느 기간인지다. 화면이 "이번 달"을
	// 자기 시계로 다시 계산하면 서버와 한 달 경계에서 어긋난다.
	SpentFrom time.Time `json:"spentFrom"`
	SpentTo   time.Time `json:"spentTo"`
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	from, to := monthRange(s.now())

	sum, err := s.store.InventorySummaryFor(r.Context(), u.StoreID, from, to)
	if err != nil {
		internalError(w, r, err, "재고 요약 조회 실패")
		return
	}
	items, err := s.store.InventoryFor(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "재고 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, inventoryResponse{
		Summary: sum, Items: items, SpentFrom: from, SpentTo: to,
	})
}

// monthRange는 "이번 달"의 경계다. 매장이 쓰는 시계 기준이다.
func monthRange(now time.Time) (time.Time, time.Time) {
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return from, from.AddDate(0, 1, 0)
}

// inventoryItemResponse는 한 품목의 상세다.
type inventoryItemResponse struct {
	store.InventoryRow
	Purchases   []store.Purchase   `json:"purchases"`
	Adjustments []store.Adjustment `json:"adjustments"`
}

func (s *Server) handleInventoryItem(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	key := r.PathValue("nameKey")

	item, err := s.store.InventoryItem(r.Context(), u.StoreID, key)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이 품목의 기록이 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "품목 조회 실패")
		return
	}
	purchases, err := s.store.ListPurchases(r.Context(), u.StoreID, key, 50)
	if err != nil {
		internalError(w, r, err, "사입 이력 조회 실패")
		return
	}
	adjustments, err := s.store.AdjustmentsFor(r.Context(), u.StoreID, key)
	if err != nil {
		internalError(w, r, err, "실사 이력 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, inventoryItemResponse{
		InventoryRow: item, Purchases: purchases, Adjustments: adjustments,
	})
}

func (s *Server) handleListPurchases(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			badRequest(w, "limit은 숫자여야 합니다.")
			return
		}
		limit = n
	}
	// 화면이 넘기는 값은 표시용 이름일 수도, 키일 수도 있다. 어느 쪽이든
	// 같은 키로 정규화해서 찾는다 — 화면이 키 만드는 법을 알 필요는 없다.
	key := ""
	if raw := r.URL.Query().Get("name"); raw != "" {
		key = domain.NameKey(raw)
	}

	list, err := s.store.ListPurchases(r.Context(), u.StoreID, key, limit)
	if err != nil {
		internalError(w, r, err, "사입 이력 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"purchases": list})
}

// createPurchaseResponse는 등록 결과다.
//
// PreviousUnitCostKrw는 같은 인형의 직전 실단가다. "지난번보다 130원
// 비싸게 샀다"는 걸 등록 직후에 알려주는 게, 나중에 비교 화면에서
// 알려주는 것보다 훨씬 쓸모 있다.
type createPurchaseResponse struct {
	store.Purchase
	Existing            bool `json:"existing"`
	PreviousUnitCostKrw int  `json:"previousUnitCostKrw"`
	QtyOnHand           int  `json:"qtyOnHand"`
}

func (s *Server) handleCreatePurchase(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	ctx := r.Context()

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" || len(idemKey) > 200 {
		badRequest(w, "요청 식별자가 없습니다. 페이지를 새로고침한 뒤 다시 시도해주세요.")
		return
	}

	if err := r.ParseMultipartForm(4 << 20); err != nil {
		badRequest(w, "첨부한 사진이 너무 크거나 형식이 올바르지 않습니다.")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck

	in, ok := s.parsePurchaseForm(w, r)
	if !ok {
		return
	}

	// 등록 전에 직전 단가를 읽어둔다. 등록 뒤에 읽으면 방금 넣은 건이
	// "직전"이 되어 자기 자신과 비교하게 된다.
	prev := 0
	if before, err := s.store.ListPurchases(ctx, u.StoreID, domain.NameKey(in.Name), 1); err == nil && len(before) > 0 {
		prev = before[0].UnitCostKRW
	}

	photos, msg := collectPhotos(r)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	if len(photos) > 1 {
		badRequest(w, "영수증은 한 장만 첨부할 수 있습니다.")
		return
	}
	written, err := s.writePhotos(ctx, photos)
	if err != nil {
		s.cleanupPhotos(ctx, written)
		internalError(w, r, err, "영수증 저장 실패")
		return
	}
	if len(written) == 1 {
		in.ReceiptKey = written[0].key
	}

	in.StoreID, in.UserID, in.IdempotencyKey = u.StoreID, u.ID, idemKey
	p, existing, err := s.store.CreatePurchase(ctx, in)
	if err != nil {
		s.cleanupPhotos(ctx, written)
		internalError(w, r, err, "사입 저장 실패")
		return
	}
	if existing {
		// 같은 멱등키의 재시도다. 방금 쓴 영수증은 고아이므로 지운다.
		s.cleanupPhotos(ctx, written)
	}

	res := createPurchaseResponse{Purchase: p, Existing: existing, PreviousUnitCostKrw: prev}
	if item, err := s.store.InventoryItem(ctx, u.StoreID, p.NameKey); err == nil {
		res.QtyOnHand = item.QtyOnHand
	}

	status := http.StatusCreated
	if existing {
		status = http.StatusOK
	}
	writeJSON(w, status, res)
}

// parsePurchaseForm은 폼을 읽고 도메인 규칙으로 검증한다.
func (s *Server) parsePurchaseForm(w http.ResponseWriter, r *http.Request) (store.CreatePurchaseInput, bool) {
	var in store.CreatePurchaseInput

	in.Name = strings.TrimSpace(r.FormValue("name"))
	in.Vendor = strings.TrimSpace(r.FormValue("vendor"))

	num := func(field, label string) (int, bool) {
		raw := strings.TrimSpace(r.FormValue(field))
		if raw == "" {
			return 0, true // 빈 값은 0. 도메인 검증이 0을 허용할지 정한다.
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			badRequest(w, label+"을(를) 숫자로 입력해주세요.")
			return 0, false
		}
		return n, true
	}

	var ok bool
	if in.UnitPriceKRW, ok = num("unitPriceKrw", "단가"); !ok {
		return in, false
	}
	if in.Qty, ok = num("qty", "수량"); !ok {
		return in, false
	}
	if in.ShippingKRW, ok = num("shippingKrw", "배송비"); !ok {
		return in, false
	}

	// 날짜는 <input type="date"> 가 보내는 YYYY-MM-DD 다. 시각이 없으므로
	// 매장이 쓰는 시계의 자정으로 읽는다. UTC로 읽으면 한국의 9월 1일이
	// 8월 31일로 기록되고, 월별 지출이 한 칸씩 밀린다.
	raw := strings.TrimSpace(r.FormValue("purchasedAt"))
	if raw == "" {
		in.PurchasedAt = s.now()
	} else {
		t, err := time.ParseInLocation("2006-01-02", raw, kst)
		if err != nil {
			badRequest(w, "사입한 날짜를 다시 확인해주세요.")
			return in, false
		}
		in.PurchasedAt = t
	}

	if err := domain.ValidatePurchase(domain.PurchaseInput{
		Name: in.Name, Vendor: in.Vendor,
		UnitPriceKRW: in.UnitPriceKRW, Qty: in.Qty, ShippingKRW: in.ShippingKRW,
		PurchasedAt: in.PurchasedAt,
	}, s.now()); err != nil {
		badRequest(w, err.Error())
		return in, false
	}
	return in, true
}

type countRequest struct {
	CountedQty int    `json:"countedQty"`
	Note       string `json:"note"`
}

// handleCountInventory는 실사 결과를 기록한다.
func (s *Server) handleCountInventory(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req countRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	if err := domain.ValidateAdjustment(req.CountedQty); err != nil {
		badRequest(w, err.Error())
		return
	}
	if len([]rune(req.Note)) > 200 {
		badRequest(w, "메모가 너무 깁니다. 200자 이내로 적어주세요.")
		return
	}

	key := r.PathValue("nameKey")
	err := s.store.AdjustInventory(r.Context(), u.StoreID, u.ID, key, req.CountedQty, req.Note)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이 품목의 기록이 없습니다.")
		return
	}
	if err != nil {
		internalError(w, r, err, "재고 조정 실패")
		return
	}

	item, err := s.store.InventoryItem(r.Context(), u.StoreID, key)
	if err != nil {
		internalError(w, r, err, "재고 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// handleSuggestions는 사입 시트의 자동완성 후보를 준다.
//
// 인형 이름과 거래처를 한 번에 돌려준다. 시트를 열 때 두 번 요청하면
// 느린 회선에서 한쪽만 먼저 뜨고, 사장님은 나머지가 고장난 줄 안다.
func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	q := r.URL.Query().Get("q")

	names, err := s.store.DollNameSuggestions(r.Context(), u.StoreID, q, 8)
	if err != nil {
		internalError(w, r, err, "인형 이름 조회 실패")
		return
	}
	vendors, err := s.store.VendorSuggestions(r.Context(), u.StoreID, 6)
	if err != nil {
		internalError(w, r, err, "거래처 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"names": names, "vendors": vendors})
}
