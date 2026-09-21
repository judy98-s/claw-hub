// Command api는 HTTP 서버를 띄운다.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/judy98-s/claw-hub/internal/app"
	"github.com/judy98-s/claw-hub/internal/httpapi"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.Build(ctx)
	if err != nil {
		// 설정이 잘못됐으면 여기서 멈춘다. 암호화 키 없이 개인정보를 받는
		// 상태로 떠 있는 것보다 안 뜨는 게 낫다.
		slog.Error("기동 실패", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	srv := &http.Server{
		Addr: a.Config.Addr,
		Handler: httpapi.New(httpapi.Deps{
			Config: a.Config, Store: a.Store, Cache: a.Cache,
			Media: a.Media, Notify: a.Notify, Payout: a.Payout,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// 사진 업로드가 느린 회선에서 오래 걸린다. 넉넉히 준다.
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	go func() {
		slog.Info("서버 시작", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("서버 종료", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("종료 신호 수신, 진행 중인 요청을 기다립니다")

	// 접수 처리 중에 끊기면 손님은 실패 화면을 보고 사장님은 절반 저장된
	// 데이터를 본다. 30초 준다.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("정상 종료 실패", "err", err)
	}
}
