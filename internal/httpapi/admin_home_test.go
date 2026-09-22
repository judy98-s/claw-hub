package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func homeOf(t *testing.T, h *harness, c *http.Cookie) homeResponse {
	t.Helper()
	rec := h.do(http.MethodGet, "/api/admin/home", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res homeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func actionKinds(res homeResponse) []string {
	out := make([]string, 0, len(res.Actions))
	for _, a := range res.Actions {
		out = append(out, a.Kind)
	}
	return out
}

func TestHome_할일이_없으면_목록이_빈다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	res := homeOf(t, h, c)
	for _, a := range res.Actions {
		if a.Kind != "setup_payout" {
			t.Errorf("신고가 없는데 할 일이 있다: %+v", a)
		}
	}
}

func TestHome_안나간_환불이_가장_위다(t *testing.T) {
	// 승인만 하고 안 보낸 건은 손님에게 돈을 약속하고 주지 않은 상태다.
	// 새 신고보다 급하다.
	h := newHarness(t)
	c := h.login(t)

	approved := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+approved+"/approve", "{}", c)
	h.seedClaim(t, nil) // 새 접수 하나

	res := homeOf(t, h, c)
	got := actionKinds(res)
	if len(got) == 0 || got[0] != "payout_pending" {
		t.Fatalf("할 일 순서 = %v, 첫 번째가 payout_pending 이어야 한다", got)
	}
	if res.Actions[0].Severity != "stop" {
		t.Errorf("Severity = %q, want stop", res.Actions[0].Severity)
	}
	// 건수만 보여주면 3건이 3천원인지 30만원인지 모른다.
	if res.Actions[0].Detail == "" {
		t.Error("금액이 없는 안내다")
	}
}

func TestHome_확인필요가_새신고보다_위다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	h.seedClaim(t, nil)                                                                     // pending
	h.seedClaim(t, func(f *formOpts) { f.amount = "30000"; f.photos = [][]byte{jpeg(64)} }) // needs_review

	got := actionKinds(homeOf(t, h, c))
	needs, pending := indexOf(got, "needs_review"), indexOf(got, "pending")
	if needs < 0 || pending < 0 {
		t.Fatalf("할 일 = %v", got)
	}
	if needs > pending {
		t.Errorf("할 일 = %v — 확인 필요가 새 신고보다 위여야 한다", got)
	}
}

func TestHome_기계에_몰리면_점검_알림이_뜬다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	for i := 0; i < machineAlertThreshold; i++ {
		h.seedClaim(t, nil)
	}

	res := homeOf(t, h, c)
	var found *homeAction
	for i := range res.Actions {
		if res.Actions[i].Kind == "machine_alert" {
			found = &res.Actions[i]
		}
	}
	if found == nil {
		t.Fatalf("기계 알림이 없다: %v", actionKinds(res))
	}
	if found.Count != machineAlertThreshold {
		t.Errorf("Count = %d, want %d", found.Count, machineAlertThreshold)
	}
}

func TestHome_오늘_집계가_따라온다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", `{"method":"manual"}`, c)

	res := homeOf(t, h, c)
	if res.TodayClaims != 1 {
		t.Errorf("TodayClaims = %d, want 1", res.TodayClaims)
	}
	if res.TodayPaid != 1 || res.TodayPaidKrw != 2000 {
		t.Errorf("TodayPaid = %d / %d원", res.TodayPaid, res.TodayPaidKrw)
	}
}

func TestHome_미인증은_401(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodGet, "/api/admin/home", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}
