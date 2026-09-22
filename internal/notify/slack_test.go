package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// capture는 Slack webhook을 흉내내고 받은 본문을 돌려준다.
func capture(t *testing.T, status int, send func(*Slack) error) string {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.WriteHeader(status)
	}))
	defer srv.Close()

	err := send(NewSlack(srv.URL, "https://claw.example.com"))
	if status < 300 && err != nil {
		t.Fatalf("전송 실패: %v", err)
	}
	if status >= 400 && err == nil {
		t.Fatalf("HTTP %d 인데 에러가 없다", status)
	}
	return got
}

func sampleClaim() ClaimNotice {
	return ClaimNotice{
		ClaimID: "claim-123", MachineLabel: "3번 기계", IssueLabel: "현금 먹음",
		AmountKRW: 2000, Status: domain.StatusPending, PhotoCount: 1,
	}
}

func TestClaimCreated_필요한_정보가_들어있다(t *testing.T) {
	body := capture(t, 200, func(s *Slack) error {
		return s.ClaimCreated(context.Background(), sampleClaim())
	})
	for _, want := range []string{"3번 기계", "현금 먹음", "2,000", "claim-123", "https://claw.example.com"} {
		if !strings.Contains(body, want) {
			t.Errorf("알림에 %q 가 없다: %s", want, body)
		}
	}
}

func TestClaimCreated_개인정보는_절대_나가지_않는다(t *testing.T) {
	// Slack 워크스페이스는 DB보다 접근 통제가 느슨하고, 알림은 검색·저장·
	// 전달된다. 사장님은 링크를 눌러 대시보드에서 보면 되고, 그 열람은
	// 감사 로그에 남는다.
	n := sampleClaim()
	body := capture(t, 200, func(s *Slack) error {
		return s.ClaimCreated(context.Background(), n)
	})
	for _, leak := range []string{"01012345678", "010-1234-5678", "3333011234567", "김민수"} {
		if strings.Contains(body, leak) {
			t.Errorf("알림에 개인정보가 들어갔다: %q", leak)
		}
	}
	// 구조체 자체에 개인정보 필드가 없어야 실수로 채울 수도 없다.
	raw, _ := json.Marshal(n)
	for _, field := range []string{"phone", "Phone", "account", "Account", "holder", "Holder"} {
		if strings.Contains(string(raw), field) {
			t.Errorf("ClaimNotice에 %q 필드가 있다 — 언젠가 채워진다", field)
		}
	}
}

func TestClaimCreated_보류건은_사유가_먼저_온다(t *testing.T) {
	n := sampleClaim()
	n.Status = domain.StatusOnHold
	n.RiskReasons = []domain.RiskReason{
		{Code: domain.ReasonRepeatHold, Message: "이 번호로 30일간 6번째 신고입니다 (보류 기준 5번)"},
	}
	body := capture(t, 200, func(s *Slack) error {
		return s.ClaimCreated(context.Background(), n)
	})

	if !strings.Contains(body, "6번째 신고") {
		t.Errorf("리스크 사유가 알림에 없다: %s", body)
	}
	// 사장님이 제목만 보고 승인하면 안 된다. 보류 표시가 기계 이름보다 앞서야 한다.
	holdIdx := strings.Index(body, "보류")
	machineIdx := strings.Index(body, "3번 기계")
	if holdIdx < 0 || holdIdx > machineIdx {
		t.Errorf("보류 표시가 앞에 오지 않는다 (보류=%d, 기계=%d)", holdIdx, machineIdx)
	}
}

func TestMachineAlert(t *testing.T) {
	body := capture(t, 200, func(s *Slack) error {
		return s.MachineAlert(context.Background(), MachineNotice{
			MachineLabel: "3번 기계", Count: 3,
			ByIssue: map[domain.IssueType]int{domain.IssueCashEaten: 2, domain.IssueClawBroken: 1},
		})
	})
	for _, want := range []string{"3번 기계", "3건", "돈만 빠짐 2", "집게 불량 1", "점검"} {
		if !strings.Contains(body, want) {
			t.Errorf("알림에 %q 가 없다: %s", want, body)
		}
	}
}

func TestDailyDigest(t *testing.T) {
	body := capture(t, 200, func(s *Slack) error {
		return s.DailyDigest(context.Background(), DigestNotice{
			Day: "2026-09-20", ClaimCount: 12, PaidCount: 9, PaidTotalKRW: 23000,
			TopMachines: []MachineLine{{Label: "3번 기계", Count: 5}},
		})
	})
	for _, want := range []string{"2026-09-20", "12건", "9건", "23,000", "3번 기계"} {
		if !strings.Contains(body, want) {
			t.Errorf("요약에 %q 가 없다: %s", want, body)
		}
	}
}

func TestPost_실패하면_에러를_반환한다(t *testing.T) {
	// 호출자는 이 에러를 로깅만 한다. 알림 실패가 접수 실패가 되면 안 된다.
	capture(t, 500, func(s *Slack) error {
		return s.ClaimCreated(context.Background(), sampleClaim())
	})
}

func TestSlackMessage_Text가_항상_채워진다(t *testing.T) {
	// blocks만 있고 text가 비면 모바일 푸시에 내용이 안 뜬다.
	sends := []func(*Slack) error{
		func(s *Slack) error { return s.ClaimCreated(context.Background(), sampleClaim()) },
		func(s *Slack) error {
			return s.MachineAlert(context.Background(), MachineNotice{MachineLabel: "3번", Count: 3, ByIssue: map[domain.IssueType]int{}})
		},
		func(s *Slack) error { return s.DailyDigest(context.Background(), DigestNotice{Day: "2026-09-20"}) },
	}
	for i, send := range sends {
		body := capture(t, 200, send)
		var msg slackMessage
		if err := json.Unmarshal([]byte(body), &msg); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		if strings.TrimSpace(msg.Text) == "" {
			t.Errorf("%d: text가 비었다 — 모바일 푸시에 내용이 안 뜬다", i)
		}
	}
}

func TestNoop은_항상_성공한다(t *testing.T) {
	n := NewNoop()
	ctx := context.Background()
	if err := n.ClaimCreated(ctx, ClaimNotice{}); err != nil {
		t.Error(err)
	}
	if err := n.MachineAlert(ctx, MachineNotice{}); err != nil {
		t.Error(err)
	}
	if err := n.DailyDigest(ctx, DigestNotice{}); err != nil {
		t.Error(err)
	}
}
