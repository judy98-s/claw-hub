package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/judy98-s/claw-hub/internal/store"
)

// fakeSubs는 저장소 대신이다. 발송 결과 기록까지 본다.
type fakeSubs struct {
	mu    sync.Mutex
	subs  []store.PushSubscription
	marks []struct {
		id   string
		gone bool
	}
	err error
}

func (f *fakeSubs) PushSubscriptionsFor(_ context.Context, storeID string) ([]store.PushSubscription, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []store.PushSubscription
	for _, s := range f.subs {
		if s.StoreID == storeID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSubs) MarkPushSent(_ context.Context, id string, gone bool, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, struct {
		id   string
		gone bool
	}{id, gone})
	return nil
}

// 진짜 브라우저 구독 키 한 쌍을 만들어, 진짜 암호화 경로를 태운다.
// 모의 객체로 때우면 VAPID 서명이나 RFC 8291 암호화가 깨져도 통과한다.
func realSubscription(t *testing.T, endpoint string) store.PushSubscription {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	_ = priv
	// p256dh 는 브라우저의 공개키 자리다. VAPID 공개키와 같은 P-256
	// uncompressed point 형식이라 그대로 쓸 수 있다.
	return store.PushSubscription{
		ID: "sub-1", StoreID: "store-1", UserID: "user-1",
		Endpoint: endpoint,
		P256dh:   pub,
		// auth 는 16바이트 base64url 이다.
		Auth: "aGVsbG93b3JsZDEyMzQ1Ng",
	}
}

func TestWebPush_실제로_암호화해서_보낸다(t *testing.T) {
	var got struct {
		mu     sync.Mutex
		hits   int
		auth   string
		enc    string
		ttl    string
		body   []byte
		urgent string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.mu.Lock()
		defer got.mu.Unlock()
		got.hits++
		got.auth = r.Header.Get("Authorization")
		got.enc = r.Header.Get("Content-Encoding")
		got.ttl = r.Header.Get("TTL")
		got.urgent = r.Header.Get("Urgency")
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	subs := &fakeSubs{subs: []store.PushSubscription{realSubscription(t, srv.URL)}}
	wp := NewWebPush(subs, pub, priv, "mailto:owner@example.com", "https://example.test")

	err = wp.ClaimCreated(context.Background(), ClaimNotice{
		ClaimID: "c1", StoreID: "store-1",
		MachineLabel: "7번 기계", IssueLabel: "돈만 빠짐",
		PaymentLabel: "현금", AmountKRW: 2000,
	})
	if err != nil {
		t.Fatalf("발송 실패: %v", err)
	}

	got.mu.Lock()
	defer got.mu.Unlock()
	if got.hits != 1 {
		t.Fatalf("요청 %d회, want 1", got.hits)
	}
	if got.auth == "" {
		t.Error("Authorization(VAPID) 헤더가 없다")
	}
	if got.enc != "aes128gcm" {
		t.Errorf("Content-Encoding = %q, want aes128gcm", got.enc)
	}
	if got.ttl == "" || got.ttl == "0" {
		t.Errorf("TTL = %q — 0 이면 폰이 꺼져 있을 때 버려진다", got.ttl)
	}
	if got.urgent != "high" {
		t.Errorf("Urgency = %q, want high — 절전 모드에서도 깨워야 한다", got.urgent)
	}

	// 본문은 암호문이어야 한다. 평문이 그대로 나가면 푸시 서비스가
	// 민원 내용을 읽을 수 있다.
	if len(got.body) == 0 {
		t.Fatal("본문이 비었다")
	}
	for _, leak := range []string{"7번 기계", "돈만 빠짐", "2,000"} {
		if containsBytes(got.body, leak) {
			t.Errorf("본문에 평문 %q 가 보인다 — 암호화가 안 됐다", leak)
		}
	}

	if len(subs.marks) != 1 || subs.marks[0].gone {
		t.Errorf("발송 기록 = %+v, 201 인데 gone 으로 표시됐다", subs.marks)
	}
}

// 410 Gone 은 "이 구독은 끝났다"는 뜻이다. 그 외 오류는 일시적일 수
// 있으므로 구독을 건드리면 안 된다.
func TestWebPush_410은_죽은_구독으로_표시한다(t *testing.T) {
	for _, c := range []struct {
		status   int
		wantGone bool
	}{
		{http.StatusGone, true},
		{http.StatusNotFound, true},
		{http.StatusInternalServerError, false},
		{http.StatusTooManyRequests, false},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
		}))
		priv, pub, _ := webpush.GenerateVAPIDKeys()
		subs := &fakeSubs{subs: []store.PushSubscription{realSubscription(t, srv.URL)}}
		wp := NewWebPush(subs, pub, priv, "mailto:a@b.c", "https://example.test")

		if err := wp.ClaimCreated(context.Background(), ClaimNotice{
			ClaimID: "c1", StoreID: "store-1", MachineLabel: "7번", IssueLabel: "x",
		}); err != nil {
			t.Fatalf("%d: %v", c.status, err)
		}
		if len(subs.marks) != 1 || subs.marks[0].gone != c.wantGone {
			t.Errorf("%d -> gone=%v, want %v", c.status, subs.marks[0].gone, c.wantGone)
		}
		srv.Close()
	}
}

// 구독이 없으면 조용히 끝낸다. 에러로 만들면 푸시를 안 켠 사장님의
// 접수마다 경고 로그가 쌓인다.
func TestWebPush_구독이_없으면_아무것도_안_한다(t *testing.T) {
	subs := &fakeSubs{}
	wp := NewWebPush(subs, "pub", "priv", "mailto:a@b.c", "https://example.test")
	if err := wp.ClaimCreated(context.Background(), ClaimNotice{StoreID: "store-1"}); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

// storeID 가 비면 보낼 곳을 모른다. 조용히 넘어가면 왜 알림이 안 오는지
// 찾을 수 없다.
func TestWebPush_storeID가_없으면_에러다(t *testing.T) {
	wp := NewWebPush(&fakeSubs{}, "pub", "priv", "mailto:a@b.c", "https://example.test")
	if err := wp.ClaimCreated(context.Background(), ClaimNotice{}); err == nil {
		t.Error("storeID 없이 성공했다")
	}
}

// 하루 요약은 읽고 끝나는 정보다. 매일 아침 잠금화면에 뜨면 사장님은
// 알림을 통째로 끄고, 그러면 진짜 급한 민원 알림까지 같이 잃는다.
func TestWebPush_일일요약은_푸시하지_않는다(t *testing.T) {
	subs := &fakeSubs{subs: []store.PushSubscription{{StoreID: "store-1", Endpoint: "https://x/y"}}}
	wp := NewWebPush(subs, "pub", "priv", "mailto:a@b.c", "https://example.test")
	if err := wp.DailyDigest(context.Background(), DigestNotice{StoreID: "store-1"}); err != nil {
		t.Fatal(err)
	}
	if len(subs.marks) != 0 {
		t.Error("일일 요약이 푸시로 나갔다")
	}
}

func containsBytes(b []byte, s string) bool {
	return len(s) > 0 && len(b) >= len(s) && indexOf(b, []byte(s)) >= 0
}

func indexOf(h, n []byte) int {
	for i := 0; i+len(n) <= len(h); i++ {
		match := true
		for j := range n {
			if h[i+j] != n[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
