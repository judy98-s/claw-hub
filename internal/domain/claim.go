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
//
// 코드는 **전역에서 한 뜻만** 갖는다. 업종마다 같은 코드를 다른 뜻으로
// 쓰면 (세탁 no_start="작동 안 함", 사진 no_start="촬영 안 됨") 집계와
// Slack 알림에서 어느 뜻인지 알 수 없다. 그래서 업종 접두사를 붙이는
// 대신 코드 자체를 겹치지 않게 지었다. 접두사를 붙이면 이미 저장된
// claw 데이터를 전부 마이그레이션해야 한다.
type IssueType string

const (
	// 공통
	IssueOther IssueType = "other" // 기타

	// 인형뽑기
	IssueDollStuck  IssueType = "doll_stuck"  // 인형 걸림
	IssueCashEaten  IssueType = "cash_eaten"  // 돈만 빠짐 (현금·카드 공통)
	IssueClawBroken IssueType = "claw_broken" // 집게 불량

	// 무인세탁
	IssueNoStart    IssueType = "no_start"    // 작동 안 함
	IssueStoppedMid IssueType = "stopped_mid" // 중간에 멈춤
	IssueNotDry     IssueType = "not_dry"     // 건조 덜 됨

	// 코인노래방
	IssueNoCredit  IssueType = "no_credit"  // 곡 수 안 올라감
	IssueMicBroken IssueType = "mic_broken" // 마이크 고장
	IssueCutOff    IssueType = "cut_off"    // 중간에 끊김

	// 무인사진관
	IssueNoPrint  IssueType = "no_print"  // 사진 안 나옴
	IssueBadPrint IssueType = "bad_print" // 인화 불량
	IssueNoShoot  IssueType = "no_shoot"  // 촬영 안 됨

	// 스터디카페
	IssueNoSeat       IssueType = "no_seat"       // 좌석 배정 안 됨
	IssueDoorLocked   IssueType = "door_locked"   // 출입문 안 열림
	IssueDoubleCharge IssueType = "double_charge" // 두 번 결제

	// 무인카페
	IssueNoDrink    IssueType = "no_drink"    // 음료 안 나옴
	IssueNoCup      IssueType = "no_cup"      // 컵 안 나옴
	IssueWrongDrink IssueType = "wrong_drink" // 다른 음료
)

// storeIssues는 업종별 증상 목록이다. 손님 화면에 보이는 순서 그대로다.
//
// docs/stores.json 이 디자인 원본이고, web/lib/stores.ts 와 이 표가 같은
// 값을 들고 있어야 한다. 세 군데가 어긋나면 손님이 고른 증상이 서버에서
// 400 으로 떨어진다 — 손님은 이유를 알 수 없고 그냥 떠난다.
// 어긋나지 않도록 docs/stores.json 과 대조하는 테스트를 둔다.
var storeIssues = map[StoreType][]IssueType{
	StoreClaw:    {IssueDollStuck, IssueCashEaten, IssueClawBroken, IssueOther},
	StoreLaundry: {IssueNoStart, IssueStoppedMid, IssueNotDry, IssueOther},
	StoreKaraoke: {IssueNoCredit, IssueMicBroken, IssueCutOff, IssueOther},
	StorePhoto:   {IssueNoPrint, IssueBadPrint, IssueNoShoot, IssueOther},
	StoreStudy:   {IssueNoSeat, IssueDoorLocked, IssueDoubleCharge, IssueOther},
	StoreCafe:    {IssueNoDrink, IssueNoCup, IssueWrongDrink, IssueOther},
}

// IssuesFor는 그 업종에서 손님이 고를 수 있는 증상이다.
// 모르는 업종이면 인형뽑기로 떨어뜨린다 — NormalizeStoreType 과 같은 규칙이다.
func IssuesFor(st StoreType) []IssueType {
	if v, ok := storeIssues[st]; ok {
		return v
	}
	return storeIssues[StoreClaw]
}

// ValidIssueFor는 그 업종에서 받을 수 있는 증상인지 본다.
//
// ParseIssueType 만으로는 부족하다. 그건 "서버가 아는 코드인가"만 보므로,
// 뽑기방 기계에 세탁기 증상을 보내는 요청이 통과한다. 그러면 집계가
// 조용히 더러워지고, 사장님 화면에 "7번 기계 · 건조 덜 됨"이 뜬다.
func ValidIssueFor(st StoreType, it IssueType) bool {
	for _, v := range IssuesFor(st) {
		if v == it {
			return true
		}
	}
	return false
}

// AllIssueTypes는 모든 업종의 증상을 합친 것이다. 중복은 없다.
//
// 저장된 코드를 라벨로 바꿀 때와, 업종을 모르는 자리(집계·알림)에서 쓴다.
func AllIssueTypes() []IssueType {
	out := make([]IssueType, 0, len(issueLabels))
	seen := map[IssueType]bool{}
	for _, st := range AllStoreTypes() {
		for _, it := range IssuesFor(st) {
			if !seen[it] {
				seen[it] = true
				out = append(out, it)
			}
		}
	}
	return out
}

var issueLabels = map[IssueType]string{
	IssueOther: "기타",

	IssueDollStuck: "인형 걸림",
	// "현금 먹음"이 아니라 "돈만 빠짐"이다. 결제 수단을 따로 묻게 된
	// 뒤로는, 카드 결제 건이 "7번 기계 · 현금 먹음 · 카드"로 표시된다.
	IssueCashEaten:  "돈만 빠짐",
	IssueClawBroken: "집게 불량",

	IssueNoStart:    "작동 안 함",
	IssueStoppedMid: "중간에 멈춤",
	IssueNotDry:     "건조 덜 됨",

	IssueNoCredit:  "곡 수 안 올라감",
	IssueMicBroken: "마이크 고장",
	IssueCutOff:    "중간에 끊김",

	IssueNoPrint:  "사진 안 나옴",
	IssueBadPrint: "인화 불량",
	IssueNoShoot:  "촬영 안 됨",

	IssueNoSeat:       "좌석 배정 안 됨",
	IssueDoorLocked:   "출입문 안 열림",
	IssueDoubleCharge: "두 번 결제",

	IssueNoDrink:    "음료 안 나옴",
	IssueNoCup:      "컵 안 나옴",
	IssueWrongDrink: "다른 음료",
}

// Label은 업종을 몰라도 쓸 수 있다. 코드가 전역에서 한 뜻이므로
// 업종을 넘겨받을 필요가 없다 — 집계·Slack 에서 이게 중요하다.
func (i IssueType) Label() string {
	if v, ok := issueLabels[i]; ok {
		return v
	}
	// 저장된 코드를 지운 뒤에도 과거 민원은 열려야 한다. 빈칸보다
	// 코드가 낫다 — 적어도 무슨 일이 있었는지 추적할 수 있다.
	return string(i)
}

// ParseIssueType은 서버가 아는 코드인지만 본다. 업종까지 맞는지는
// ValidIssueFor 가 본다 — 접수 경로에서는 둘 다 통과해야 한다.
func ParseIssueType(s string) (IssueType, error) {
	if _, ok := issueLabels[IssueType(s)]; ok {
		return IssueType(s), nil
	}
	return "", fmt.Errorf("알 수 없는 증상: %q", s)
}

// Actor는 어떤 동작을 일으킨 주체다.
type Actor struct {
	Kind string // ActorCustomer | ActorOwner | ActorSystem | ActorLink
	ID   string
	// Name은 화면에 보여줄 이름이다. 저장하지 않고 읽을 때 채운다.
	//
	// "승인됨"만으로는 직원이 여러 명일 때 누가 했는지 알 수 없다.
	// 돈이 오가는 기록에서 그건 빈칸이나 마찬가지다.
	Name string
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

// PaymentMethod는 손님이 무엇으로 결제했는지다.
//
// 처리 방법이 완전히 다르다. 현금은 계좌로 송금하고, 카드는 단말기에서
// 승인을 취소한다. 그래서 접수 단계에서 갈라야 하고, 카드 건에는 계좌번호를
// 받지 않는다 — 받을 이유가 없다.
type PaymentMethod string

const (
	PaymentCash    PaymentMethod = "cash"
	PaymentCard    PaymentMethod = "card"
	PaymentUnknown PaymentMethod = "unknown" // 이 구분이 생기기 전의 기존 건
)

func AllPaymentMethods() []PaymentMethod {
	return []PaymentMethod{PaymentCash, PaymentCard}
}

var paymentLabels = map[PaymentMethod]string{
	PaymentCash:    "현금",
	PaymentCard:    "카드",
	PaymentUnknown: "확인 안 됨",
}

func (p PaymentMethod) Label() string { return paymentLabels[p] }

// NeedsAccount는 환불에 계좌 정보가 필요한지 알려준다.
// 카드 결제는 단말기에서 취소하므로 계좌를 받지 않는다.
func (p PaymentMethod) NeedsAccount() bool { return p != PaymentCard }

func ParsePaymentMethod(s string) (PaymentMethod, error) {
	for _, v := range AllPaymentMethods() {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("결제 수단을 선택해주세요")
}

// PayoutMethod는 환불이 어떤 경로로 처리됐는지다.
const (
	PayoutDeeplink = "deeplink"  // 토스·카카오뱅크 앱으로 보냄
	PayoutManual   = "manual"    // 사장님이 다른 방법으로 보내고 수동 기록
	PayoutCardVoid = "card_void" // 카드 승인을 단말기에서 취소함
)

// AllPayoutMethods는 기록 가능한 처리 경로다.
func AllPayoutMethods() []string {
	return []string{PayoutDeeplink, PayoutManual, PayoutCardVoid}
}

// Claim은 신고 건 하나다.
type Claim struct {
	ID        string
	StoreID   string
	MachineID string

	IssueType     IssueType
	PaymentMethod PaymentMethod
	AmountKRW     int
	Description   string
	// CardLast4는 카드 결제 건에서 거래를 찾기 위한 단서다.
	CardLast4 string
	// PaidAtGuess는 손님이 기억하는 대략의 결제 시각이다.
	PaidAtGuess time.Time

	Status    Status
	RiskScore int

	CreatedAt    time.Time
	ResolvedAt   time.Time
	PaidAt       time.Time
	PayoutMethod string
	// PaidAmountKRW는 실제로 보낸 금액이다. 요청액과 다를 수 있다.
	// 0이면 아직 송금 전이다.
	PaidAmountKRW int
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
