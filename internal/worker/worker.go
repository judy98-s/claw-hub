// Package worker는 주기 작업을 돌린다.
//
// 기계 점검 알림, 일일 요약, 사진 만료 삭제. 전부 시각을 인자로 받으므로
// 테스트에서 시간을 건너뛸 수 있다.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/judy98-s/claw-hub/internal/cache"
	"github.com/judy98-s/claw-hub/internal/media"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/store"
)

// Store는 worker가 저장소에 요구하는 전부다.
type Store interface {
	MachinesOverThreshold(ctx context.Context, threshold int, now time.Time) ([]store.MachineActivity, error)
	DailyDigest(ctx context.Context, day time.Time) ([]store.DigestRow, error)
	ExpiredPhotos(ctx context.Context, now time.Time, limit int) ([]store.Photo, error)
	DeletePhotoRow(ctx context.Context, id string) error
}

type Worker struct {
	store     Store
	cache     cache.Cache
	media     media.Storage
	notify    notify.Notifier
	threshold int
}

func New(s Store, c cache.Cache, m media.Storage, n notify.Notifier, alertThreshold int) *Worker {
	return &Worker{store: s, cache: c, media: m, notify: n, threshold: alertThreshold}
}

// CheckMachineAlerts는 24시간 내 신고가 몰린 기계를 사장님에게 알린다.
//
// 같은 기계를 24시간에 한 번만 알린다. 임계치를 넘은 뒤로는 매 실행마다
// 조건이 참이므로, 디듀프가 없으면 5분마다 같은 알림이 온다. 그러면
// 사장님은 알림을 끄고, 끄는 순간 이 기능은 없는 것이 된다.
func (w *Worker) CheckMachineAlerts(ctx context.Context, now time.Time) error {
	machines, err := w.store.MachinesOverThreshold(ctx, w.threshold, now)
	if err != nil {
		return fmt.Errorf("기계 활동 조회: %w", err)
	}

	for _, m := range machines {
		key := fmt.Sprintf("alert:machine:%s:%s", m.MachineID, now.Format("2006-01-02"))
		fresh, err := w.cache.SetNX(ctx, key, "1", 24*time.Hour)
		if err != nil {
			// 캐시가 죽었으면 알림을 건너뛴다. 중복 알림으로 사장님을
			// 괴롭히는 것보다 한 번 놓치는 게 낫다 — 대시보드에도 떠 있다.
			slog.Warn("알림 중복 확인 실패 — 건너뜀", "err", err, "machine", m.Label)
			continue
		}
		if !fresh {
			continue
		}

		if err := w.notify.MachineAlert(ctx, notify.MachineNotice{
			MachineLabel: m.Label, Count: m.Count, ByIssue: m.ByIssue,
		}); err != nil {
			slog.Warn("기계 알림 실패", "err", err, "machine", m.Label)
		}
	}
	return nil
}

// SendDailyDigest는 지정한 날짜의 요약을 보낸다.
func (w *Worker) SendDailyDigest(ctx context.Context, day time.Time) error {
	rows, err := w.store.DailyDigest(ctx, day)
	if err != nil {
		return fmt.Errorf("일일 요약 집계: %w", err)
	}

	for _, r := range rows {
		lines := make([]notify.MachineLine, 0, len(r.TopMachines))
		for _, m := range r.TopMachines {
			lines = append(lines, notify.MachineLine{Label: m.Label, Count: m.Count})
		}
		if err := w.notify.DailyDigest(ctx, notify.DigestNotice{
			Day:        day.Format("2006-01-02"),
			ClaimCount: r.ClaimCount, PaidCount: r.PaidCount, PaidTotalKRW: r.PaidTotalKRW,
			TopMachines: lines,
		}); err != nil {
			slog.Warn("일일 요약 전송 실패", "err", err, "store", r.StoreID)
		}
	}
	return nil
}

// purgeBatch는 한 번에 처리할 만료 사진 수다.
const purgeBatch = 200

// PurgeExpiredPhotos는 보존 기간이 지난 사진을 지운다.
//
// 파일 삭제가 실패해도 DB 행은 지운다. 파일이 이미 없는 경우가 대부분이고,
// 행을 남겨두면 워커가 매 실행마다 같은 사진을 다시 시도하며 로그만 채운다.
func (w *Worker) PurgeExpiredPhotos(ctx context.Context, now time.Time) (int, error) {
	photos, err := w.store.ExpiredPhotos(ctx, now, purgeBatch)
	if err != nil {
		return 0, fmt.Errorf("만료 사진 조회: %w", err)
	}

	deleted := 0
	for _, p := range photos {
		if err := w.media.Delete(ctx, p.ObjectKey); err != nil {
			slog.Warn("사진 파일 삭제 실패 — DB 행은 지움", "err", err, "key", p.ObjectKey)
		}
		if err := w.store.DeletePhotoRow(ctx, p.ID); err != nil {
			return deleted, fmt.Errorf("사진 행 삭제: %w", err)
		}
		deleted++
	}
	return deleted, nil
}

// kst는 일일 요약 시각의 기준 시간대다.
var kst = time.FixedZone("KST", 9*60*60)

// digestHour는 일일 요약을 보내는 시각(KST)이다.
// 아침 9시. 문을 열기 전에 어제 무슨 일이 있었는지 보게 한다.
const digestHour = 9

// Run은 주기 작업 루프를 돈다. ctx가 끝나면 반환한다.
func (w *Worker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastDigest := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()

			if err := w.CheckMachineAlerts(ctx, now); err != nil {
				slog.Error("기계 알림 확인 실패", "err", err)
			}
			if n, err := w.PurgeExpiredPhotos(ctx, now); err != nil {
				slog.Error("만료 사진 삭제 실패", "err", err)
			} else if n > 0 {
				slog.Info("만료 사진 삭제", "count", n)
			}

			// 하루에 한 번. 날짜가 바뀌고 digestHour를 지났을 때.
			nowKST := now.In(kst)
			today := nowKST.Format("2006-01-02")
			if nowKST.Hour() >= digestHour && lastDigest != today {
				if err := w.SendDailyDigest(ctx, nowKST.AddDate(0, 0, -1)); err != nil {
					slog.Error("일일 요약 실패", "err", err)
				} else {
					lastDigest = today
				}
			}
		}
	}
}
