// Command worker는 주기 작업을 돌린다.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/judy98-s/claw-hub/internal/app"
	"github.com/judy98-s/claw-hub/internal/worker"
)

// tickInterval은 주기 작업 간격이다.
// 기계 알림은 늦어도 5분 안에 나가야 사장님이 손님보다 먼저 안다.
const tickInterval = 5 * time.Minute

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.Build(ctx)
	if err != nil {
		slog.Error("기동 실패", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	w := worker.New(a.Store, a.Cache, a.Media, a.Notify, a.Config.Policy.MachineAlertCount)

	slog.Info("워커 시작", "interval", tickInterval)
	w.Run(ctx, tickInterval)
	slog.Info("워커 종료")
}
