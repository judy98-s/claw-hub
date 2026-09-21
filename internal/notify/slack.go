package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// Slack은 Incoming Webhook으로 알림을 보낸다.
type Slack struct {
	webhookURL string
	baseURL    string
	client     *http.Client
}

func NewSlack(webhookURL, baseURL string) *Slack {
	return &Slack{
		webhookURL: webhookURL,
		baseURL:    strings.TrimRight(baseURL, "/"),
		// 알림 때문에 요청이 오래 붙들리면 안 된다.
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackMessage struct {
	Text   string       `json:"text"` // 알림 미리보기와 접근성 대체 텍스트
	Blocks []slackBlock `json:"blocks,omitempty"`
}

func section(md string) slackBlock {
	return slackBlock{Type: "section", Text: &slackText{Type: "mrkdwn", Text: md}}
}

func (s *Slack) ClaimCreated(ctx context.Context, n ClaimNotice) error {
	headline := fmt.Sprintf("*%s* · %s · *%s원*", n.MachineLabel, n.IssueLabel, comma(n.AmountKRW))

	var body strings.Builder
	// 보류 건은 사유가 먼저다. 사장님이 제목만 보고 승인을 누르면 안 된다.
	if n.Status == domain.StatusOnHold || n.Status == domain.StatusNeedsReview {
		body.WriteString(fmt.Sprintf(":warning: *%s*\n", n.Status.Label()))
	}
	body.WriteString(headline)
	for _, r := range n.RiskReasons {
		body.WriteString("\n• " + r.Message)
	}
	if n.PhotoCount > 0 {
		body.WriteString(fmt.Sprintf("\n사진 %d장 첨부", n.PhotoCount))
	}

	link := n.URL
	if link == "" {
		link = fmt.Sprintf("%s/admin/claims/%s", s.baseURL, n.ClaimID)
	}

	msg := slackMessage{
		Text: fmt.Sprintf("[%s] %s %s %s원", n.Status.Label(), n.MachineLabel, n.IssueLabel, comma(n.AmountKRW)),
		Blocks: []slackBlock{
			section(body.String()),
			section(fmt.Sprintf("<%s|열어서 처리하기>", link)),
		},
	}
	return s.post(ctx, msg)
}

func (s *Slack) MachineAlert(ctx context.Context, n MachineNotice) error {
	parts := make([]string, 0, len(n.ByIssue))
	for _, it := range domain.AllIssueTypes() {
		if c := n.ByIssue[it]; c > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", it.Label(), c))
		}
	}
	text := fmt.Sprintf(":wrench: *%s* · 24시간 내 %d건 접수 (%s)\n점검이 필요해 보입니다.",
		n.MachineLabel, n.Count, strings.Join(parts, ", "))

	return s.post(ctx, slackMessage{
		Text:   fmt.Sprintf("%s 24시간 내 %d건 — 점검 필요", n.MachineLabel, n.Count),
		Blocks: []slackBlock{section(text)},
	})
}

func (s *Slack) DailyDigest(ctx context.Context, n DigestNotice) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%s 요약*\n접수 %d건 · 환불 %d건 · %s원",
		n.Day, n.ClaimCount, n.PaidCount, comma(n.PaidTotalKRW)))
	if len(n.TopMachines) > 0 {
		b.WriteString("\n\n*접수가 많은 기계*")
		for _, m := range n.TopMachines {
			b.WriteString(fmt.Sprintf("\n• %s — %d건", m.Label, m.Count))
		}
	}
	return s.post(ctx, slackMessage{
		Text:   fmt.Sprintf("%s 요약: 접수 %d건, 환불 %d건", n.Day, n.ClaimCount, n.PaidCount),
		Blocks: []slackBlock{section(b.String())},
	})
}

func (s *Slack) post(ctx context.Context, msg slackMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("알림 직렬화: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("알림 요청 생성: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("알림 전송: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("알림 전송 실패 (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
