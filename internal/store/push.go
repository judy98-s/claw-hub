package store

import (
	"context"
	"fmt"
	"time"
)

// PushSubscription은 브라우저 하나의 푸시 구독이다.
type PushSubscription struct {
	ID       string
	StoreID  string
	UserID   string
	Endpoint string
	P256dh   string
	Auth     string
}

// SavePushSubscription은 구독을 저장한다. 같은 endpoint 면 덮어쓴다.
//
// 브라우저는 권한을 다시 묻거나 앱을 다시 열 때 같은 endpoint 로 다시
// 구독한다. INSERT 만 하면 UNIQUE 위반이고, 지우고 넣으면 그 사이에 온
// 알림을 놓친다.
func (s *Store) SavePushSubscription(ctx context.Context, sub PushSubscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (store_id, user_id, endpoint, p256dh, auth)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (endpoint) DO UPDATE
		   SET store_id = EXCLUDED.store_id,
		       user_id  = EXCLUDED.user_id,
		       p256dh   = EXCLUDED.p256dh,
		       auth     = EXCLUDED.auth,
		       failures = 0`,
		sub.StoreID, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth)
	if err != nil {
		return fmt.Errorf("푸시 구독 저장: %w", err)
	}
	return nil
}

// DeletePushSubscription은 구독을 지운다. 없는 것을 지워도 성공이다 —
// 끄기를 두 번 눌렀다고 에러를 보여줄 이유가 없다.
func (s *Store) DeletePushSubscription(ctx context.Context, endpoint string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint=$1`, endpoint); err != nil {
		return fmt.Errorf("푸시 구독 삭제: %w", err)
	}
	return nil
}

// PushSubscriptionsFor는 한 매장의 살아 있는 구독을 반환한다.
func (s *Store) PushSubscriptionsFor(ctx context.Context, storeID string) ([]PushSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, store_id, user_id, endpoint, p256dh, auth
		  FROM push_subscriptions WHERE store_id=$1`, storeID)
	if err != nil {
		return nil, fmt.Errorf("푸시 구독 조회: %w", err)
	}
	defer rows.Close()

	var out []PushSubscription
	for rows.Next() {
		var p PushSubscription
		if err := rows.Scan(&p.ID, &p.StoreID, &p.UserID, &p.Endpoint, &p.P256dh, &p.Auth); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkPushSent는 발송 결과를 기록한다.
//
// gone 이면 브라우저가 "이 구독은 끝났다"고 답한 것이다(404/410). 그래도
// 바로 지우지 않고 두 번까지 봐준다 — 네트워크 장애를 404 로 오인해
// 멀쩡한 구독을 날리면, 사장님은 알림이 끊긴 걸 한참 뒤에 안다.
func (s *Store) MarkPushSent(ctx context.Context, id string, gone bool, now time.Time) error {
	if !gone {
		_, err := s.pool.Exec(ctx,
			`UPDATE push_subscriptions SET failures=0, last_sent_at=$1 WHERE id=$2`, now, id)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE push_subscriptions SET failures = failures + 1 WHERE id=$1`, id)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE id=$1 AND failures >= 3`, id)
	return err
}
