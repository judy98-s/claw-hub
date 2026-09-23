package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/store"
)

// marketHarness는 매장 두 곳을 세운다.
// 장터는 매장이 하나면 아무것도 검증되지 않는다.
func marketHarness(t *testing.T) (*harness, *http.Cookie, *http.Cookie) {
	t.Helper()
	h := newHarness(t)

	// 기본 매장(store-1)을 장터에 쓸 수 있게 채운다.
	h.store.storeDetail.RegionCode = "seoul"
	h.store.storeDetail.RegionDetail = "강남구"
	h.store.storeDetail.BizNo = "2208162517"
	h.store.storeDetail.Phone = "0212345678"

	other := h.store.addStore("store-2", "부산 매장", "0519998888")
	return h, h.login(t), h.loginAs(t, other)
}

func postJSON(h *harness, path, body string, c *http.Cookie) *httptest.ResponseRecorder {
	return h.do(http.MethodPost, path, body, c)
}

const sellBody = `{"name":"쿠로미 중형 30cm","kind":"sell","qty":40,"unitPriceKrw":1800,"note":"직거래만"}`

func createListing(t *testing.T, h *harness, body string, c *http.Cookie) store.Listing {
	t.Helper()
	rec := postJSON(h, "/api/admin/market", body, c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("글쓰기 실패: %d %s", rec.Code, rec.Body)
	}
	var l store.Listing
	json.Unmarshal(rec.Body.Bytes(), &l) //nolint:errcheck
	return l
}

func TestMarket_글을_올리면_남에게_보인다(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	createListing(t, h, sellBody, mine)

	rec := h.do(http.MethodGet, "/api/admin/market", "", theirs)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res struct {
		Listings []store.Listing `json:"listings"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Listings) != 1 {
		t.Fatalf("목록 = %+v", res.Listings)
	}
	if res.Listings[0].Mine {
		t.Error("남의 글인데 Mine=true")
	}
	if res.Listings[0].RegionName != "서울" || res.Listings[0].RegionDetail != "강남구" {
		t.Errorf("지역 = %q %q", res.Listings[0].RegionName, res.Listings[0].RegionDetail)
	}
}

func TestMarket_목록_응답에_전화번호가_없다(t *testing.T) {
	// 이게 이 기능의 핵심 방어선이다. 번호가 HTML에 미리 들어가면
	// 봇이 하루면 전부 긁어간다.
	h, mine, theirs := marketHarness(t)
	createListing(t, h, sellBody, mine)

	for _, path := range []string{"/api/admin/market", "/api/admin/market/mine"} {
		body := h.do(http.MethodGet, path, "", theirs).Body.String()
		if strings.Contains(body, "0212345678") {
			t.Errorf("%s 응답에 전화번호가 있다: %s", path, body)
		}
		if strings.Contains(body, `"phone"`) {
			t.Errorf("%s 응답에 phone 키가 있다", path)
		}
	}
}

func TestMarket_전화버튼을_눌러야_번호가_온다(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	rec := postJSON(h, "/api/admin/market/"+l.ID+"/contact", "{}", theirs)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var c contactResponse
	json.Unmarshal(rec.Body.Bytes(), &c) //nolint:errcheck
	if c.Phone != "0212345678" {
		t.Errorf("Phone = %q", c.Phone)
	}
	if c.BizNo != "220-81-62517" {
		t.Errorf("BizNo = %q — 화면에는 하이픈이 들어간 형태로 나가야 한다", c.BizNo)
	}
	if c.RemainingToday != contactsPerDay-1 {
		t.Errorf("RemainingToday = %d, want %d", c.RemainingToday, contactsPerDay-1)
	}
}

func TestMarket_자기_글에는_전화가_403(t *testing.T) {
	// 자기 번호를 자기가 열람할 이유가 없고, 그 기록이 남으면 감사
	// 로그가 쓰레기가 된다.
	h, mine, _ := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	rec := postJSON(h, "/api/admin/market/"+l.ID+"/contact", "{}", mine)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	// 기록도 남지 않아야 한다.
	if n, _ := h.store.ContactsTodayFor(t.Context(), "store-1", h.server.nowKST()); n != 0 {
		t.Errorf("자기 글 열람이 기록됐다: %d건", n)
	}
}

func TestMarket_하루_상한을_넘기면_429지만_목록은_계속_보인다(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	for i := 0; i < contactsPerDay; i++ {
		if rec := postJSON(h, "/api/admin/market/"+l.ID+"/contact", "{}", theirs); rec.Code != http.StatusOK {
			t.Fatalf("%d번째 열람 = %d", i+1, rec.Code)
		}
	}
	rec := postJSON(h, "/api/admin/market/"+l.ID+"/contact", "{}", theirs)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("상한 초과 status = %d, want 429", rec.Code)
	}

	// 장터를 못 쓰게 만드는 게 아니라 수집을 막는 게 목적이다.
	if rec := h.do(http.MethodGet, "/api/admin/market", "", theirs); rec.Code != http.StatusOK {
		t.Errorf("상한에 걸렸다고 목록까지 막혔다: %d", rec.Code)
	}
}

func TestMarket_설정이_비면_글을_못_쓴다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := postJSON(h, "/api/admin/market", sellBody, c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409. body = %s", rec.Code, rec.Body)
	}
	// 어디로 가야 하는지 알려줘야 한다.
	if !strings.Contains(rec.Body.String(), "설정") {
		t.Errorf("설정 화면을 가리키지 않는다: %s", rec.Body)
	}
}

func TestMarket_입력_검증(t *testing.T) {
	tests := []struct{ name, body string }{
		{"이름 없음", `{"name":"  ","kind":"sell","qty":10,"unitPriceKrw":100}`},
		{"수량 0", `{"name":"쿠로미","kind":"sell","qty":0,"unitPriceKrw":100}`},
		{"수량 음수", `{"name":"쿠로미","kind":"sell","qty":-5,"unitPriceKrw":100}`},
		{"판매인데 0원", `{"name":"쿠로미","kind":"sell","qty":10,"unitPriceKrw":0}`},
		{"가격 음수", `{"name":"쿠로미","kind":"sell","qty":10,"unitPriceKrw":-1}`},
		{"알 수 없는 거래 방식", `{"name":"쿠로미","kind":"give","qty":10,"unitPriceKrw":100}`},
		{"거래 방식 없음", `{"name":"쿠로미","qty":10,"unitPriceKrw":100}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, mine, _ := marketHarness(t)
			rec := postJSON(h, "/api/admin/market", tc.body, mine)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body = %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestMarket_교환글은_가격없이_통과하고_0으로_저장된다(t *testing.T) {
	h, mine, _ := marketHarness(t)
	l := createListing(t, h, `{"name":"짱구 대형","kind":"swap","qty":20,"unitPriceKrw":5000}`, mine)
	if l.UnitPriceKRW != 0 {
		t.Errorf("UnitPriceKRW = %d, want 0", l.UnitPriceKRW)
	}
}

func TestMarket_내_글만_내릴_수_있다(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	if rec := postJSON(h, "/api/admin/market/"+l.ID+"/close", "{}", theirs); rec.Code != http.StatusNotFound {
		t.Fatalf("남이 내렸다: %d", rec.Code)
	}
	if rec := postJSON(h, "/api/admin/market/"+l.ID+"/close", "{}", mine); rec.Code != http.StatusOK {
		t.Fatalf("내 글을 못 내렸다: %d %s", rec.Code, rec.Body)
	}
	// 내린 뒤에는 남에게 안 보인다.
	if rec := h.do(http.MethodGet, "/api/admin/market/"+l.ID, "", theirs); rec.Code != http.StatusNotFound {
		t.Errorf("내린 글의 상세가 열렸다: %d", rec.Code)
	}
	if rec := postJSON(h, "/api/admin/market/"+l.ID+"/contact", "{}", theirs); rec.Code != http.StatusNotFound {
		t.Errorf("내린 글의 번호가 나왔다: %d", rec.Code)
	}
}

func TestMarket_신고(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	// 내 글은 신고 대상이 아니다.
	if rec := postJSON(h, "/api/admin/market/"+l.ID+"/report", `{"reason":"x"}`, mine); rec.Code != http.StatusBadRequest {
		t.Errorf("내 글 신고 status = %d, want 400", rec.Code)
	}

	rec := postJSON(h, "/api/admin/market/"+l.ID+"/report", `{"reason":"사진과 다름"}`, theirs)
	if rec.Code != http.StatusOK {
		t.Fatalf("신고 실패: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Removed bool `json:"removed"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if res.Removed {
		t.Error("한 건으로 내려갔다")
	}

	// 같은 매장이 더 눌러도 안 내려간다.
	for i := 0; i < 3; i++ {
		postJSON(h, "/api/admin/market/"+l.ID+"/report", "{}", theirs)
	}
	if rec := h.do(http.MethodGet, "/api/admin/market/"+l.ID, "", theirs); rec.Code != http.StatusOK {
		t.Errorf("한 매장의 반복 신고로 글이 내려갔다: %d", rec.Code)
	}
}

func TestMarket_세_매장이_신고하면_내려간다(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	l := createListing(t, h, sellBody, mine)

	cookies := []*http.Cookie{theirs}
	for i := 3; i <= 4; i++ {
		u := h.store.addStore(fmt.Sprintf("store-%d", i), fmt.Sprintf("매장 %d", i), "0212223333")
		cookies = append(cookies, h.loginAs(t, u))
	}

	var removed bool
	for _, c := range cookies {
		rec := postJSON(h, "/api/admin/market/"+l.ID+"/report", "{}", c)
		var res struct {
			Removed bool `json:"removed"`
		}
		json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
		removed = removed || res.Removed
	}
	if !removed {
		t.Fatal("세 매장이 신고했는데 안 내려갔다")
	}
	if rec := h.do(http.MethodGet, "/api/admin/market/"+l.ID, "", theirs); rec.Code != http.StatusNotFound {
		t.Errorf("신고 누적된 글이 아직 열린다: %d", rec.Code)
	}
}

func TestMarket_내_글_목록에는_내린_글도_나온다(t *testing.T) {
	h, mine, _ := marketHarness(t)
	l := createListing(t, h, sellBody, mine)
	postJSON(h, "/api/admin/market/"+l.ID+"/close", "{}", mine)

	rec := h.do(http.MethodGet, "/api/admin/market/mine", "", mine)
	var res struct {
		Listings []store.Listing `json:"listings"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Listings) != 1 || res.Listings[0].Status != "closed" {
		t.Fatalf("내 글 목록 = %+v", res.Listings)
	}
}

func TestMarket_지역과_거래방식_필터(t *testing.T) {
	h, mine, theirs := marketHarness(t)
	createListing(t, h, sellBody, mine)                                // 서울·판매
	createListing(t, h, `{"name":"짱구","kind":"swap","qty":5}`, theirs) // 부산·교환

	count := func(q string, c *http.Cookie) int {
		rec := h.do(http.MethodGet, "/api/admin/market"+q, "", c)
		var res struct {
			Listings []store.Listing `json:"listings"`
		}
		json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
		return len(res.Listings)
	}
	if n := count("", mine); n != 2 {
		t.Errorf("전체 = %d건, want 2", n)
	}
	if n := count("?region=seoul", mine); n != 1 {
		t.Errorf("서울 = %d건, want 1", n)
	}
	if n := count("?kind=swap", mine); n != 1 {
		t.Errorf("교환 = %d건, want 1", n)
	}
	if rec := h.do(http.MethodGet, "/api/admin/market?region=atlantis", "", mine); rec.Code != http.StatusBadRequest {
		t.Errorf("없는 지역 status = %d, want 400", rec.Code)
	}
}

func TestMarket_미인증은_401(t *testing.T) {
	h, _, _ := marketHarness(t)
	gets := []string{"/api/admin/market", "/api/admin/market/mine", "/api/admin/market/x", "/api/admin/regions"}
	for _, p := range gets {
		if rec := h.do(http.MethodGet, p, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s = %d, want 401", p, rec.Code)
		}
	}
	posts := []string{"/api/admin/market", "/api/admin/market/x/contact",
		"/api/admin/market/x/close", "/api/admin/market/x/report"}
	for _, p := range posts {
		if rec := postJSON(h, p, "{}", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s = %d, want 401", p, rec.Code)
		}
	}
}

func TestSettings_지역과_사업자등록번호(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	// 체크섬이 틀린 번호는 400. 사장님이 읽을 메시지가 나간다.
	bad := `{"name":"테스트 매장","phone":"0212345678","regionCode":"seoul","regionDetail":"강남구","bizNo":"220-81-62518"}`
	if rec := h.do(http.MethodPatch, "/api/admin/store", bad, c); rec.Code != http.StatusBadRequest {
		t.Fatalf("틀린 사업자번호 status = %d, want 400", rec.Code)
	}
	// 없는 지역도 400.
	badRegion := `{"name":"테스트 매장","phone":"0212345678","regionCode":"atlantis","regionDetail":"강남구"}`
	if rec := h.do(http.MethodPatch, "/api/admin/store", badRegion, c); rec.Code != http.StatusBadRequest {
		t.Errorf("없는 지역 status = %d, want 400", rec.Code)
	}
	// 지역을 골랐으면 시·군·구도 있어야 한다.
	noDetail := `{"name":"테스트 매장","phone":"0212345678","regionCode":"seoul"}`
	if rec := h.do(http.MethodPatch, "/api/admin/store", noDetail, c); rec.Code != http.StatusBadRequest {
		t.Errorf("시·군·구 없이 통과했다: %d", rec.Code)
	}

	good := `{"name":"테스트 매장","phone":"0212345678","regionCode":"seoul","regionDetail":"강남구","bizNo":"220-81-62517"}`
	rec := h.do(http.MethodPatch, "/api/admin/store", good, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res storeResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if !res.CanPostListing {
		t.Error("설정을 채웠는데 CanPostListing=false")
	}
	if res.BizNo != "220-81-62517" {
		t.Errorf("BizNo = %q — 화면에는 하이픈 형태로 나가야 한다", res.BizNo)
	}
}
