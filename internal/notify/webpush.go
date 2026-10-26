package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/judy98-s/claw-hub/internal/store"
)

// SubscriptionStore는 푸시가 필요로 하는 저장소 부분이다.
//
// *store.Store 전체가 아니라 이 셋만 받는다 — 알림 채널이 DB 전체를
// 만질 이유가 없고, 테스트에서 가짜를 만들기도 쉽다.
type SubscriptionStore interface {
	PushSubscriptionsFor(ctx context.Context, storeID string) ([]store.PushSubscription, error)
	MarkPushSent(ctx context.Context, id string, gone bool, now time.Time) error
}

// WebPush는 사장님 브라우저로 푸시를 보낸다.
//
// 사장님 전용이다. 손님은 앱을 설치하지 않는다 — QR 을 찍고 한 번 쓰고
// 떠난다 (docs/DESIGN-V2.md §8).
type WebPush struct {
	subs    SubscriptionStore
	public  string
	private string
	// subject 는 VAPID 의 연락처다. 푸시 서비스가 문제가 생겼을 때
	// 연락할 곳이라 mailto: 또는 https: 여야 한다.
	subject string
	baseURL string
	now     func() time.Time
}

func NewWebPush(subs SubscriptionStore, public, private, subject, baseURL string) *WebPush {
	return &WebPush{
		subs: subs, public: public, private: private,
		subject: subject, baseURL: baseURL, now: time.Now,
	}
}

// pushPayload는 서비스 워커가 받는 본문이다.
//
// 금액과 기계 이름까지만 담는다. 전화번호·계좌는 넣지 않는다 — 푸시는
// 잠금화면에 뜨고, 그 화면은 누구나 본다. Slack 알림에 개인정보를 안
// 넣기로 한 것과 같은 이유다.
type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

func (w *WebPush) ClaimCreated(ctx context.Context, n ClaimNotice) error {
	url := n.URL
	if url == "" {
		url = w.baseURL + "/admin/claims"
	}
	return w.send(ctx, n.StoreID, pushPayload{
		Title: fmt.Sprintf("%s · %s", n.MachineLabel, n.IssueLabel),
		Body:  fmt.Sprintf("%s원 · %s", comma(n.AmountKRW), n.PaymentLabel),
		URL:   url,
		// 같은 건으로 여러 번 오면 알림이 쌓인다. tag 가 같으면
		// 브라우저가 덮어쓴다.
		Tag: "claim-" + n.ClaimID,
	})
}

func (w *WebPush) MachineAlert(ctx context.Context, n MachineNotice) error {
	return w.send(ctx, n.StoreID, pushPayload{
		Title: n.MachineLabel + " 점검이 필요해 보여요",
		Body:  fmt.Sprintf("24시간 내 %d건 접수됐어요.", n.Count),
		URL:   w.baseURL + "/admin/machines",
		Tag:   "machine-" + n.MachineLabel,
	})
}

// DailyDigest는 푸시로 보내지 않는다.
//
// 하루 요약은 읽고 끝나는 정보지 지금 행동할 일이 아니다. 매일 아침
// 잠금화면에 뜨면 사장님은 알림을 통째로 끄고, 그러면 진짜 급한
// 민원 알림까지 같이 잃는다.
func (w *WebPush) DailyDigest(context.Context, DigestNotice) error { return nil }

func (w *WebPush) send(ctx context.Context, storeID string, p pushPayload) error {
	if storeID == "" {
		// 매장을 모르면 보낼 곳을 모른다. 조용히 넘어가면 왜 알림이
		// 안 오는지 못 찾는다.
		return fmt.Errorf("푸시: storeID 가 비어 있다")
	}
	subs, err := w.subs.PushSubscriptionsFor(ctx, storeID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("푸시 본문: %w", err)
	}

	for _, s := range subs {
		sub := &webpush.Subscription{
			Endpoint: s.Endpoint,
			Keys:     webpush.Keys{P256dh: s.P256dh, Auth: s.Auth},
		}
		res, err := webpush.SendNotificationWithContext(ctx, body, sub, &webpush.Options{
			Subscriber:      w.subject,
			VAPIDPublicKey:  w.public,
			VAPIDPrivateKey: w.private,
			// 사장님이 폰을 보고 있지 않을 때가 대부분이다. 푸시 서비스가
			// 12시간은 들고 있다가 전달한다.
			TTL: 12 * 60 * 60,
			// 민원 알림은 지금 보라는 것이다. 절전 모드에서도 깨운다.
			Urgency: webpush.UrgencyHigh,
		})
		if err != nil {
			slog.Warn("푸시 발송 실패", "err", err, "endpoint_id", s.ID)
			continue
		}
		// 404/410 은 "이 구독은 끝났다"는 뜻이다. 그 외 오류는 일시적일
		// 수 있으므로 구독을 건드리지 않는다.
		gone := res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone
		res.Body.Close()
		if err := w.subs.MarkPushSent(ctx, s.ID, gone, w.now()); err != nil {
			slog.Warn("푸시 발송 기록 실패", "err", err, "endpoint_id", s.ID)
		}
	}
	return nil
}
