package notify

import (
	"context"
	"errors"
)

// Multi는 여러 채널에 같은 알림을 보낸다.
//
// 한 채널이 실패해도 나머지는 보낸다. Slack 웹훅이 만료됐다고 푸시까지
// 멈추면, 사장님은 알림이 아예 안 오는 걸로 겪는다. 에러는 모아서
// 돌려주고, 호출자는 지금처럼 로깅만 한다 (알림 실패가 접수 실패로
// 번져서는 안 된다).
type Multi struct {
	targets []Notifier
}

func NewMulti(targets ...Notifier) Multi {
	live := make([]Notifier, 0, len(targets))
	for _, t := range targets {
		if t != nil {
			live = append(live, t)
		}
	}
	return Multi{targets: live}
}

func (m Multi) ClaimCreated(ctx context.Context, n ClaimNotice) error {
	return m.fan(func(t Notifier) error { return t.ClaimCreated(ctx, n) })
}

func (m Multi) MachineAlert(ctx context.Context, n MachineNotice) error {
	return m.fan(func(t Notifier) error { return t.MachineAlert(ctx, n) })
}

func (m Multi) DailyDigest(ctx context.Context, n DigestNotice) error {
	return m.fan(func(t Notifier) error { return t.DailyDigest(ctx, n) })
}

func (m Multi) fan(send func(Notifier) error) error {
	var errs []error
	for _, t := range m.targets {
		if err := send(t); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
