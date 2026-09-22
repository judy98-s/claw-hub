package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

// purchaseOpts는 사입 등록 폼이다.
type purchaseOpts struct {
	name, vendor string
	unitPrice    string
	qty          string
	shipping     string
	purchasedAt  string
	receipt      []byte
	omit         map[string]bool
}

func defaultPurchase() purchaseOpts {
	return purchaseOpts{
		name: "쿠로미 중형 30cm", vendor: "캐치돌",
		unitPrice: "2300", qty: "60", shipping: "3000",
		purchasedAt: "2026-09-14",
	}
}

func (h *harness) postPurchase(t *testing.T, o purchaseOpts, idemKey string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	set := func(k, v string) {
		if o.omit[k] {
			return
		}
		w.WriteField(k, v) //nolint:errcheck
	}
	set("name", o.name)
	set("vendor", o.vendor)
	set("unitPriceKrw", o.unitPrice)
	set("qty", o.qty)
	set("shippingKrw", o.shipping)
	set("purchasedAt", o.purchasedAt)
	if len(o.receipt) > 0 {
		fw, _ := w.CreateFormFile("photos", "receipt.jpg")
		fw.Write(o.receipt) //nolint:errcheck
	}
	w.Close() //nolint:errcheck

	req := httptest.NewRequest(http.MethodPost, "/api/admin/purchases", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func (h *harness) inventory(t *testing.T, c *http.Cookie) inventoryResponse {
	t.Helper()
	rec := h.do(http.MethodGet, "/api/admin/inventory", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res inventoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPurchase_등록하면_재고에_반영된다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.postPurchase(t, defaultPurchase(), "buy-1", c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res createPurchaseResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if res.UnitCostKRW != 2350 {
		t.Errorf("UnitCostKRW = %d, want 2350", res.UnitCostKRW)
	}
	if res.QtyOnHand != 60 {
		t.Errorf("QtyOnHand = %d, want 60", res.QtyOnHand)
	}

	inv := h.inventory(t, c)
	if len(inv.Items) != 1 || inv.Items[0].QtyOnHand != 60 {
		t.Fatalf("재고 = %+v", inv.Items)
	}
	if inv.Summary.ValueKRW != 141000 {
		t.Errorf("재고 자산 = %d, want 141000", inv.Summary.ValueKRW)
	}
}

func TestPurchase_지난번_단가를_알려준다(t *testing.T) {
	// "지난번보다 130원 비싸게 샀다"는 걸 등록 직후에 아는 게, 나중에
	// 비교 화면에서 아는 것보다 훨씬 쓸모 있다.
	h := newHarness(t)
	c := h.login(t)

	first := defaultPurchase()
	first.unitPrice = "2000"
	first.shipping = "0"
	if rec := h.postPurchase(t, first, "prev-1", c); rec.Code != http.StatusCreated {
		t.Fatalf("첫 등록 실패: %d %s", rec.Code, rec.Body)
	}

	second := defaultPurchase()
	second.purchasedAt = "2026-09-20"
	rec := h.postPurchase(t, second, "prev-2", c)
	var res createPurchaseResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck

	if res.PreviousUnitCostKrw != 2000 {
		t.Errorf("PreviousUnitCostKrw = %d, want 2000", res.PreviousUnitCostKrw)
	}
	// 방금 넣은 건이 "직전"이 되면 자기 자신과 비교하게 된다.
	if res.PreviousUnitCostKrw == res.UnitCostKRW {
		t.Error("직전 단가가 방금 등록한 건과 같다 — 자기 자신과 비교하고 있다")
	}
}

func TestPurchase_더블탭은_두_번_등록되지_않는다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	if rec := h.postPurchase(t, defaultPurchase(), "tap", c); rec.Code != http.StatusCreated {
		t.Fatalf("첫 등록 = %d", rec.Code)
	}
	rec := h.postPurchase(t, defaultPurchase(), "tap", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("재시도 status = %d, want 200", rec.Code)
	}
	var res createPurchaseResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if !res.Existing {
		t.Error("existing=false — 새로 등록됐다")
	}

	inv := h.inventory(t, c)
	if inv.Items[0].QtyOnHand != 60 {
		t.Errorf("수량 = %d, want 60 — 120이면 이중 등록이다", inv.Items[0].QtyOnHand)
	}
}

func TestPurchase_멱등키가_없으면_400(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	if rec := h.postPurchase(t, defaultPurchase(), "", c); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPurchase_입력_검증(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*purchaseOpts)
	}{
		{"이름 없음", func(p *purchaseOpts) { p.name = "  " }},
		{"이름 너무 김", func(p *purchaseOpts) { p.name = strings.Repeat("가", 81) }},
		{"수량 0", func(p *purchaseOpts) { p.qty = "0" }},
		{"수량 없음", func(p *purchaseOpts) { p.omit = map[string]bool{"qty": true} }},
		{"수량이 숫자가 아님", func(p *purchaseOpts) { p.qty = "예순개" }},
		{"수량 음수", func(p *purchaseOpts) { p.qty = "-5" }},
		{"단가 음수", func(p *purchaseOpts) { p.unitPrice = "-100" }},
		{"배송비 음수", func(p *purchaseOpts) { p.shipping = "-1" }},
		{"미래 날짜", func(p *purchaseOpts) { p.purchasedAt = "2030-01-01" }},
		{"날짜 형식 오류", func(p *purchaseOpts) { p.purchasedAt = "9월 14일" }},
		{"연도 오타", func(p *purchaseOpts) { p.purchasedAt = "1926-09-14" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			c := h.login(t)
			o := defaultPurchase()
			tc.mut(&o)

			rec := h.postPurchase(t, o, "v-1", c)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body = %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "message") {
				t.Errorf("사장님이 읽을 메시지가 없다: %s", rec.Body)
			}
		})
	}
}

func TestPurchase_배송비를_비우면_0으로_본다(t *testing.T) {
	// 배송비는 선택이다. 비웠다고 거부하면 대부분의 등록이 막힌다.
	h := newHarness(t)
	c := h.login(t)
	o := defaultPurchase()
	o.shipping = ""

	rec := h.postPurchase(t, o, "ship-0", c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res createPurchaseResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if res.UnitCostKRW != 2300 {
		t.Errorf("UnitCostKRW = %d, want 2300", res.UnitCostKRW)
	}
}

func TestPurchase_띄어쓰기가_달라도_한_줄로_합쳐진다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	a := defaultPurchase()
	b := defaultPurchase()
	b.name = "쿠로미중형30CM"
	b.purchasedAt = "2026-09-20"
	for i, o := range []purchaseOpts{a, b} {
		if rec := h.postPurchase(t, o, fmt.Sprintf("merge-%d", i), c); rec.Code != http.StatusCreated {
			t.Fatalf("등록 %d 실패: %d %s", i, rec.Code, rec.Body)
		}
	}

	inv := h.inventory(t, c)
	if len(inv.Items) != 1 {
		t.Fatalf("품목이 %d줄이다: %+v", len(inv.Items), inv.Items)
	}
	if inv.Items[0].QtyOnHand != 120 {
		t.Errorf("수량 = %d, want 120", inv.Items[0].QtyOnHand)
	}
}

func TestInventory_실사로_수량을_맞춘다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	h.postPurchase(t, defaultPurchase(), "count-1", c)

	key := domain.NameKey("쿠로미 중형 30cm")
	rec := h.do(http.MethodPost, "/api/admin/inventory/"+key+"/count", `{"countedQty":42,"note":"실사"}`, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var item store.InventoryRow
	json.Unmarshal(rec.Body.Bytes(), &item) //nolint:errcheck
	if item.QtyOnHand != 42 {
		t.Errorf("QtyOnHand = %d, want 42", item.QtyOnHand)
	}

	// 0개는 정상이다. 다 팔렸다는 것도 정보다.
	if rec := h.do(http.MethodPost, "/api/admin/inventory/"+key+"/count", `{"countedQty":0}`, c); rec.Code != http.StatusOK {
		t.Errorf("0개가 거부됐다: %d %s", rec.Code, rec.Body)
	}
	// 음수는 오타다.
	if rec := h.do(http.MethodPost, "/api/admin/inventory/"+key+"/count", `{"countedQty":-1}`, c); rec.Code != http.StatusBadRequest {
		t.Errorf("음수가 통과했다: %d", rec.Code)
	}
	// 없는 품목
	if rec := h.do(http.MethodPost, "/api/admin/inventory/없는품목/count", `{"countedQty":1}`, c); rec.Code != http.StatusNotFound {
		t.Errorf("없는 품목 status = %d, want 404", rec.Code)
	}
}

func TestInventoryItem_사입과_실사_이력이_함께_온다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	h.postPurchase(t, defaultPurchase(), "hist-1", c)
	key := domain.NameKey("쿠로미 중형 30cm")
	h.do(http.MethodPost, "/api/admin/inventory/"+key+"/count", `{"countedQty":50}`, c)

	rec := h.do(http.MethodGet, "/api/admin/inventory/"+key, "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res inventoryItemResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Purchases) != 1 || len(res.Adjustments) != 1 {
		t.Fatalf("이력 = 사입 %d건, 실사 %d건", len(res.Purchases), len(res.Adjustments))
	}
	if res.Adjustments[0].Delta != -10 {
		t.Errorf("Delta = %d, want -10", res.Adjustments[0].Delta)
	}
}

func TestSuggestions_이름과_거래처를_한_번에_준다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	h.postPurchase(t, defaultPurchase(), "sug-1", c)

	rec := h.do(http.MethodGet, "/api/admin/suggestions?q=쿠로미중형", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res struct {
		Names   []string `json:"names"`
		Vendors []string `json:"vendors"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Names) != 1 || res.Names[0] != "쿠로미 중형 30cm" {
		t.Errorf("Names = %v — 띄어쓰기를 무시하고 찾아야 한다", res.Names)
	}
	if len(res.Vendors) != 1 || res.Vendors[0] != "캐치돌" {
		t.Errorf("Vendors = %v", res.Vendors)
	}
}

func TestInventory_빈_장부도_200이다(t *testing.T) {
	// 빈 배열과 null 은 다르다. null 이 가면 화면이 .map 에서 터진다.
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodGet, "/api/admin/inventory", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("빈 목록이 [] 가 아니다: %s", rec.Body)
	}
}

func TestInventory_미인증은_401(t *testing.T) {
	h := newHarness(t)
	paths := []string{
		"/api/admin/inventory", "/api/admin/inventory/x",
		"/api/admin/purchases", "/api/admin/suggestions",
	}
	for _, p := range paths {
		if rec := h.do(http.MethodGet, p, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", p, rec.Code)
		}
	}
	if rec := h.postPurchase(t, defaultPurchase(), "unauth", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST purchases status = %d, want 401", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/admin/inventory/x/count", `{"countedQty":1}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST count status = %d, want 401", rec.Code)
	}
}

func TestPurchase_영수증은_보존기간_없이_저장된다(t *testing.T) {
	// 신고 사진은 개인정보라 보존 기간이 지나면 지우지만, 영수증은
	// 사장님 장부의 증빙이다. 만료 워커가 가져가면 안 된다.
	h := newHarness(t)
	c := h.login(t)
	o := defaultPurchase()
	o.receipt = jpeg(128)

	rec := h.postPurchase(t, o, "receipt-1", c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res createPurchaseResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if !res.HasReceipt {
		t.Fatal("영수증이 저장되지 않았다")
	}
	// 영수증은 claim_photos 가 아니므로 만료 대상 목록에 아예 없다.
	// 여기서는 첨부 자체가 성공했는지만 본다.
}
