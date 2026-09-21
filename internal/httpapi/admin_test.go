package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// login은 로그인해서 세션 쿠키를 얻는다.
func (h *harness) login(t *testing.T) *http.Cookie {
	t.Helper()
	body := `{"email":"owner@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("로그인 실패: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("세션 쿠키가 없다")
	return nil
}

// do는 인증된 요청을 보낸다. cookie가 nil이면 인증 없이 보낸다.
func (h *harness) do(method, path string, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, r)
	return rec
}

// seedClaim은 접수 한 건을 만들고 그 ID를 돌려준다.
func (h *harness) seedClaim(t *testing.T, mut func(*formOpts)) string {
	t.Helper()
	f := defaultForm()
	if mut != nil {
		mut(&f)
	}
	rec := h.postClaim(f, fmt.Sprintf("seed-%d", h.store.claimCount()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("접수 실패: %d %s", rec.Code, rec.Body)
	}
	var res claimResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	return res.ID
}

// ── 인증 ────────────────────────────────────────────────────────────────

func TestAdmin_미인증은_401(t *testing.T) {
	h := newHarness(t)
	paths := []string{
		"/api/admin/me", "/api/admin/claims", "/api/admin/claims/x",
		"/api/admin/machines", "/api/admin/contacts", "/api/admin/stats/daily",
	}
	for _, p := range paths {
		if rec := h.do(http.MethodGet, p, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", p, rec.Code)
		}
	}
}

func TestLogin_쿠키_속성(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	if !c.HttpOnly {
		t.Error("HttpOnly가 아니다 — XSS로 세션을 훔칠 수 있다")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax — CSRF 기본 방어", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q", c.Path)
	}
	// http://localhost 개발 환경에서는 Secure를 켜면 쿠키가 저장되지 않는다.
	if c.Secure {
		t.Error("http 환경인데 Secure가 켜졌다 — 쿠키가 저장되지 않는다")
	}
}

func TestLogin_https면_Secure가_켜진다(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Config.PublicBaseURL = "https://claw.example.com" })
	if c := h.login(t); !c.Secure {
		t.Error("https 환경인데 Secure가 꺼졌다")
	}
}

func TestLogin_실패는_어느쪽이_틀렸는지_알려주지_않는다(t *testing.T) {
	h := newHarness(t)
	cases := []string{
		`{"email":"owner@example.com","password":"wrong"}`,
		`{"email":"nobody@example.com","password":"secret123"}`,
	}
	var messages []string
	for _, body := range cases {
		rec := h.do(http.MethodPost, "/api/admin/login", body, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		var e errorBody
		json.Unmarshal(rec.Body.Bytes(), &e) //nolint:errcheck
		messages = append(messages, e.Message)
	}
	if messages[0] != messages[1] {
		t.Errorf("메시지가 다르다: %q vs %q — 가입 여부가 샌다", messages[0], messages[1])
	}
}

func TestLogin_잠긴계정은_429(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/admin/login",
		`{"email":"locked@example.com","password":"secret123"}`, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

func TestLogin_이메일_대소문자_무시(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/api/admin/login",
		`{"email":"  OWNER@Example.COM ","password":"secret123"}`, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d — 이메일 대소문자로 로그인이 막히면 안 된다", rec.Code)
	}
}

func TestSession_위조된_쿠키는_거부된다(t *testing.T) {
	h := newHarness(t)
	good := h.login(t)

	forged := []string{
		"user-1.9999999999.deadbeef", // 서명 위조
		good.Value + "x",             // 서명 변조
		"user-1.1.",                  // 만료
		"",                           // 빈 값
		"garbage",                    // 형식 오류
	}
	for _, v := range forged {
		rec := h.do(http.MethodGet, "/api/admin/me", "", &http.Cookie{Name: sessionCookie, Value: v})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("쿠키 %q 가 통과됐다: %d", v, rec.Code)
		}
	}
}

func TestLogout_쿠키를_지운다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPost, "/api/admin/logout", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, sc := range rec.Result().Cookies() {
		if sc.Name == sessionCookie && sc.MaxAge >= 0 {
			t.Errorf("쿠키가 만료되지 않았다: MaxAge=%d", sc.MaxAge)
		}
	}
}

// ── claim 조회와 처리 ───────────────────────────────────────────────────

func TestClaimDetail_계좌가_복호화되어_나온다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	rec := h.do(http.MethodGet, "/api/admin/claims/"+id, "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var d claimDetailResponse
	json.Unmarshal(rec.Body.Bytes(), &d) //nolint:errcheck

	if d.AccountNo != "3333011234567" {
		t.Errorf("AccountNo = %q", d.AccountNo)
	}
	if d.Phone != "01012345678" {
		t.Errorf("Phone = %q", d.Phone)
	}
	if d.BankName != "토스뱅크" {
		t.Errorf("BankName = %q — 코드만 보여주면 사장님이 못 읽는다", d.BankName)
	}
	if len(d.AllowedTransitions) == 0 {
		t.Error("AllowedTransitions가 비었다 — UI가 어떤 버튼을 띄울지 모른다")
	}
}

func TestClaimDetail_고액이면_통화를_권한다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	small := h.seedClaim(t, nil)
	big := h.seedClaim(t, func(f *formOpts) {
		f.amount = "50000"
		f.photos = [][]byte{jpeg(1000)}
	})

	for _, tc := range []struct {
		id   string
		want bool
	}{{small, false}, {big, true}} {
		rec := h.do(http.MethodGet, "/api/admin/claims/"+tc.id, "", c)
		var d claimDetailResponse
		json.Unmarshal(rec.Body.Bytes(), &d) //nolint:errcheck
		if d.CallRecommended != tc.want {
			t.Errorf("claim %s: CallRecommended = %v, want %v", tc.id, d.CallRecommended, tc.want)
		}
	}
}

func TestClaimDetail_없는_건은_404(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	if rec := h.do(http.MethodGet, "/api/admin/claims/없음", "", c); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestTransition_승인과_송금(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	if rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", `{"note":"사진 확인"}`, c); rec.Code != http.StatusOK {
		t.Fatalf("승인 status = %d, body = %s", rec.Code, rec.Body)
	}
	rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", `{"method":"deeplink"}`, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("송금 기록 status = %d, body = %s", rec.Code, rec.Body)
	}

	detail := h.do(http.MethodGet, "/api/admin/claims/"+id, "", c)
	var d claimDetailResponse
	json.Unmarshal(detail.Body.Bytes(), &d) //nolint:errcheck
	if d.Status != domain.StatusPaid {
		t.Errorf("Status = %s, want paid", d.Status)
	}
	if d.PaidAt == nil {
		t.Error("PaidAt이 비었다")
	}
}

func TestMarkPaid_송금방법이_없으면_거부(t *testing.T) {
	// "이 건 진짜 보냈나"를 나중에 확인할 근거가 필요하다.
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)

	for _, body := range []string{"{}", `{"method":""}`, `{"method":"carrier_pigeon"}`} {
		rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", body, c)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestTransition_pending에서_paid_직행은_거부(t *testing.T) {
	// 승인 없이 송금 기록이 되면 감사 흐름이 끊긴다.
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", `{"method":"manual"}`, c)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestTransition_paid를_되돌릴_수_없다(t *testing.T) {
	// 되돌릴 수 있으면 이중 환불이 난다.
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/mark-paid", `{"method":"manual"}`, c)

	for _, action := range []string{"approve", "reject"} {
		rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/"+action, "{}", c)
		if rec.Code != http.StatusConflict {
			t.Errorf("paid → %s status = %d, want 409", action, rec.Code)
		}
	}
}

func TestTransition_두번_승인하면_두번째는_409(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	if rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c); rec.Code != http.StatusOK {
		t.Fatalf("첫 승인 실패: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c); rec.Code != http.StatusConflict {
		t.Errorf("두 번째 승인 status = %d, want 409", rec.Code)
	}
}

// ── 송금 링크 ───────────────────────────────────────────────────────────

func TestPayoutLinks_승인전에는_409(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)

	if rec := h.do(http.MethodGet, "/api/admin/claims/"+id+"/payout-links", "", c); rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestPayoutLinks_딥링크와_수동경로를_함께_준다(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)

	rec := h.do(http.MethodGet, "/api/admin/claims/"+id+"/payout-links", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res payoutLinksResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck

	if len(res.Links) != 1 || res.Links[0].Provider != "toss" {
		t.Errorf("Links = %+v", res.Links)
	}
	// 딥링크가 있어도 수동 경로는 항상 함께 와야 한다. PC에서 열면 딥링크가
	// 아무것도 안 하고, 그때 계좌 복사가 유일한 길이다.
	if res.AccountNo == "" || res.BankName == "" || res.CopyText == "" {
		t.Errorf("수동 경로 정보가 없다: %+v", res)
	}
}

func TestPayoutLinks_딥링크_미설정이어도_수동경로는_온다(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Payout = newEmptyPayout()
	})
	c := h.login(t)
	id := h.seedClaim(t, nil)
	h.do(http.MethodPost, "/api/admin/claims/"+id+"/approve", "{}", c)

	rec := h.do(http.MethodGet, "/api/admin/claims/"+id+"/payout-links", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var res payoutLinksResponse
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Links) != 0 {
		t.Errorf("Links = %+v, want 0", res.Links)
	}
	if res.CopyText == "" {
		t.Error("딥링크가 없는데 복사 텍스트도 없다 — 사장님이 송금할 방법이 없다")
	}
}

// ── 사진 ────────────────────────────────────────────────────────────────

func TestPhoto_캐시금지_헤더(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	id := h.seedClaim(t, func(f *formOpts) { f.photos = [][]byte{jpeg(1000)} })

	detail := h.do(http.MethodGet, "/api/admin/claims/"+id, "", c)
	var d claimDetailResponse
	json.Unmarshal(detail.Body.Bytes(), &d) //nolint:errcheck
	if len(d.PhotoIDs) != 1 {
		t.Fatalf("사진 %d장", len(d.PhotoIDs))
	}

	rec := h.do(http.MethodGet, "/api/admin/photos/"+d.PhotoIDs[0]+"?claimId="+id, "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q — 개인정보가 담긴 이미지가 캐시된다", cc)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff가 없다 — 브라우저가 타입을 재해석할 수 있다")
	}
}

func TestPhoto_claimId_없으면_400(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	if rec := h.do(http.MethodGet, "/api/admin/photos/abc", "", c); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// ── 기계 ────────────────────────────────────────────────────────────────

func TestMachines_생성과_수정(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	rec := h.do(http.MethodPost, "/api/admin/machines", `{"label":"5번 기계","location":"안쪽"}`, c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var m machineItem
	json.Unmarshal(rec.Body.Bytes(), &m) //nolint:errcheck

	if m.Code == "" {
		t.Error("코드가 비었다")
	}
	if !strings.HasSuffix(m.QRTarget, "/r/"+m.Code) {
		t.Errorf("QRTarget = %q — QR이 가리킬 주소가 틀렸다", m.QRTarget)
	}

	// PATCH는 부분 수정이다. label만 보내면 location은 유지돼야 한다.
	upd := h.do(http.MethodPatch, "/api/admin/machines/"+m.ID, `{"label":"5번 기계(수리중)"}`, c)
	if upd.Code != http.StatusOK {
		t.Fatalf("수정 status = %d, body = %s", upd.Code, upd.Body)
	}
	var m2 machineItem
	json.Unmarshal(upd.Body.Bytes(), &m2) //nolint:errcheck
	if m2.Label != "5번 기계(수리중)" {
		t.Errorf("Label = %q", m2.Label)
	}
	if m2.Location != "안쪽" {
		t.Errorf("Location = %q — 보내지 않은 필드가 지워졌다", m2.Location)
	}
	if m2.Code != m.Code {
		t.Errorf("코드가 바뀌었다: %q → %q — 스티커는 이미 붙어 있다", m.Code, m2.Code)
	}
}

func TestMachines_이름_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	for _, body := range []string{`{"label":""}`, `{"label":"   "}`, fmt.Sprintf(`{"label":%q}`, strings.Repeat("가", 51))} {
		if rec := h.do(http.MethodPost, "/api/admin/machines", body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestMachineQR_유효한_PNG(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	m := h.store.machines["ABCD23"]

	rec := h.do(http.MethodGet, "/api/admin/machines/"+m.ID+"/qr.png", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Error("PNG 매직바이트가 아니다")
	}
}

func TestMachineQR_없는_기계는_404(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	if rec := h.do(http.MethodGet, "/api/admin/machines/없음/qr.png", "", c); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ── 연락처와 통계 ───────────────────────────────────────────────────────

func TestSetContactStatus_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	validHash := strings.Repeat("ab", 32) // 32바이트 hex

	if rec := h.do(http.MethodPatch, "/api/admin/contacts/"+validHash, `{"status":"blocked"}`, c); rec.Code != http.StatusOK {
		t.Errorf("정상 요청 status = %d, body = %s", rec.Code, rec.Body)
	}
	bad := []struct{ hash, body string }{
		{"짧음", `{"status":"blocked"}`},
		{strings.Repeat("ab", 10), `{"status":"blocked"}`},
		{validHash, `{"status":"banned"}`},
		{validHash, `{"status":""}`},
	}
	for _, tc := range bad {
		if rec := h.do(http.MethodPatch, "/api/admin/contacts/"+tc.hash, tc.body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("hash=%q body=%s: status = %d, want 400", tc.hash, tc.body, rec.Code)
		}
	}
}

func TestDailyStats_기간_검증(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)

	if rec := h.do(http.MethodGet, "/api/admin/stats/daily", "", c); rec.Code != http.StatusOK {
		t.Errorf("기본 조회 status = %d — 파라미터 없이 바로 그릴 수 있어야 한다", rec.Code)
	}
	bad := []string{
		"?from=2026-13-01",
		"?from=2026-09-01&to=2026-08-01", // 역순
		"?from=2020-01-01&to=2026-01-01", // 1년 초과
	}
	for _, q := range bad {
		if rec := h.do(http.MethodGet, "/api/admin/stats/daily"+q, "", c); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func TestListClaims_상태필터(t *testing.T) {
	h := newHarness(t)
	c := h.login(t)
	h.seedClaim(t, nil)

	rec := h.do(http.MethodGet, "/api/admin/claims?status=pending", "", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodGet, "/api/admin/claims?status=없는상태", "", c); rec.Code != http.StatusBadRequest {
		t.Errorf("알 수 없는 상태 status = %d, want 400", rec.Code)
	}
}
