package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 가입할 때 업종을 잘못 고른 사장님이 되돌릴 방법이 없으면 /setup 을 다시
// 돌려 매장을 새로 만들게 되고, 기계와 민원이 첫 매장에 남아 접근할 수
// 없게 된다. storetype.go 주석이 "나중에 설정에서 바꿀 수 있다"고 적어둔
// 길이 실제로는 API 에 없었다.
func TestUpdateStore_업종을_바꿀_수_있다(t *testing.T) {
	h := newHarness(t)
	jar := h.login(t)

	rec := h.do(http.MethodPatch, "/api/admin/store",
		`{"name":"지영 테스트","storeType":"laundry"}`, jar)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res storeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.StoreType != "laundry" {
		t.Errorf("storeType = %q, want laundry", res.StoreType)
	}

	// 손님 화면이 바로 따라와야 한다. 업종만 바뀌고 증상 목록이 그대로면
	// 세탁소 손님에게 "인형 걸림"이 계속 보인다.
	mrec := httptest.NewRecorder()
	h.mux.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/api/public/machines/ABCD23", nil))
	var m machineResponse
	if err := json.Unmarshal(mrec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.StoreType != "laundry" {
		t.Errorf("손님 화면 storeType = %q", m.StoreType)
	}
	if len(m.Issues) == 0 || m.Issues[0].Value != "no_start" {
		t.Errorf("증상 목록이 안 바뀌었다: %+v", m.Issues)
	}
}

// 장터 설정만 고치는 화면은 storeType 을 안 보낸다. 그 요청이 업종을
// 기본값으로 덮어쓰면 손님 화면이 통째로 바뀐다.
func TestUpdateStore_업종을_안_보내면_유지한다(t *testing.T) {
	h := newHarness(t)
	jar := h.login(t)

	if rec := h.do(http.MethodPatch, "/api/admin/store",
		`{"name":"가게","storeType":"karaoke"}`, jar); rec.Code != http.StatusOK {
		t.Fatalf("첫 변경 실패: %d %s", rec.Code, rec.Body)
	}
	rec := h.do(http.MethodPatch, "/api/admin/store",
		`{"name":"가게","regionDetail":"강남구"}`, jar)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res storeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.StoreType != "karaoke" {
		t.Errorf("storeType = %q — 업종을 안 보낸 요청이 덮어썼다", res.StoreType)
	}
}

func TestUpdateStore_모르는_업종은_거절한다(t *testing.T) {
	h := newHarness(t)
	jar := h.login(t)
	rec := h.do(http.MethodPatch, "/api/admin/store",
		`{"name":"가게","storeType":"nonsense"}`, jar)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 — 오타를 조용히 기본값으로 떨어뜨리면 안 된다", rec.Code)
	}
}
