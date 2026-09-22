// Package notify는 사장님에게 알림을 보낸다.
//
// 알림은 부가 경로다. Slack이 죽어도 claim은 DB에 남고 대시보드에 뜬다.
// 알림 실패가 접수 실패로 번져서는 안 되므로, 호출자는 에러를 로깅만 한다.
package notify

import (
	"context"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// ClaimNotice는 새 신고 알림에 필요한 정보다.
//
// 전화번호와 계좌번호가 없다. 의도적이다 — Slack 워크스페이스는 DB보다
// 접근 통제가 느슨하고, 알림은 검색·저장·전달된다. 사장님은 링크를 눌러
// 대시보드에서 보면 되고, 그 열람은 감사 로그에 남는다.
type ClaimNotice struct {
	ClaimID string
	// URL은 사장님이 누를 링크다. 서명된 접근 토큰이 붙어 있어 로그인
	// 없이 그 건만 열린다. 비어 있으면 대시보드 주소로 떨어진다.
	URL          string
	MachineLabel string
	IssueLabel   string
	// PaymentLabel은 "현금" 또는 "카드"다. 사장님이 알림만 보고 단말기를
	// 켤지 송금 앱을 켤지 판단할 수 있어야 한다.
	PaymentLabel string
	AmountKRW    int
	Status       domain.Status
	RiskReasons  []domain.RiskReason
	PhotoCount   int
	CreatedAt    time.Time
}

// MachineNotice는 점검이 필요해 보이는 기계 알림이다.
type MachineNotice struct {
	MachineLabel string
	Count        int
	ByIssue      map[domain.IssueType]int
}

// DigestNotice는 하루치 요약이다.
type DigestNotice struct {
	Day          string
	ClaimCount   int
	PaidCount    int
	PaidTotalKRW int
	TopMachines  []MachineLine
}

type MachineLine struct {
	Label string
	Count int
}

// Notifier는 알림 채널이다.
//
// 지금은 Slack 하나지만 인터페이스로 둔 이유: 카카오 알림톡은 비즈니스
// 채널 개설과 심사가 필요해 2단계로 미뤘다. 그때 파일 하나만 추가하면 된다.
type Notifier interface {
	ClaimCreated(ctx context.Context, n ClaimNotice) error
	MachineAlert(ctx context.Context, n MachineNotice) error
	DailyDigest(ctx context.Context, n DigestNotice) error
}

// Noop은 알림을 보내지 않는다. SLACK_WEBHOOK_URL이 비었을 때 쓴다.
type Noop struct{}

func NewNoop() Noop                                            { return Noop{} }
func (Noop) ClaimCreated(context.Context, ClaimNotice) error   { return nil }
func (Noop) MachineAlert(context.Context, MachineNotice) error { return nil }
func (Noop) DailyDigest(context.Context, DigestNotice) error   { return nil }
