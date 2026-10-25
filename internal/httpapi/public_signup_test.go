package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// signupOpen은 가입을 열어 둔 하네스를 만든다.
//
// config.Config 의 제로값은 SignupOpen=false 다. 실제 기동은 config.Load 를
// 거쳐 기본값 true 를 받지만, 테스트는 구조체를 직접 만들기 때문에 명시해야
// 한다. 덕분에 "닫혀 있을 때" 가 기본으로 검증된다.
func signupOpen(d *Deps) { d.Config.SignupOpen = true }

func signupBody(email string) string {
	return fmt.Sprintf(`{"storeName":"홍대 인형뽑기","phone":"0212345678",`+
		`"email":%q,"password":"test-password-1234","name":"김사장"}`, email)
}

func TestSignup_성공하면_바로_로그인된다(t *testing.T) {
	h := newHarness(t, signupOpen)

	rr := h.do(http.MethodPost, "/api/public/signup", signupBody("new@example.com"), nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}

	var got userResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Email != "new@example.com" || got.Name != "김사장" {
		t.Errorf("응답 = %+v", got)
	}

	// 가입 직후 세션 쿠키가 발급되어야 한다. 방금 정한 비밀번호를 한 번 더
	// 치게 하는 단계는 이탈만 만든다.
	var session *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("세션 쿠키가 없다 — 가입 후 로그인 상태가 아니다")
	}

	me := h.do(http.MethodGet, "/api/admin/me", "", session)
	if me.Code != http.StatusOK {
		t.Fatalf("/me 상태 = %d", me.Code)
	}
}

func TestSignup_가입이_닫혀_있으면_403(t *testing.T) {
	h := newHarness(t) // SignupOpen 기본 false

	rr := h.do(http.MethodPost, "/api/public/signup", signupBody("x@example.com"), nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}
	// 404 가 아니라 403 이어야 한다. 경로가 없는 것이 아니라 닫혀 있다는
	// 뜻을 화면이 구분해서 보여줄 수 있어야 한다.

	st := h.do(http.MethodGet, "/api/public/signup", "", nil)
	var s struct{ Open bool }
	if err := json.Unmarshal(st.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Open {
		t.Error("상태 조회가 open=true 를 돌려줬다")
	}
}

func TestSignup_이메일이_겹치면_409(t *testing.T) {
	h := newHarness(t, signupOpen)

	if rr := h.do(http.MethodPost, "/api/public/signup", signupBody("dup@example.com"), nil); rr.Code != http.StatusCreated {
		t.Fatalf("첫 가입 상태 = %d", rr.Code)
	}
	rr := h.do(http.MethodPost, "/api/public/signup", signupBody("dup@example.com"), nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}
	// 로그인과 달리 중복을 숨기지 않는다. 숨기면 "가입했는데 로그인이
	// 안 된다" 는 상태에 갇힌다.
	if body := rr.Body.String(); !strings.Contains(body, "이미 가입된") {
		t.Errorf("본문 = %s", body)
	}
}

func TestSignup_입력_검증(t *testing.T) {
	cases := []struct{ name, body string }{
		{"매장 이름 없음", `{"storeName":"","email":"a@b.com","password":"test-password-1234"}`},
		{"매장 이름 공백만", `{"storeName":"   ","email":"a@b.com","password":"test-password-1234"}`},
		{"이메일 형식", `{"storeName":"가게","email":"not-an-email","password":"test-password-1234"}`},
		{"비밀번호 7자", `{"storeName":"가게","email":"a@b.com","password":"1234567"}`},
		{"본문이 JSON 아님", `당신은 누구십니까`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, signupOpen)
			rr := h.do(http.MethodPost, "/api/public/signup", c.body, nil)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestSignup_72자를_넘는_비밀번호는_거부한다(t *testing.T) {
	// bcrypt 는 72바이트를 넘으면 잘라 버린다. 조용히 자르면 사용자가 믿는
	// 비밀번호와 실제로 저장된 것이 달라진다.
	long := make([]byte, 73)
	for i := range long {
		long[i] = 'a'
	}
	h := newHarness(t, signupOpen)
	body := fmt.Sprintf(`{"storeName":"가게","email":"a@b.com","password":%q}`, string(long))
	rr := h.do(http.MethodPost, "/api/public/signup", body, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}
}

func TestSignup_IP당_하루_횟수를_넘으면_429(t *testing.T) {
	h := newHarness(t, signupOpen)

	for i := 0; i < signupsPerIPPerDay; i++ {
		rr := h.do(http.MethodPost, "/api/public/signup",
			signupBody(fmt.Sprintf("ok%d@example.com", i)), nil)
		if rr.Code != http.StatusCreated {
			t.Fatalf("%d번째 가입 상태 = %d, 본문 = %s", i+1, rr.Code, rr.Body.String())
		}
	}
	rr := h.do(http.MethodPost, "/api/public/signup", signupBody("over@example.com"), nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}
}

func TestSignup_이름을_비우면_사장님이_된다(t *testing.T) {
	h := newHarness(t, signupOpen)
	body := `{"storeName":"가게","email":"noname@example.com","password":"test-password-1234"}`
	rr := h.do(http.MethodPost, "/api/public/signup", body, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("상태 = %d, 본문 = %s", rr.Code, rr.Body.String())
	}
	var got userResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "사장님" {
		t.Errorf("이름 = %q, want 사장님", got.Name)
	}
}
