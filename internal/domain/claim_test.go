package domain

import (
	"testing"
	"time"
)

var owner = Actor{Kind: ActorOwner, ID: "owner-1"}

func claimWith(s Status) *Claim {
	return &Claim{ID: "c1", StoreID: "s1", MachineID: "m1", AmountKRW: 2000, Status: s}
}

// 허용되는 전이 전수. 이 표에 없는 전이는 전부 거부되어야 한다.
var allowed = []struct{ from, to Status }{
	{StatusPending, StatusApproved},
	{StatusPending, StatusRejected},
	{StatusPending, StatusNeedsReview},
	{StatusPending, StatusOnHold},
	{StatusNeedsReview, StatusApproved},
	{StatusNeedsReview, StatusRejected},
	{StatusNeedsReview, StatusOnHold},
	{StatusOnHold, StatusNeedsReview},
	{StatusOnHold, StatusApproved},
	{StatusOnHold, StatusRejected},
	{StatusApproved, StatusPaid},
	{StatusApproved, StatusRejected},
}

func isAllowed(from, to Status) bool {
	for _, a := range allowed {
		if a.from == from && a.to == to {
			return true
		}
	}
	return false
}

func TestTransition_전수표(t *testing.T) {
	all := AllStatuses()
	for _, from := range all {
		for _, to := range all {
			c := claimWith(from)
			_, err := c.Transition(to, owner, "")
			want := isAllowed(from, to)
			if want && err != nil {
				t.Errorf("%s→%s 는 허용되어야 하는데 거부됨: %v", from, to, err)
			}
			if !want && err == nil {
				t.Errorf("%s→%s 는 거부되어야 하는데 허용됨", from, to)
			}
		}
	}
}

func TestTransition_종료상태에서는_어디로도_못_간다(t *testing.T) {
	// paid는 이미 돈이 나간 상태다. 되돌리면 이중 환불이 난다.
	for _, terminal := range []Status{StatusPaid, StatusRejected} {
		for _, to := range AllStatuses() {
			c := claimWith(terminal)
			if _, err := c.Transition(to, owner, ""); err == nil {
				t.Errorf("종료 상태 %s 에서 %s 로 전이가 허용됐다", terminal, to)
			}
		}
	}
}

func TestTransition_자기자신으로의_전이는_거부(t *testing.T) {
	// 버튼 두 번 누름이 감사 로그에 중복 이벤트를 남기면 안 된다.
	for _, s := range AllStatuses() {
		c := claimWith(s)
		if _, err := c.Transition(s, owner, ""); err == nil {
			t.Errorf("%s → %s (자기 자신) 전이가 허용됐다", s, s)
		}
	}
}

func TestTransition_실패시_Claim은_변경되지_않는다(t *testing.T) {
	c := claimWith(StatusPaid)
	before := *c
	if _, err := c.Transition(StatusApproved, owner, ""); err == nil {
		t.Fatal("금지 전이가 통과됐다")
	}
	if *c != before {
		t.Error("전이 실패인데 Claim이 변경됐다")
	}
}

func TestTransition_성공시_Event를_남긴다(t *testing.T) {
	c := claimWith(StatusPending)
	ev, err := c.Transition(StatusApproved, owner, "사진 확인함")
	if err != nil {
		t.Fatal(err)
	}
	if ev.ClaimID != "c1" {
		t.Errorf("Event.ClaimID = %q", ev.ClaimID)
	}
	if ev.From != StatusPending || ev.To != StatusApproved {
		t.Errorf("Event from/to = %s/%s", ev.From, ev.To)
	}
	if ev.Actor.Kind != owner.Kind || ev.Actor.ID != owner.ID {
		t.Errorf("Event.Actor = %+v", ev.Actor)
	}
	if ev.Note != "사진 확인함" {
		t.Errorf("Event.Note = %q", ev.Note)
	}
	if ev.Action != ActionTransition {
		t.Errorf("Event.Action = %q", ev.Action)
	}
	if ev.At.IsZero() {
		t.Error("Event.At 이 비어 있다")
	}
	if c.Status != StatusApproved {
		t.Errorf("Claim.Status = %s, want approved", c.Status)
	}
}

func TestTransition_시각_기록(t *testing.T) {
	t.Run("approved→paid 는 PaidAt을 채운다", func(t *testing.T) {
		c := claimWith(StatusApproved)
		if _, err := c.Transition(StatusPaid, owner, ""); err != nil {
			t.Fatal(err)
		}
		if c.PaidAt.IsZero() {
			t.Error("PaidAt 이 비어 있다")
		}
		if c.ResolvedAt.IsZero() {
			t.Error("paid 는 종료 상태인데 ResolvedAt 이 비어 있다")
		}
	})

	t.Run("rejected 는 ResolvedAt만 채운다", func(t *testing.T) {
		c := claimWith(StatusPending)
		if _, err := c.Transition(StatusRejected, owner, "증빙 부족"); err != nil {
			t.Fatal(err)
		}
		if c.ResolvedAt.IsZero() {
			t.Error("ResolvedAt 이 비어 있다")
		}
		if !c.PaidAt.IsZero() {
			t.Error("거절인데 PaidAt 이 채워졌다")
		}
	})

	t.Run("중간 상태는 ResolvedAt을 채우지 않는다", func(t *testing.T) {
		c := claimWith(StatusPending)
		if _, err := c.Transition(StatusNeedsReview, owner, ""); err != nil {
			t.Fatal(err)
		}
		if !c.ResolvedAt.IsZero() {
			t.Error("needs_review 인데 ResolvedAt 이 채워졌다")
		}
	})
}

func TestCanTransition은_Transition과_일치한다(t *testing.T) {
	for _, from := range AllStatuses() {
		for _, to := range AllStatuses() {
			c := claimWith(from)
			can := c.CanTransition(to)
			_, err := claimWith(from).Transition(to, owner, "")
			if can != (err == nil) {
				t.Errorf("%s→%s: CanTransition=%v 인데 Transition err=%v", from, to, can, err)
			}
		}
	}
}

func TestCanTransition은_부작용이_없다(t *testing.T) {
	c := claimWith(StatusPending)
	before := *c
	c.CanTransition(StatusApproved)
	if *c != before {
		t.Error("CanTransition 이 Claim을 변경했다")
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range AllStatuses() {
		got, err := ParseStatus(string(s))
		if err != nil || got != s {
			t.Errorf("ParseStatus(%q) = %v, %v", s, got, err)
		}
	}
	for _, bad := range []string{"", "PENDING", "done", "완료"} {
		if _, err := ParseStatus(bad); err == nil {
			t.Errorf("ParseStatus(%q) 가 통과됐다", bad)
		}
	}
}

func TestParseIssueType(t *testing.T) {
	for _, it := range AllIssueTypes() {
		got, err := ParseIssueType(string(it))
		if err != nil || got != it {
			t.Errorf("ParseIssueType(%q) = %v, %v", it, got, err)
		}
	}
	for _, bad := range []string{"", "broken", "인형걸림"} {
		if _, err := ParseIssueType(bad); err == nil {
			t.Errorf("ParseIssueType(%q) 가 통과됐다", bad)
		}
	}
}

func TestIssueType_한국어_라벨(t *testing.T) {
	// 라벨은 Slack 알림과 대시보드에 그대로 나간다. 비어 있으면 안 된다.
	want := map[IssueType]string{
		IssueDollStuck:  "인형 걸림",
		IssueCashEaten:  "돈만 빠짐",
		IssueClawBroken: "집게 불량",
		IssueOther:      "기타",
	}
	for it, label := range want {
		if got := it.Label(); got != label {
			t.Errorf("%s.Label() = %q, want %q", it, got, label)
		}
	}
}

func TestStatus_한국어_라벨_전부_채워져있다(t *testing.T) {
	for _, s := range AllStatuses() {
		if s.Label() == "" {
			t.Errorf("Status %s 의 라벨이 비어 있다", s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := map[Status]bool{StatusPaid: true, StatusRejected: true}
	for _, s := range AllStatuses() {
		if got := s.IsTerminal(); got != terminal[s] {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, terminal[s])
		}
	}
}

func TestTransition_Actor가_비면_거부(t *testing.T) {
	// 누가 승인했는지 모르는 감사 로그는 감사 로그가 아니다.
	c := claimWith(StatusPending)
	if _, err := c.Transition(StatusApproved, Actor{}, ""); err == nil {
		t.Fatal("빈 Actor로 전이가 허용됐다")
	}
}

func TestNewEvent_생성이벤트(t *testing.T) {
	ev := NewEvent("c1", Actor{Kind: ActorCustomer}, ActionCreate, "", StatusPending, "")
	if ev.At.IsZero() || ev.Action != ActionCreate || ev.To != StatusPending {
		t.Errorf("NewEvent = %+v", ev)
	}
	if time.Since(ev.At) > time.Minute {
		t.Error("Event.At 이 현재 시각이 아니다")
	}
}
