// Package domain은 이 시스템의 규칙을 담는다.
//
// 이 패키지는 다른 internal 패키지를 import 하지 않는다. DB도 HTTP도 모르는
// 순수 코드라서 인프라 없이 전수 테스트가 가능하다. 틀리면 돈이 나가는
// 로직(상태 전이, 리스크 평가)을 전부 여기 모아둔 이유다.
package domain

import (
	"fmt"
	"time"
)

// Status는 신고 건의 처리 상태다.
type Status string

const (
	StatusPending     Status = "pending"      // 접수됨, 사장님 승인 대기
	StatusNeedsReview Status = "needs_review" // 고액이거나 리스크 플래그 — 확인 필요
	StatusOnHold      Status = "on_hold"      // 반복 신고 등으로 자동 보류
	StatusApproved    Status = "approved"     // 승인됨, 송금 대기
	StatusRejected    Status = "rejected"     // 거절 (종료)
	StatusPaid        Status = "paid"         // 송금 완료 (종료)
)

// AllStatuses는 모든 상태를 선언 순서로 반환한다.
func AllStatuses() []Status {
	return []Status{StatusPending, StatusNeedsReview, StatusOnHold, StatusApproved, StatusRejected, StatusPaid}
}

var statusLabels = map[Status]string{
	StatusPending:     "접수됨",
	StatusNeedsReview: "확인 필요",
	StatusOnHold:      "보류",
	StatusApproved:    "승인됨",
	StatusRejected:    "거절",
	StatusPaid:        "송금 완료",
}

// Label은 화면과 알림에 쓰는 한국어 표기다.
func (s Status) Label() string { return statusLabels[s] }

// IsTerminal은 더 이상 전이할 수 없는 상태인지 알려준다.
func (s Status) IsTerminal() bool { return s == StatusPaid || s == StatusRejected }

// ParseStatus는 외부 입력을 Status로 변환한다.
func ParseStatus(s string) (Status, error) {
	for _, v := range AllStatuses() {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("알 수 없는 상태: %q", s)
}

// IssueType은 손님이 고른 증상이다.
type IssueType string

const (
	IssueDollStuck  IssueType = "doll_stuck"  // 인형 걸림
	IssueCashEaten  IssueType = "cash_eaten"  // 현금 먹음
	IssueClawBroken IssueType = "claw_broken" // 집게 불량
	IssueOther      IssueType = "other"       // 기타
)

func AllIssueTypes() []IssueType {
	return []IssueType{IssueDollStuck, IssueCashEaten, IssueClawBroken, IssueOther}
}

var issueLabels = map[IssueType]string{
	IssueDollStuck:  "인형 걸림",
	IssueCashEaten:  "현금 먹음",
	IssueClawBroken: "집게 불량",
	IssueOther:      "기타",
}

func (i IssueType) Label() string { return issueLabels[i] }

func ParseIssueType(s string) (IssueType, error) {
	for _, v := range AllIssueTypes() {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("알 수 없는 증상: %q", s)
}

// Actor는 어떤 동작을 일으킨 주체다.
type Actor struct {
	Kind string // ActorCustomer | ActorOwner | ActorSystem
	ID   string
}

const (
	ActorCustomer = "customer"
	ActorOwner    = "owner"
	ActorSystem   = "system"
	// ActorLink는 Slack 알림 링크로 로그인 없이 들어온 경우다.
	// 누가 눌렀는지는 알 수 없지만, 로그인 처리와 구분되어야 감사 로그가
	// 사실을 말한다.
	ActorLink = "link"
)

// 감사 로그에 남는 동작 종류.
const (
	ActionCreate      = "create"
	ActionTransition  = "transition"
	ActionViewContact = "view_contact" // 계좌·전화번호를 복호화해 열람함
)

// Event는 감사 로그 한 줄이다. 돈이 오가므로 append-only로만 쌓는다.
type Event struct {
	ClaimID string
	Actor   Actor
	Action  string
	From    Status
	To      Status
	Note    string
	At      time.Time
}

// NewEvent는 상태 전이가 아닌 동작(생성, 열람)의 이벤트를 만든다.
func NewEvent(claimID string, by Actor, action string, from, to Status, note string) Event {
	return Event{ClaimID: claimID, Actor: by, Action: action, From: from, To: to, Note: note, At: time.Now()}
}

// PayoutMethod는 환불이 어떤 경로로 나갔는지다.
const (
	PayoutDeeplink = "deeplink" // 토스·카카오뱅크 앱으로 보냄
	PayoutManual   = "manual"   // 사장님이 다른 방법으로 보내고 수동 기록
)

// Claim은 신고 건 하나다.
type Claim struct {
	ID        string
	StoreID   string
	MachineID string

	IssueType   IssueType
	AmountKRW   int
	Description string

	Status    Status
	RiskScore int

	CreatedAt    time.Time
	ResolvedAt   time.Time
	PaidAt       time.Time
	PayoutMethod string
}

// transitions는 허용되는 상태 전이 전부다.
//
// 종료 상태(paid, rejected)는 키가 없으므로 어디로도 갈 수 없다. paid를
// 되돌릴 수 있게 하면 이중 환불이 나고, 그건 이 시스템이 막아야 할 바로 그 일이다.
var transitions = map[Status][]Status{
	StatusPending:     {StatusNeedsReview, StatusOnHold, StatusApproved, StatusRejected},
	StatusNeedsReview: {StatusOnHold, StatusApproved, StatusRejected},
	StatusOnHold:      {StatusNeedsReview, StatusApproved, StatusRejected},

	// approved→rejected를 허용하는 이유: 승인 후 송금 직전에 사진을 다시 보니
	// 조작이었다거나, 계좌가 틀려 송금이 반려되는 일이 실제로 생긴다.
	// 되돌릴 길이 없으면 사장님이 DB를 직접 고치게 된다.
	StatusApproved: {StatusPaid, StatusRejected},
}

// CanTransition은 전이 가능 여부만 알려준다. Claim을 변경하지 않는다.
func (c *Claim) CanTransition(to Status) bool {
	return c.transitionError(to) == nil
}

func (c *Claim) transitionError(to Status) error {
	if to == c.Status {
		return fmt.Errorf("이미 %s 상태입니다", c.Status.Label())
	}
	for _, allowed := range transitions[c.Status] {
		if allowed == to {
			return nil
		}
	}
	return fmt.Errorf("%s 에서 %s 로는 바꿀 수 없습니다", c.Status.Label(), to.Label())
}

// Transition은 상태를 바꾸고 감사 이벤트를 반환한다.
// 실패하면 Claim은 전혀 변경되지 않는다.
func (c *Claim) Transition(to Status, by Actor, note string) (Event, error) {
	if by.Kind == "" {
		// 누가 승인했는지 모르는 감사 로그는 감사 로그가 아니다.
		return Event{}, fmt.Errorf("동작 주체(Actor)가 필요합니다")
	}
	if _, err := ParseStatus(string(to)); err != nil {
		return Event{}, err
	}
	if err := c.transitionError(to); err != nil {
		return Event{}, err
	}

	from := c.Status
	now := time.Now()
	c.Status = to
	if to.IsTerminal() {
		c.ResolvedAt = now
	}
	if to == StatusPaid {
		c.PaidAt = now
	}

	return Event{
		ClaimID: c.ID, Actor: by, Action: ActionTransition,
		From: from, To: to, Note: note, At: now,
	}, nil
}
