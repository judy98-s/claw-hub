//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

func TestPushSubscription_저장_조회_삭제(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	u, err := s.CreateUser(ctx, storeID, "push@example.com", "password1", "사장", "")
	if err != nil {
		t.Fatal(err)
	}

	sub := PushSubscription{
		StoreID: storeID, UserID: u.ID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/abc",
		P256dh:   "key1", Auth: "auth1",
	}
	if err := s.SavePushSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}

	// 브라우저는 권한을 다시 묻거나 앱을 다시 열 때 같은 endpoint 로
	// 또 구독한다. INSERT 만 하면 UNIQUE 위반이고, 그러면 알림이 끊긴다.
	sub.P256dh, sub.Auth = "key2", "auth2"
	if err := s.SavePushSubscription(ctx, sub); err != nil {
		t.Fatalf("같은 endpoint 재구독 실패: %v", err)
	}

	got, err := s.PushSubscriptionsFor(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("구독 %d개, want 1 — 중복되면 알림이 두 번 간다", len(got))
	}
	if got[0].P256dh != "key2" {
		t.Errorf("P256dh = %q, 재구독 값으로 갱신돼야 한다", got[0].P256dh)
	}

	if err := s.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PushSubscriptionsFor(ctx, storeID); len(got) != 0 {
		t.Errorf("삭제 후 %d개", len(got))
	}
	// 없는 것을 또 지워도 성공이다.
	if err := s.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
		t.Errorf("두 번째 삭제: %v", err)
	}
}

// 발송이 404/410 으로 돌아와도 바로 지우지 않는다. 일시적인 장애를
// 404 로 오인해 멀쩡한 구독을 날리면, 사장님은 알림이 끊긴 걸 한참 뒤에 안다.
func TestMarkPushSent_세_번_실패해야_지운다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)
	u, err := s.CreateUser(ctx, storeID, "push@example.com", "password1", "사장", "")
	if err != nil {
		t.Fatal(err)
	}
	sub := PushSubscription{StoreID: storeID, UserID: u.ID,
		Endpoint: "https://x.example/e", P256dh: "k", Auth: "a"}
	if err := s.SavePushSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	live, _ := s.PushSubscriptionsFor(ctx, storeID)
	id := live[0].ID
	now := time.Now()

	for i := 1; i <= 2; i++ {
		if err := s.MarkPushSent(ctx, id, true, now); err != nil {
			t.Fatal(err)
		}
		got, _ := s.PushSubscriptionsFor(ctx, storeID)
		if len(got) != 1 {
			t.Fatalf("%d번째 실패에 지워졌다", i)
		}
	}
	if err := s.MarkPushSent(ctx, id, true, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PushSubscriptionsFor(ctx, storeID); len(got) != 0 {
		t.Error("세 번 실패했는데 남아 있다")
	}
}

func TestMarkPushSent_성공하면_실패_횟수를_되돌린다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)
	u, err := s.CreateUser(ctx, storeID, "push@example.com", "password1", "사장", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePushSubscription(ctx, PushSubscription{
		StoreID: storeID, UserID: u.ID,
		Endpoint: "https://x.example/e", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatal(err)
	}
	live, _ := s.PushSubscriptionsFor(ctx, storeID)
	id := live[0].ID
	now := time.Now()

	// 두 번 실패한 뒤 한 번 성공하면 다시 처음부터여야 한다. 안 그러면
	// 몇 주에 걸쳐 쌓인 일시적 실패가 멀쩡한 구독을 지운다.
	_ = s.MarkPushSent(ctx, id, true, now)
	_ = s.MarkPushSent(ctx, id, true, now)
	if err := s.MarkPushSent(ctx, id, false, now); err != nil {
		t.Fatal(err)
	}
	_ = s.MarkPushSent(ctx, id, true, now)
	if got, _ := s.PushSubscriptionsFor(ctx, storeID); len(got) != 1 {
		t.Error("성공 뒤 한 번 실패했는데 지워졌다")
	}
}

// 다른 매장의 구독으로 알림이 가면 남의 민원 내용이 넘어간다.
func TestPushSubscriptionsFor_매장_범위(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeA, _ := fixture(t, s)
	storeB, _, err := createStore(ctx, s, "남의 매장", "0212345679")
	if err != nil {
		t.Fatal(err)
	}
	uA, _ := s.CreateUser(ctx, storeA, "a@example.com", "password1", "A", "")
	uB, _ := s.CreateUser(ctx, storeB, "b@example.com", "password1", "B", "")

	_ = s.SavePushSubscription(ctx, PushSubscription{StoreID: storeA, UserID: uA.ID,
		Endpoint: "https://x.example/a", P256dh: "k", Auth: "a"})
	_ = s.SavePushSubscription(ctx, PushSubscription{StoreID: storeB, UserID: uB.ID,
		Endpoint: "https://x.example/b", P256dh: "k", Auth: "a"})

	got, err := s.PushSubscriptionsFor(ctx, storeA)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Endpoint != "https://x.example/a" {
		t.Errorf("남의 매장 구독이 섞였다: %+v", got)
	}
}
