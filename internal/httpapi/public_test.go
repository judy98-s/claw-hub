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
	"time"

	"github.com/judy98-s/claw-hub/internal/cache"
	"github.com/judy98-s/claw-hub/internal/config"
	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/payout"
)

// jpeg는 유효한 JPEG 매직바이트로 시작하는 바이트열이다.
func jpeg(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'})
	return b
}

type harness struct {
	server *Server
	store  *fakeStore
	media  *fakeMedia
	notify *fakeNotifier
	mux    http.Handler
}

func newHarness(t *testing.T, opts ...func(*Deps)) *harness {
	t.Helper()

	st := newFakeStore()
	st.addMachine("ABCD23", "3번 기계")
	md := newFakeMedia()
	nt := &fakeNotifier{}

	secret := make([]byte, 32)
	d := Deps{
		Config: config.Config{
			SessionSecret: secret,
			PublicBaseURL: "http://localhost:3000",
			Policy:        config.Policy{ReviewThresholdKRW: 10000, RepeatWatchCount: 3, RepeatHoldCount: 5, AccountSharingPhones: 3, PayoutCeilingKRW: 50000, MaxAmountKRW: 1000000, MachineAlertCount: 3, PhotoRetentionDays: 90},
		},
		Store: st, Cache: cache.NewMemory(), Media: md, Notify: nt,
		Payout: payout.NewDeeplink(map[string]string{
			"toss": "supertoss://send?bank={bank}&accountNo={account}&amount={amount}",
		}),
	}
	for _, o := range opts {
		o(&d)
	}
	srv := New(d)
	return &harness{server: srv, store: st, media: md, notify: nt, mux: srv.Handler()}
}

// claimForm은 접수 요청 multipart 본문을 만든다.
type formOpts struct {
	machineCode   string
	issueType     string
	paymentMethod string
	cardLast4     string
	paidAtGuess   string
	amount        string
	phone         string
	bankCode      string
	account       string
	holder        string
	description   string
	photos        [][]byte
	photoNames    []string
	omit          map[string]bool
}

func defaultForm() formOpts {
	return formOpts{
		machineCode: "ABCD23", issueType: "cash_eaten",
		paymentMethod: "cash", amount: "2000",
		phone: "010-1234-5678", bankCode: "090", account: "3333-01-1234567",
		holder: "김민수", description: "천원 두 번 넣었는데 안 나옴",
	}
}

func buildForm(o formOpts) (string, *bytes.Buffer) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	set := func(k, v string) {
		if o.omit[k] {
			return
		}
		w.WriteField(k, v) //nolint:errcheck
	}
	set("machineCode", o.machineCode)
	set("issueType", o.issueType)
	set("paymentMethod", o.paymentMethod)
	set("cardLast4", o.cardLast4)
	set("paidAtGuess", o.paidAtGuess)
	set("amountKrw", o.amount)
	set("phone", o.phone)
	set("bankCode", o.bankCode)
	set("accountNo", o.account)
	set("holder", o.holder)
	set("description", o.description)

	for i, p := range o.photos {
		name := fmt.Sprintf("photo%d.jpg", i)
		if i < len(o.photoNames) {
			name = o.photoNames[i]
		}
		fw, _ := w.CreateFormFile("photos", name)
		fw.Write(p) //nolint:errcheck
	}
	w.Close() //nolint:errcheck
	return w.FormDataContentType(), &buf
}

func (h *harness) postClaim(o formOpts, idemKey string) *httptest.ResponseRecorder {
	ct, body := buildForm(o)
	req := httptest.NewRequest(http.MethodPost, "/api/public/claims", body)
	req.Header.Set("Content-Type", ct)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func TestCreateClaim_정상접수(t *testing.T) {
	h := newHarness(t)
	rec := h.postClaim(defaultForm(), "key-1")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res claimResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.ID == "" || res.ReceiptCode == "" {
		t.Errorf("응답 = %+v", res)
	}
	if len(res.ReceiptCode) != 8 {
		t.Errorf("접수번호 = %q, 손님이 읽어줄 수 있는 길이여야 한다", res.ReceiptCode)
	}
	if h.notify.sent() != 1 {
		t.Errorf("알림 %d건, want 1", h.notify.sent())
	}
}

func TestCreateClaim_응답에_개인정보가_되돌아오지_않는다(t *testing.T) {
	// 돌려줄 이유가 없고, 응답이 캐시되거나 로그에 남으면 그게 유출 경로다.
	h := newHarness(t)
	rec := h.postClaim(defaultForm(), "key-1")

	body := rec.Body.String()
	for _, leak := range []string{"01012345678", "3333011234567", "김민수", "010-1234-5678"} {
		if strings.Contains(body, leak) {
			t.Errorf("응답에 %q 가 들어 있다: %s", leak, body)
		}
	}
}

func TestCreateClaim_멱등키_누락(t *testing.T) {
	h := newHarness(t)
	if rec := h.postClaim(defaultForm(), ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if h.store.claimCount() != 0 {
		t.Error("멱등키 없이 claim이 생성됐다")
	}
}

// ── Review Focus #1: 멱등키 중복 ────────────────────────────────────────

func TestCreateClaim_같은_멱등키는_한_건만_만든다(t *testing.T) {
	h := newHarness(t)

	first := h.postClaim(defaultForm(), "same-key")
	if first.Code != http.StatusCreated {
		t.Fatalf("첫 접수 status = %d", first.Code)
	}
	second := h.postClaim(defaultForm(), "same-key")
	if second.Code != http.StatusOK {
		t.Errorf("재시도 status = %d, want 200", second.Code)
	}

	var a, b claimResponse
	json.Unmarshal(first.Body.Bytes(), &a)  //nolint:errcheck
	json.Unmarshal(second.Body.Bytes(), &b) //nolint:errcheck
	if a.ID != b.ID {
		t.Errorf("다른 건이 만들어졌다: %s vs %s", a.ID, b.ID)
	}
	if h.store.claimCount() != 1 {
		t.Errorf("claim %d건, want 1 — 이중 환불로 이어진다", h.store.claimCount())
	}
	if h.notify.sent() != 1 {
		t.Errorf("알림 %d건, want 1 — 재시도마다 사장님을 부르면 안 된다", h.notify.sent())
	}
}

func TestCreateClaim_중복재시도의_사진은_고아로_남지_않는다(t *testing.T) {
	h := newHarness(t)
	f := defaultForm()
	f.photos = [][]byte{jpeg(1000)}

	h.postClaim(f, "same-key")
	afterFirst := h.media.count()
	h.postClaim(f, "same-key")

	if h.media.count() != afterFirst {
		t.Errorf("사진 %d개 → %d개. 재시도가 고아 파일을 남겼다", afterFirst, h.media.count())
	}
}

// ── Review Focus #2: 캐시 장애 ──────────────────────────────────────────

func TestCreateClaim_Redis가_죽어도_접수는_된다(t *testing.T) {
	// 고장난 기계 앞에 선 손님이 Redis 때문에 환불을 못 받으면 안 된다.
	h := newHarness(t, func(d *Deps) { d.Cache = failingCache{} })

	rec := h.postClaim(defaultForm(), "key-1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("캐시 장애로 접수가 막혔다: status = %d, body = %s", rec.Code, rec.Body)
	}
	if h.store.claimCount() != 1 {
		t.Error("claim이 저장되지 않았다")
	}
}

// ── Review Focus #3: 금액 경계값 ───────────────────────────────────────

func TestCreateClaim_금액_경계값(t *testing.T) {
	tests := []struct {
		name       string
		amount     string
		photos     int
		wantStatus int
	}{
		{"1원", "1", 0, http.StatusCreated},
		{"9999원 사진없음", "9999", 0, http.StatusCreated},
		{"10000원 사진없음", "10000", 0, http.StatusBadRequest},
		{"10000원 사진1장", "10000", 1, http.StatusCreated},
		{"10001원 사진1장", "10001", 1, http.StatusCreated},
		{"0원", "0", 0, http.StatusBadRequest},
		{"음수", "-5000", 0, http.StatusBadRequest},
		{"상한 정확히", "1000000", 1, http.StatusCreated},
		{"상한 초과", "1000001", 1, http.StatusBadRequest},
		{"숫자 아님", "이만원", 0, http.StatusBadRequest},
		{"빈 값", "", 0, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			f := defaultForm()
			f.amount = tc.amount
			for i := 0; i < tc.photos; i++ {
				f.photos = append(f.photos, jpeg(1000))
			}

			rec := h.postClaim(f, "key-"+tc.name)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d. body = %s", rec.Code, tc.wantStatus, rec.Body)
			}
			if tc.wantStatus == http.StatusBadRequest {
				if h.store.claimCount() != 0 {
					t.Error("거부됐는데 claim이 생성됐다")
				}
				if h.media.count() != 0 {
					t.Error("거부됐는데 사진이 저장됐다")
				}
			}
		})
	}
}

func TestCreateClaim_고액은_needs_review로_들어간다(t *testing.T) {
	h := newHarness(t)
	f := defaultForm()
	f.amount = "50000"
	f.photos = [][]byte{jpeg(1000)}

	if rec := h.postClaim(f, "key-1"); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := h.notify.claims[0].Status; got != domain.StatusNeedsReview {
		t.Errorf("Status = %s, want needs_review", got)
	}
}

// ── Review Focus #4: 비활성·없는 기계 ──────────────────────────────────

func TestCreateClaim_없거나_비활성인_기계(t *testing.T) {
	tests := []struct{ name, code string }{
		{"없는 코드", "ZZZZZZ"},
		{"형식 오류", "abc"},
		{"빈 값", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			f := defaultForm()
			f.machineCode = tc.code
			f.photos = [][]byte{jpeg(1000)}

			rec := h.postClaim(f, "key-1")
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 404 또는 400", rec.Code)
			}
			if h.store.claimCount() != 0 {
				t.Error("claim이 생성됐다")
			}
			if h.media.count() != 0 {
				t.Error("사진이 저장됐다 — 기계 확인이 사진보다 먼저여야 한다")
			}
		})
	}
}

func TestCreateClaim_비활성_기계(t *testing.T) {
	h := newHarness(t)
	m := h.store.machines["ABCD23"]
	m.Active = false
	h.store.machines["ABCD23"] = m

	f := defaultForm()
	f.photos = [][]byte{jpeg(1000)}
	rec := h.postClaim(f, "key-1")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if h.store.claimCount() != 0 || h.media.count() != 0 {
		t.Error("비활성 기계에 claim이나 사진이 남았다")
	}
}

// ── Review Focus #5: 사진 상한과 위장 파일 ─────────────────────────────

func TestCreateClaim_사진_상한과_형식(t *testing.T) {
	tests := []struct {
		name   string
		photos [][]byte
		names  []string
	}{
		{"4장", [][]byte{jpeg(100), jpeg(100), jpeg(100), jpeg(100)}, nil},
		{"5MB 초과", [][]byte{jpeg(5<<20 + 1)}, nil},
		{"텍스트를 jpg로 위장", [][]byte{[]byte("이건 사진이 아닙니다 정말로")}, []string{"real.jpg"}},
		{"ELF 실행파일", [][]byte{append([]byte{0x7F, 'E', 'L', 'F'}, make([]byte, 100)...)}, []string{"a.jpg"}},
		{"SVG", [][]byte{[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}, []string{"a.jpg"}},
		{"빈 파일", [][]byte{{}}, []string{"a.jpg"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			f := defaultForm()
			f.photos, f.photoNames = tc.photos, tc.names

			rec := h.postClaim(f, "key-1")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body = %s", rec.Code, rec.Body)
			}
			if h.store.claimCount() != 0 {
				t.Error("거부됐는데 claim이 생성됐다")
			}
			if h.media.count() != 0 {
				t.Errorf("거부됐는데 사진 %d개가 저장됐다", h.media.count())
			}
		})
	}
}

func TestCreateClaim_사진_3장은_허용(t *testing.T) {
	h := newHarness(t)
	f := defaultForm()
	f.photos = [][]byte{jpeg(1000), jpeg(1000), jpeg(1000)}

	if rec := h.postClaim(f, "key-1"); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if h.media.count() != 3 {
		t.Errorf("사진 %d장 저장, want 3", h.media.count())
	}
}

func TestCreateClaim_저장실패시_사진이_남지_않는다(t *testing.T) {
	h := newHarness(t)
	h.store.createErr = fmt.Errorf("DB 연결 끊김")

	f := defaultForm()
	f.photos = [][]byte{jpeg(1000), jpeg(1000)}

	if rec := h.postClaim(f, "key-1"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if h.media.count() != 0 {
		t.Errorf("저장 실패 후 사진 %d개가 남았다", h.media.count())
	}
}

// ── 입력 검증 ───────────────────────────────────────────────────────────

func TestCreateClaim_필드_검증(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*formOpts)
	}{
		{"증상 없음", func(f *formOpts) { f.issueType = "" }},
		{"알 수 없는 증상", func(f *formOpts) { f.issueType = "broken" }},
		{"전화번호 유선", func(f *formOpts) { f.phone = "0212345678" }},
		{"전화번호 형식오류", func(f *formOpts) { f.phone = "010-1234-567a" }},
		{"전화번호 없음", func(f *formOpts) { f.phone = "" }},
		{"은행 없음", func(f *formOpts) { f.bankCode = "" }},
		{"알 수 없는 은행", func(f *formOpts) { f.bankCode = "999" }},
		{"계좌 너무 짧음", func(f *formOpts) { f.account = "123" }},
		{"계좌 너무 김", func(f *formOpts) { f.account = strings.Repeat("1", 21) }},
		{"예금주 없음", func(f *formOpts) { f.holder = "" }},
		{"예금주 너무 김", func(f *formOpts) { f.holder = strings.Repeat("가", 41) }},
		{"설명 너무 김", func(f *formOpts) { f.description = strings.Repeat("가", 1001) }},
		{"결제 수단 없음", func(f *formOpts) { f.paymentMethod = "" }},
		{"알 수 없는 결제 수단", func(f *formOpts) { f.paymentMethod = "point" }},
		{"카드인데 뒷자리 없음", func(f *formOpts) { f.paymentMethod = "card" }},
		{"카드 뒷자리 자릿수 오류", func(f *formOpts) {
			f.paymentMethod = "card"
			f.cardLast4 = "123"
		}},
		{"카드 뒷자리가 숫자가 아님", func(f *formOpts) {
			f.paymentMethod = "card"
			f.cardLast4 = "12a4"
		}},
		{"카드 결제 시각 형식 오류", func(f *formOpts) {
			f.paymentMethod = "card"
			f.cardLast4 = "1234"
			f.paidAtGuess = "어제 저녁"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			f := defaultForm()
			tc.mut(&f)

			rec := h.postClaim(f, "key-1")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body = %s", rec.Code, rec.Body)
			}
			// 에러 메시지는 손님이 읽는다. 한국어여야 하고 비어 있으면 안 된다.
			var e errorBody
			json.Unmarshal(rec.Body.Bytes(), &e) //nolint:errcheck
			if strings.TrimSpace(e.Message) == "" {
				t.Error("에러 메시지가 비었다 — 손님이 뭘 고쳐야 할지 모른다")
			}
		})
	}
}

func TestCreateClaim_레이트리밋(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < ipClaimsPerHour; i++ {
		if rec := h.postClaim(defaultForm(), fmt.Sprintf("key-%d", i)); rec.Code != http.StatusCreated {
			t.Fatalf("%d번째 접수 실패: %d", i, rec.Code)
		}
	}
	rec := h.postClaim(defaultForm(), "key-over")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

func TestCreateClaim_빠른_중복제출은_409(t *testing.T) {
	h := newHarness(t)
	h.store.facts.LastSameMachineAt = time.Now().Add(-1 * time.Minute)

	rec := h.postClaim(defaultForm(), "key-1")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if h.store.claimCount() != 0 {
		t.Error("중복인데 claim이 생성됐다")
	}
}

func TestCreateClaim_차단된_연락처도_정상처럼_보인다(t *testing.T) {
	// 차단 사실을 알려주면 번호를 바꿔가며 우회하는 법을 학습시킨다.
	h := newHarness(t)
	h.store.facts.ManualStatus = domain.ContactBlocked

	rec := h.postClaim(defaultForm(), "key-1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d — 차단 대상도 접수는 받아야 한다", rec.Code)
	}

	body := rec.Body.String()
	for _, leak := range []string{"차단", "보류", "blocked", "on_hold", "거부"} {
		if strings.Contains(body, leak) {
			t.Errorf("응답에 %q 가 노출됐다: %s", leak, body)
		}
	}
	// 내부적으로는 보류여야 한다.
	if got := h.notify.claims[0].Status; got != domain.StatusOnHold {
		t.Errorf("내부 Status = %s, want on_hold", got)
	}
}

func TestCreateClaim_알림실패는_접수를_막지_않는다(t *testing.T) {
	h := newHarness(t)
	h.notify.sendErr = fmt.Errorf("slack 502")

	if rec := h.postClaim(defaultForm(), "key-1"); rec.Code != http.StatusCreated {
		t.Errorf("알림 실패로 접수가 막혔다: %d", rec.Code)
	}
}

func TestCreateClaim_리스크조회_실패는_500(t *testing.T) {
	h := newHarness(t)
	h.store.riskErr = fmt.Errorf("DB 타임아웃")

	rec := h.postClaim(defaultForm(), "key-1")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if h.media.count() != 0 {
		t.Error("리스크 조회 실패 후 사진이 남았다")
	}
	// DB 에러 원문이 밖으로 나가면 안 된다.
	if strings.Contains(rec.Body.String(), "타임아웃") {
		t.Errorf("내부 에러가 노출됐다: %s", rec.Body)
	}
}

// ── 기계 조회 ───────────────────────────────────────────────────────────

func TestMachineByCode(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/api/public/machines/ABCD23", nil)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var res machineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Label != "3번 기계" || res.StoreName != "테스트 매장" {
		t.Errorf("응답 = %+v", res)
	}
	// 손님 폼이 "1만원 이상은 사진 필수"를 인라인으로 안내하려면 임계값이 필요하다.
	if res.ReviewThresholdKRW != 10000 || res.MaxPhotos != maxPhotos {
		t.Errorf("정책값이 안 내려온다: %+v", res)
	}
}

func TestMachineByCode_소문자와_혼동문자를_보정한다(t *testing.T) {
	// 스티커가 닳으면 손님이 코드를 눈으로 읽어 입력한다.
	h := newHarness(t)
	h.store.addMachine("QBCD23", "1번 기계")

	for _, typed := range []string{"qbcd23", "0BCD23", "OBCD23", "QBCD-23"} {
		req := httptest.NewRequest(http.MethodGet, "/api/public/machines/"+typed, nil)
		rec := httptest.NewRecorder()
		h.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%q 입력이 404가 됐다", typed)
		}
	}
}

func TestMachineByCode_없으면_매장연락처를_안내한다(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/api/public/machines/ZZZZZZ", nil)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body) //nolint:errcheck
	if body["storePhone"] != "0212345678" {
		t.Errorf("매장 연락처가 없다: %v — 손님이 어디로 연락할지 모른다", body)
	}
}

func TestBanks(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/api/public/banks", nil)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var res struct {
		Banks []payout.Bank `json:"banks"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Banks) < 10 {
		t.Errorf("은행 %d개", len(res.Banks))
	}
}

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestReceiptCode(t *testing.T) {
	got := receiptCode("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	if got != "A1B2C3D4" {
		t.Errorf("receiptCode = %q", got)
	}
	if receiptCode("abc") != "ABC" {
		t.Errorf("짧은 입력 처리 실패")
	}
}

func TestComma(t *testing.T) {
	tests := map[int]string{0: "0", 999: "999", 1000: "1,000", 10000: "10,000", 1000000: "1,000,000"}
	for in, want := range tests {
		if got := comma(in); got != want {
			t.Errorf("comma(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateClaim_재시도는_중복규칙보다_멱등키가_먼저다(t *testing.T) {
	// 회귀 테스트. 실제로 띄워보니 이 순서가 뒤바뀌어 있었다.
	//
	// 첫 접수가 끝나면 RiskFactsFor는 LastSameMachineAt을 방금 시각으로
	// 돌려준다. 그 상태에서 같은 멱등키로 재시도하면 "빠른 중복 제출"
	// 규칙이 409로 막아버렸다. 네트워크가 끊겨 다시 보낸 손님 — 멱등키가
	// 존재하는 바로 그 상황 — 이 접수번호 대신 에러를 받는다.
	h := newHarness(t)

	first := h.postClaim(defaultForm(), "retry-key")
	if first.Code != http.StatusCreated {
		t.Fatalf("첫 접수 status = %d", first.Code)
	}

	// 첫 접수 직후의 현실을 재현한다.
	h.store.facts.LastSameMachineAt = time.Now().Add(-30 * time.Second)
	h.store.facts.PhoneClaims30d = 2

	second := h.postClaim(defaultForm(), "retry-key")
	if second.Code != http.StatusOK {
		t.Fatalf("재시도 status = %d, want 200. body = %s", second.Code, second.Body)
	}

	var a, b claimResponse
	json.Unmarshal(first.Body.Bytes(), &a)  //nolint:errcheck
	json.Unmarshal(second.Body.Bytes(), &b) //nolint:errcheck
	if a.ID != b.ID {
		t.Errorf("재시도가 다른 건을 돌려줬다: %s vs %s", a.ID, b.ID)
	}
	if h.store.claimCount() != 1 {
		t.Errorf("claim %d건, want 1", h.store.claimCount())
	}
}

func TestCreateClaim_다른_멱등키의_빠른재제출은_여전히_막는다(t *testing.T) {
	// 위 수정이 중복 방어를 통째로 없애지 않았는지 확인한다.
	// 같은 손님이 새로고침해서 새 키로 또 내는 경우는 막아야 한다.
	h := newHarness(t)

	if rec := h.postClaim(defaultForm(), "key-1"); rec.Code != http.StatusCreated {
		t.Fatalf("첫 접수 status = %d", rec.Code)
	}
	h.store.facts.LastSameMachineAt = time.Now().Add(-30 * time.Second)

	rec := h.postClaim(defaultForm(), "key-2-다른키")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 — 새 키로 낸 빠른 재제출은 막아야 한다", rec.Code)
	}
	if h.store.claimCount() != 1 {
		t.Errorf("claim %d건, want 1", h.store.claimCount())
	}
}
