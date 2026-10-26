// Package app은 설정에서 의존성을 조립한다.
//
// api와 worker가 같은 조립 코드를 쓴다. 두 프로세스가 서로 다른 방식으로
// DB에 붙거나 다른 정책값을 읽으면 원인을 찾기 어려운 차이가 생긴다.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/judy98-s/claw-hub/internal/cache"
	"github.com/judy98-s/claw-hub/internal/config"
	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/media"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

// App은 조립된 의존성 묶음이다.
type App struct {
	Config config.Config
	Store  *store.Store
	Cache  cache.Cache
	Media  media.Storage
	Notify notify.Notifier
	Payout payout.Payout
}

// Build는 설정을 읽고 전부 연결한다.
func Build(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	cipher, err := crypto.NewCipher(cfg.DataEncryptionKey)
	if err != nil {
		return nil, err
	}
	hasher, err := crypto.NewHasher(cfg.HashPepper)
	if err != nil {
		return nil, err
	}

	st, err := store.Open(ctx, cfg.DatabaseURL, cipher, hasher)
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(ctx); err != nil {
		st.Close()
		return nil, fmt.Errorf("마이그레이션: %w", err)
	}

	c, err := cache.New(cfg.RedisURL)
	if err != nil {
		st.Close()
		return nil, err
	}
	if cfg.RedisURL == "" {
		slog.Info("REDIS_URL이 비어 있어 인메모리 캐시를 씁니다. 인스턴스를 여러 대 띄우면 레이트리밋이 인스턴스별로 따로 세어집니다.")
	}

	md, err := media.NewLocal(cfg.MediaDir)
	if err != nil {
		st.Close()
		return nil, err
	}

	// 채널은 여러 개를 동시에 쓴다. 하나가 죽어도 나머지는 보낸다 —
	// Slack 웹훅이 만료됐다고 푸시까지 멈추면 사장님은 알림이 아예
	// 안 오는 걸로 겪는다.
	var channels []notify.Notifier
	if cfg.SlackWebhookURL != "" {
		channels = append(channels, notify.NewSlack(cfg.SlackWebhookURL, cfg.PublicBaseURL))
	} else {
		slog.Warn("SLACK_WEBHOOK_URL이 비어 있어 Slack 알림이 비활성화됩니다. 대시보드는 그대로 동작합니다.")
	}

	switch {
	case cfg.VapidPublicKey != "" && cfg.VapidPrivateKey != "" && cfg.VapidSubject != "":
		channels = append(channels, notify.NewWebPush(st,
			cfg.VapidPublicKey, cfg.VapidPrivateKey, cfg.VapidSubject, cfg.PublicBaseURL))
	case cfg.VapidPublicKey != "" || cfg.VapidPrivateKey != "" || cfg.VapidSubject != "":
		// 일부만 채운 것은 거의 항상 실수다. 조용히 끄면 왜 푸시가
		// 안 오는지 찾느라 하루를 쓴다.
		slog.Warn("VAPID 설정이 일부만 채워져 웹 푸시가 비활성화됩니다. " +
			"VAPID_PUBLIC_KEY, VAPID_PRIVATE_KEY, VAPID_SUBJECT 가 모두 필요합니다.")
	default:
		slog.Info("VAPID 키가 없어 웹 푸시가 비활성화됩니다.")
	}

	var nt notify.Notifier = notify.NewNoop()
	if len(channels) > 0 {
		nt = notify.NewMulti(channels...)
	}

	if len(cfg.PayoutDeeplinkTemplates) == 0 {
		slog.Warn("송금 딥링크 템플릿이 없습니다. 사장님 화면에 계좌 복사와 수동 기록만 표시됩니다.")
	}

	return &App{
		Config: cfg, Store: st, Cache: c, Media: md,
		Notify: nt, Payout: payout.NewDeeplink(cfg.PayoutDeeplinkTemplates),
	}, nil
}

func (a *App) Close() {
	a.Store.Close()
	if err := a.Cache.Close(); err != nil {
		slog.Warn("캐시 종료 실패", "err", err)
	}
}
