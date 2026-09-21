package domain

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

// base는 아무 규칙도 걸리지 않는 정상 입력이다.
func base() RiskInput {
	return RiskInput{
		AmountKRW:             2000,
		PhoneClaims30d:        1,
		PhonePaidTotal30d:     2000,
		AccountDistinctPhones: 1,
		ManualStatus:          ContactNormal,
		Now:                   now,
	}
}

func eval(mut func(*RiskInput)) RiskResult {
	in := base()
	if mut != nil {
		mut(&in)
	}
	return Evaluate(in, DefaultPolicy())
}

func hasReason(r RiskResult, code string) bool {
	for _, x := range r.Reasons {
		if x.Code == code {
			return true
		}
	}
	return false
}

func TestEvaluate_정상건은_pending(t *testing.T) {
	r := eval(nil)
	if r.Status != StatusPending {
		t.Errorf("Status = %s, want pending", r.Status)
	}
	if len(r.Reasons) != 0 {
		t.Errorf("정상인데 사유가 있다: %+v", r.Reasons)
	}
	if r.RejectDuplicate {
		t.Error("정상인데 중복으로 판정됐다")
	}
}

func TestEvaluate_금액_경계값(t *testing.T) {
	tests := []struct {
		amount int
		want   Status
		reason bool
	}{
		{1, StatusPending, false},
		{1999, StatusPending, false},
		{9999, StatusPending, false},     // 임계 바로 아래
		{10000, StatusNeedsReview, true}, // 임계 정확히 — 여기서 사진이 필수가 된다
		{10001, StatusNeedsReview, true},
		{999999, StatusNeedsReview, true},
	}
	for _, tc := range tests {
		r := eval(func(in *RiskInput) { in.AmountKRW = tc.amount })
		if r.Status != tc.want {
			t.Errorf("금액 %d: Status = %s, want %s", tc.amount, r.Status, tc.want)
		}
		if hasReason(r, ReasonHighAmount) != tc.reason {
			t.Errorf("금액 %d: high_amount 사유 = %v, want %v", tc.amount, !tc.reason, tc.reason)
		}
	}
}

func TestValidateAmount_경계값(t *testing.T) {
	p := DefaultPolicy()
	bad := []int{0, -1, -10000, p.MaxAmountKRW + 1, 9999999}
	for _, a := range bad {
		if err := ValidateAmount(a, p); err == nil {
			t.Errorf("금액 %d 가 통과됐다", a)
		}
	}
	good := []int{1, 1000, 10000, p.MaxAmountKRW}
	for _, a := range good {
		if err := ValidateAmount(a, p); err != nil {
			t.Errorf("금액 %d 가 거부됐다: %v", a, err)
		}
	}
}

func TestEvaluate_반복신고_경계값(t *testing.T) {
	tests := []struct {
		count int
		want  Status
	}{
		{1, StatusPending},
		{2, StatusPending},     // watch 바로 아래
		{3, StatusNeedsReview}, // watch 임계
		{4, StatusNeedsReview},
		{5, StatusOnHold}, // hold 임계
		{9, StatusOnHold},
	}
	for _, tc := range tests {
		r := eval(func(in *RiskInput) { in.PhoneClaims30d = tc.count })
		if r.Status != tc.want {
			t.Errorf("30일 %d건: Status = %s, want %s", tc.count, r.Status, tc.want)
		}
	}
}

func TestEvaluate_계좌공유_경계값(t *testing.T) {
	tests := []struct {
		phones int
		want   Status
	}{
		{1, StatusPending},
		{2, StatusPending},
		{3, StatusNeedsReview}, // 한 계좌에 번호 3개 = 조직적 신고 의심
		{7, StatusNeedsReview},
	}
	for _, tc := range tests {
		r := eval(func(in *RiskInput) { in.AccountDistinctPhones = tc.phones })
		if r.Status != tc.want {
			t.Errorf("계좌에 번호 %d개: Status = %s, want %s", tc.phones, r.Status, tc.want)
		}
	}
}

func TestEvaluate_누적지급_경계값(t *testing.T) {
	p := DefaultPolicy()
	tests := []struct {
		paid int
		want Status
	}{
		{0, StatusPending},
		{p.PayoutCeilingKRW - 1, StatusPending},
		{p.PayoutCeilingKRW, StatusPending},         // 한도 정확히는 아직 통과
		{p.PayoutCeilingKRW + 1, StatusNeedsReview}, // 초과부터 검토
	}
	for _, tc := range tests {
		r := eval(func(in *RiskInput) { in.PhonePaidTotal30d = tc.paid })
		if r.Status != tc.want {
			t.Errorf("누적 %d원: Status = %s, want %s", tc.paid, r.Status, tc.want)
		}
	}
}

func TestEvaluate_수동차단은_다른게_다_정상이어도_보류(t *testing.T) {
	r := eval(func(in *RiskInput) { in.ManualStatus = ContactBlocked })
	if r.Status != StatusOnHold {
		t.Errorf("Status = %s, want on_hold", r.Status)
	}
	if !hasReason(r, ReasonManualBlock) {
		t.Error("manual_block 사유가 없다")
	}
}

func TestEvaluate_수동관찰은_검토대상(t *testing.T) {
	r := eval(func(in *RiskInput) { in.ManualStatus = ContactWatch })
	if r.Status != StatusNeedsReview {
		t.Errorf("Status = %s, want needs_review", r.Status)
	}
}

func TestEvaluate_빠른중복제출(t *testing.T) {
	p := DefaultPolicy()
	tests := []struct {
		name string
		last time.Time
		want bool
	}{
		{"1분 전", now.Add(-1 * time.Minute), true},
		{"9분 전", now.Add(-9 * time.Minute), true},
		{"창 경계 직전", now.Add(-p.RapidDuplicateWindow + time.Second), true},
		{"창 경계 정확히", now.Add(-p.RapidDuplicateWindow), false},
		{"11분 전", now.Add(-11 * time.Minute), false},
		{"이력 없음", time.Time{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := eval(func(in *RiskInput) { in.LastSameMachineAt = tc.last })
			if r.RejectDuplicate != tc.want {
				t.Errorf("RejectDuplicate = %v, want %v", r.RejectDuplicate, tc.want)
			}
		})
	}
}

func TestEvaluate_합성_우선순위는_가장_강한_상태가_이긴다(t *testing.T) {
	// on_hold > needs_review > pending
	r := eval(func(in *RiskInput) {
		in.AmountKRW = 50000         // needs_review
		in.PhoneClaims30d = 6        // on_hold
		in.AccountDistinctPhones = 4 // needs_review
	})
	if r.Status != StatusOnHold {
		t.Errorf("Status = %s, want on_hold (가장 강한 상태가 이겨야 한다)", r.Status)
	}
}

func TestEvaluate_발동한_규칙이_전부_담긴다(t *testing.T) {
	// 사장님은 "왜 보류됐는지"를 전부 봐야 판단할 수 있다.
	// 가장 강한 규칙 하나만 남기면 나머지 정황이 사라진다.
	r := eval(func(in *RiskInput) {
		in.AmountKRW = 50000
		in.PhoneClaims30d = 6
		in.AccountDistinctPhones = 4
		in.PhonePaidTotal30d = 100000
	})
	for _, want := range []string{ReasonHighAmount, ReasonRepeatHold, ReasonAccountSharing, ReasonPayoutCeiling} {
		if !hasReason(r, want) {
			t.Errorf("사유 %s 가 빠졌다. 있는 것: %+v", want, r.Reasons)
		}
	}
}

func TestEvaluate_사유메시지는_숫자가_채워진_한국어_문장이다(t *testing.T) {
	// "위험도 72점"으로는 사장님이 판단할 수 없다.
	// "이 번호로 30일간 4번째 신고입니다"를 봐야 승인할지 전화할지 정한다.
	r := eval(func(in *RiskInput) {
		in.AmountKRW = 50000
		in.PhoneClaims30d = 4
		in.AccountDistinctPhones = 3
		in.PhonePaidTotal30d = 100000
	})
	if len(r.Reasons) == 0 {
		t.Fatal("사유가 없다")
	}
	for _, x := range r.Reasons {
		if x.Code == "" {
			t.Error("Code 가 비었다")
		}
		if strings.TrimSpace(x.Message) == "" {
			t.Errorf("%s: Message 가 비었다", x.Code)
		}
		if !strings.ContainsAny(x.Message, "0123456789") {
			t.Errorf("%s: 메시지에 숫자가 없다 — 구체성이 없으면 판단에 못 쓴다: %q", x.Code, x.Message)
		}
	}
}

func TestEvaluate_점수는_사유가_늘수록_커진다(t *testing.T) {
	clean := eval(nil)
	dirty := eval(func(in *RiskInput) {
		in.AmountKRW = 50000
		in.PhoneClaims30d = 6
	})
	if clean.Score != 0 {
		t.Errorf("정상 건 Score = %d, want 0", clean.Score)
	}
	if dirty.Score <= clean.Score {
		t.Errorf("사유가 많은 건의 Score(%d)가 정상 건(%d)보다 크지 않다", dirty.Score, clean.Score)
	}
	if dirty.Score > 100 {
		t.Errorf("Score = %d, 100을 넘으면 안 된다", dirty.Score)
	}
}

func TestEvaluate_정책값_주입이_동작한다(t *testing.T) {
	p := DefaultPolicy()
	p.ReviewThresholdKRW = 30000
	in := base()
	in.AmountKRW = 20000
	if r := Evaluate(in, p); r.Status != StatusPending {
		t.Errorf("임계 3만원 정책에서 2만원이 %s 가 됐다", r.Status)
	}
	in.AmountKRW = 30000
	if r := Evaluate(in, p); r.Status != StatusNeedsReview {
		t.Errorf("임계 3만원 정책에서 3만원이 %s 가 됐다", r.Status)
	}
}

func TestRequiresPhoto(t *testing.T) {
	p := DefaultPolicy()
	tests := []struct {
		amount int
		want   bool
	}{
		{1000, false},
		{9999, false},
		{10000, true},
		{50000, true},
	}
	for _, tc := range tests {
		if got := RequiresPhoto(tc.amount, p); got != tc.want {
			t.Errorf("RequiresPhoto(%d) = %v, want %v", tc.amount, got, tc.want)
		}
	}
}

func TestParseContactStatus(t *testing.T) {
	for _, s := range []string{ContactNormal, ContactWatch, ContactBlocked} {
		if got, err := ParseContactStatus(s); err != nil || got != s {
			t.Errorf("ParseContactStatus(%q) = %q, %v", s, got, err)
		}
	}
	for _, bad := range []string{"", "banned", "차단"} {
		if _, err := ParseContactStatus(bad); err == nil {
			t.Errorf("ParseContactStatus(%q) 가 통과됐다", bad)
		}
	}
}
