package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// ── 보류 버튼 ───────────────────────────────────────────────────────────

func TestHold_사장님이_직접_보류할_수_있다(t *testing.T) {
	// 자동 보류만 있으면 "지금 판단 못 하겠다, 전화해보고 정하자"를
	// 표현할 방법이 없다. 그러면 접수함에 그냥 남겨두고 잊힌다.
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/hold", `{"note":"전화해보고 결정"}`, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := h.store.claims[id].Status; got != domain.StatusOnHold {
		t.Errorf("Status = %s, want on_hold", got)
	}
}

func TestHold_보류에서_다시_꺼낼_수_있다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	h.do(http.MethodPost, "/api/admin/claims/"+id+"/hold", "{}", c)
	if rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c); rec.Code != http.StatusOK {
		t.Errorf("보류 → 승인 status = %d", rec.Code)
	}
}

func TestHold_이미_송금한_건은_보류할_수_없다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", `{"method":"manual"}`, c)

	if rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/hold", "{}", c); rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestHold_링크토큰으로도_된다(t *testing.T) {
	// Slack 알림에서 바로 "나중에 보자"를 할 수 있어야 한다.
	h := newHarness(t)
	id := h.seedClaim(t, nil)
	tok := h.linkFor(t, id)

	if rec := h.doToken(http.MethodPost, "/api/admin/claims/"+id+"/hold", "{}", tok); rec.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

// ── 내 계정 ─────────────────────────────────────────────────────────────

func TestUpdateMe_이름과_연락처(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPatch, "/api/admin/me", `{"name":"손지영","phone":"010-9999-8888"}`, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var p profileResponse
	json.Unmarshal(rec.Body.Bytes(), &p) //nolint:errcheck

	if p.Name != "손지영" {
		t.Errorf("Name = %q", p.Name)
	}
	// 화면에 어떻게 쳤든 저장은 표준형이어야 반복 조회가 어긋나지 않는다.
	if p.Phone != "01099998888" {
		t.Errorf("Phone = %q, want 01099998888", p.Phone)
	}
	// 이메일은 이 화면에서 바꾸지 않는다. 로그인 수단이라 별도 절차가 필요하다.
	if p.Email != "owner@example.com" {
		t.Errorf("Email 이 바뀌었다: %q", p.Email)
	}
}

func TestUpdateMe_연락처는_선택이다(t *testing.T) {
	// 혼자 쓰는 사장님은 자기 번호를 적을 이유가 없다.
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPatch, "/api/admin/me", `{"name":"사장님","phone":""}`, c)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestUpdateMe_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	bad := []string{
		`{"name":"","phone":""}`,
		`{"name":"   ","phone":""}`,
		fmt.Sprintf(`{"name":%q}`, strings.Repeat("가", 41)),
		`{"name":"사장님","phone":"0212345678"}`,    // 유선번호
		`{"name":"사장님","phone":"010-1234-567a"}`, // 문자 섞임
	}
	for _, body := range bad {
		if rec := h.do(http.MethodPatch, "/api/admin/me", body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, rec.Code)
		}
	}
}

// ── 매장 정보 ───────────────────────────────────────────────────────────

func TestStore_조회와_수정(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodGet, "/api/admin/store", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var s storeResponse
	json.Unmarshal(rec.Body.Bytes(), &s) //nolint:errcheck
	if s.Name != "테스트 매장" {
		t.Errorf("Name = %q", s.Name)
	}

	upd := h.do(http.MethodPatch, "/api/admin/store", `{"name":"채현이 매장","phone":"02-333-4444"}`, c)
	if upd.Code != http.StatusOK {
		t.Fatalf("수정 status = %d, body = %s", upd.Code, upd.Body)
	}
	json.Unmarshal(upd.Body.Bytes(), &s) //nolint:errcheck
	if s.Name != "채현이 매장" {
		t.Errorf("Name = %q", s.Name)
	}
	// 매장 대표번호는 유선도 받는다. 손님 안내용이라 휴대폰일 이유가 없다.
	if s.Phone != "023334444" {
		t.Errorf("Phone = %q, want 023334444", s.Phone)
	}
}

func TestStore_이름_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	for _, body := range []string{
		`{"name":"","phone":""}`,
		fmt.Sprintf(`{"name":%q}`, strings.Repeat("가", 61)),
		`{"name":"매장","phone":"123"}`,
	} {
		if rec := h.do(http.MethodPatch, "/api/admin/store", body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, rec.Code)
		}
	}
}

// ── 직원 계정 ───────────────────────────────────────────────────────────

func TestUsers_직원_추가와_로그인(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPost, "/api/admin/users",
		`{"email":"staff@example.com","password":"staffpass1","name":"김직원","phone":"010-5555-6666"}`, c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var p profileResponse
	json.Unmarshal(rec.Body.Bytes(), &p) //nolint:errcheck
	if p.Name != "김직원" || !p.Active {
		t.Errorf("응답 = %+v", p)
	}

	list := h.do(http.MethodGet, "/api/admin/users", "", c)
	var lr struct {
		Users []profileResponse `json:"users"`
		MeID  string            `json:"meId"`
	}
	json.Unmarshal(list.Body.Bytes(), &lr) //nolint:errcheck
	if len(lr.Users) != 2 {
		t.Errorf("계정 %d개, want 2", len(lr.Users))
	}
	// 화면이 "나"를 표시하고 자기 비활성화 버튼을 숨기려면 필요하다.
	if lr.MeID != "user-1" {
		t.Errorf("meId = %q", lr.MeID)
	}
}

func TestUsers_추가_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	bad := []string{
		`{"email":"이메일아님","password":"staffpass1","name":"김직원"}`,
		`{"email":"a@b.com","password":"짧음","name":"김직원"}`,
		`{"email":"a@b.com","password":"staffpass1","name":""}`,
		`{"email":"owner@example.com","password":"staffpass1","name":"중복"}`,
		`{"email":"a@b.com","password":"staffpass1","name":"김직원","phone":"0212345678"}`,
	}
	for _, body := range bad {
		if rec := h.do(http.MethodPost, "/api/admin/users", body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestUsers_비활성화하면_로그인이_막힌다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPost, "/api/admin/users",
		`{"email":"staff@example.com","password":"staffpass1","name":"김직원"}`, c)
	var p profileResponse
	json.Unmarshal(rec.Body.Bytes(), &p) //nolint:errcheck

	if rec := h.do(http.MethodPatch, "/api/admin/users/"+p.ID, `{"active":false}`, c); rec.Code != http.StatusOK {
		t.Fatalf("비활성화 status = %d, body = %s", rec.Code, rec.Body)
	}
	if _, err := h.store.UserByID(t.Context(), p.ID); err == nil {
		t.Error("비활성 계정이 조회된다 — 세션 쿠키가 살아 있으면 계속 들어온다")
	}
}

func TestUsers_자기_계정은_비활성화할_수_없다(t *testing.T) {
	// 끄는 순간 로그아웃되고, 되돌릴 사람이 없을 수 있다.
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPatch, "/api/admin/users/user-1", `{"active":false}`, c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUsers_마지막_계정은_비활성화할_수_없다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	// 직원을 하나 만들고, 그 직원으로 사장님을 끄려 해도 남는 게 없으면 막혀야 한다.
	h.do(http.MethodPost, "/api/admin/users",
		`{"email":"staff@example.com","password":"staffpass1","name":"김직원"}`, c)

	// 직원을 먼저 끄고
	if rec := h.do(http.MethodPatch, "/api/admin/users/user-2", `{"active":false}`, c); rec.Code != http.StatusOK {
		t.Fatalf("직원 비활성화 status = %d", rec.Code)
	}
	// 이제 사장님 하나만 남았다. 다른 계정으로도 끌 수 없어야 한다.
	if err := h.store.SetUserActive(t.Context(), "store-1", "user-1", false); err == nil {
		t.Error("마지막 계정이 비활성화됐다 — 아무도 로그인하지 못한다")
	}
}

func TestUsers_남의_매장_계정은_못_건드린다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	if rec := h.do(http.MethodPatch, "/api/admin/users/없는계정", `{"active":false}`, c); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSettings_미인증은_401(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPatch, "/api/admin/me"},
		{http.MethodGet, "/api/admin/store"},
		{http.MethodPatch, "/api/admin/store"},
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users"},
		{http.MethodPatch, "/api/admin/users/user-2"},
	} {
		if rec := h.do(tc.method, tc.path, "{}", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

func TestSettings_링크토큰으로는_못_들어간다(t *testing.T) {
	// 링크 토큰은 그 신고 건 하나에만 통한다. 계정 관리에 닿으면
	// 링크 하나로 직원을 추가할 수 있게 된다.
	h := newHarness(t)
	id := h.seedClaim(t, nil)
	tok := h.linkFor(t, id)

	for _, p := range []string{"/api/admin/store", "/api/admin/users"} {
		if rec := h.doToken(http.MethodGet, p, "", tok); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", p, rec.Code)
		}
	}
}
