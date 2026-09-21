// Package httpapi는 HTTP 표면을 담당한다.
//
// 저장소는 좁은 인터페이스로 받는다. 핸들러 테스트에 Postgres가 필요하면
// 테스트가 느려지고, 느려지면 안 돌리게 된다.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/judy98-s/claw-hub/internal/cache"
	"github.com/judy98-s/claw-hub/internal/config"
	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/media"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

// Store는 httpapi가 저장소에 요구하는 전부다.
type Store interface {
	// 공개 접수
	MachineByCode(ctx context.Context, code string) (domain.Machine, store.StoreInfo, error)
	CreateClaim(ctx context.Context, in store.CreateClaimInput) (store.CreateClaimResult, error)
	ClaimByIdempotencyKey(ctx context.Context, storeID, key string) (store.CreateClaimResult, bool, error)
	RiskFactsFor(ctx context.Context, q store.RiskQuery) (domain.RiskInput, error)

	// 관리자
	Authenticate(ctx context.Context, email, password string) (store.User, error)
	UserByID(ctx context.Context, id string) (store.User, error)
	ListClaims(ctx context.Context, f store.ClaimFilter) ([]store.ClaimSummary, string, error)
	ClaimByID(ctx context.Context, storeID, id string, by domain.Actor) (store.ClaimDetail, error)
	ApplyTransition(ctx context.Context, storeID string, c *domain.Claim, ev domain.Event) error

	ListMachines(ctx context.Context, storeID string) ([]store.MachineStat, error)
	MachineByID(ctx context.Context, storeID, id string) (domain.Machine, error)
	CreateMachine(ctx context.Context, storeID, label, location string) (domain.Machine, error)
	UpdateMachine(ctx context.Context, storeID, id, label, location string, active bool) error

	ListContacts(ctx context.Context, storeID string) ([]store.ContactSummary, error)
	SetContactStatus(ctx context.Context, storeID string, hash []byte, kind, status, note, by string) error
	DailyStats(ctx context.Context, storeID string, from, to time.Time) ([]store.DailyStat, error)
}

// Server는 의존성을 모아 라우터를 만든다.
type Server struct {
	cfg        config.Config
	store      Store
	cache      cache.Cache
	media      media.Storage
	notify     notify.Notifier
	payout     payout.Payout
	policy     domain.Policy
	session    *sessionCodec
	claimToken *claimTokenCodec
	now        func() time.Time
}

// Deps는 Server 생성에 필요한 것들이다.
type Deps struct {
	Config config.Config
	Store  Store
	Cache  cache.Cache
	Media  media.Storage
	Notify notify.Notifier
	Payout payout.Payout
	Now    func() time.Time // 테스트용. nil이면 time.Now
}

func New(d Deps) *Server {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	p := d.Config.Policy
	return &Server{
		cfg: d.Config, store: d.Store, cache: d.Cache, media: d.Media,
		notify: d.Notify, payout: d.Payout,
		policy: domain.Policy{
			ReviewThresholdKRW:   p.ReviewThresholdKRW,
			RepeatWatchCount:     p.RepeatWatchCount,
			RepeatHoldCount:      p.RepeatHoldCount,
			AccountSharingPhones: p.AccountSharingPhones,
			PayoutCeilingKRW:     p.PayoutCeilingKRW,
			MaxAmountKRW:         p.MaxAmountKRW,
			RapidDuplicateWindow: domain.DefaultPolicy().RapidDuplicateWindow,
		},
		session:    newSessionCodec(d.Config.SessionSecret),
		claimToken: newClaimTokenCodec(d.Config.SessionSecret),
		now:        now,
	}
}

// maxRequestBytes는 요청 전체 상한이다. 사진 5MB × 3장 + 여유.
const maxRequestBytes = 16 << 20

// Handler는 전체 라우터를 반환한다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 공개 — 인증 없음, 레이트리밋 적용
	mux.HandleFunc("GET /api/public/machines/{code}", s.handleMachineByCode)
	mux.HandleFunc("GET /api/public/banks", s.handleBanks)
	mux.Handle("POST /api/public/claims", s.rateLimitPublic(http.HandlerFunc(s.handleCreateClaim)))

	// 관리자 — 세션 인증
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleLogout)
	mux.Handle("GET /api/admin/me", s.authed(s.handleMe))
	mux.Handle("GET /api/admin/claims", s.authed(s.handleListClaims))
	// 아래 라우트들은 로그인 세션 또는 Slack 링크의 서명 토큰으로 들어온다.
	// 토큰은 그 건 하나에만 통하므로, 링크가 새어도 다른 손님의 계좌는
	// 열리지 않는다.
	mux.Handle("GET /api/admin/claims/{id}", s.claimScoped(s.handleClaimDetail))
	mux.Handle("POST /api/admin/claims/{id}/approve", s.claimScoped(s.handleApprove))
	mux.Handle("POST /api/admin/claims/{id}/reject", s.claimScoped(s.handleReject))
	mux.Handle("POST /api/admin/claims/{id}/mark-paid", s.claimScoped(s.handleMarkPaid))
	mux.Handle("GET /api/admin/claims/{id}/payout-links", s.claimScoped(s.handlePayoutLinks))
	mux.Handle("GET /api/admin/claims/{id}/photos/{photoId}", s.claimScoped(s.handlePhoto))

	mux.Handle("GET /api/admin/machines", s.authed(s.handleListMachines))
	mux.Handle("POST /api/admin/machines", s.authed(s.handleCreateMachine))
	mux.Handle("PATCH /api/admin/machines/{id}", s.authed(s.handleUpdateMachine))
	mux.Handle("GET /api/admin/machines/{id}/qr.png", s.authed(s.handleMachineQR))

	mux.Handle("GET /api/admin/contacts", s.authed(s.handleListContacts))
	mux.Handle("PATCH /api/admin/contacts/{hash}", s.authed(s.handleSetContactStatus))
	mux.Handle("GET /api/admin/stats/daily", s.authed(s.handleDailyStats))

	var h http.Handler = mux
	h = withMaxBytes(maxRequestBytes, h)
	h = withRecover(h)
	h = withRequestID(h)
	return h
}
