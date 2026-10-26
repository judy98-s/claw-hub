package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPushKey_키가_없으면_꺼짐으로_알린다(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodGet, "/api/admin/push/key", "", h.login(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var res struct {
		PublicKey string `json:"publicKey"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	// 테스트 설정에는 VAPID 키가 없다. 화면은 이 값을 보고 버튼을 숨긴다 —
	// 눌러봤자 실패하는 버튼을 보여주는 것보다 없는 게 낫다.
	if res.Enabled {
		t.Error("키가 없는데 enabled=true")
	}
}

func TestPushSubscribe_저장하고_지운다(t *testing.T) {
	h := newHarness(t)
	jar := h.login(t)

	body := `{"endpoint":"https://fcm.googleapis.com/fcm/send/abc123",
	          "keys":{"p256dh":"BNcRd...","auth":"tBHI..."}}`
	if rec := h.do(http.MethodPost, "/api/admin/push/subscribe", body, jar); rec.Code != http.StatusOK {
		t.Fatalf("구독 status = %d, body = %s", rec.Code, rec.Body)
	}
	if n := len(h.store.pushSubs); n != 1 {
		t.Fatalf("구독 %d개, want 1", n)
	}

	// 같은 endpoint 로 다시 구독해도 한 개여야 한다. 브라우저는 권한을
	// 다시 묻거나 앱을 다시 열 때 같은 endpoint 로 또 구독한다.
	if rec := h.do(http.MethodPost, "/api/admin/push/subscribe", body, jar); rec.Code != http.StatusOK {
		t.Fatalf("재구독 status = %d", rec.Code)
	}
	if n := len(h.store.pushSubs); n != 1 {
		t.Errorf("재구독 후 %d개 — 중복 발송이 된다", n)
	}

	un := `{"endpoint":"https://fcm.googleapis.com/fcm/send/abc123"}`
	if rec := h.do(http.MethodPost, "/api/admin/push/unsubscribe", un, jar); rec.Code != http.StatusOK {
		t.Fatalf("해지 status = %d", rec.Code)
	}
	if n := len(h.store.pushSubs); n != 0 {
		t.Errorf("해지 후 %d개", n)
	}

	// 없는 것을 또 지워도 성공이다 — 끄기를 두 번 눌렀다고 에러를
	// 보여줄 이유가 없다.
	if rec := h.do(http.MethodPost, "/api/admin/push/unsubscribe", un, jar); rec.Code != http.StatusOK {
		t.Errorf("두 번째 해지 status = %d", rec.Code)
	}
}

// endpoint 는 푸시 서비스 주소다. 아무 문자열이나 저장하면 발송 때마다
// 실패하고 실패 로그만 쌓인다.
func TestPushSubscribe_잘못된_구독은_거절한다(t *testing.T) {
	h := newHarness(t)
	jar := h.login(t)

	for _, body := range []string{
		`{"endpoint":"http://insecure.example/x","keys":{"p256dh":"a","auth":"b"}}`,
		`{"endpoint":"not-a-url","keys":{"p256dh":"a","auth":"b"}}`,
		`{"endpoint":"https://ok.example/x","keys":{"p256dh":"","auth":"b"}}`,
		`{"endpoint":"https://ok.example/x","keys":{"p256dh":"a","auth":""}}`,
		`{"endpoint":"","keys":{"p256dh":"a","auth":"b"}}`,
	} {
		if rec := h.do(http.MethodPost, "/api/admin/push/subscribe", body, jar); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 — %s", rec.Code, body)
		}
	}
	if n := len(h.store.pushSubs); n != 0 {
		t.Errorf("거절했는데 %d개가 저장됐다", n)
	}
}

func TestPush_로그인이_필요하다(t *testing.T) {
	h := newHarness(t)
	for _, p := range []string{"/api/admin/push/key", "/api/admin/push/subscribe"} {
		method := http.MethodGet
		body := ""
		if p != "/api/admin/push/key" {
			method, body = http.MethodPost, `{"endpoint":"https://x/y","keys":{"p256dh":"a","auth":"b"}}`
		}
		if rec := h.do(method, p, body, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", p, rec.Code)
		}
	}
}
