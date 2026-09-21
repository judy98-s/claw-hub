package domain

import (
	"fmt"
	"time"
)

// 리스크 사유 코드. DB와 API에 그대로 나가므로 값을 바꾸면 과거 기록과 어긋난다.
const (
	ReasonManualBlock    = "manual_block"
	ReasonManualWatch    = "manual_watch"
	ReasonRepeatWatch    = "repeat_watch"
	ReasonRepeatHold     = "repeat_hold"
	ReasonAccountSharing = "account_sharing"
	ReasonPayoutCeiling  = "payout_ceiling"
	ReasonHighAmount     = "high_amount"
)

// RiskReason은 발동한 규칙 하나다.
//
// Message는 사장님 화면에 그대로 보이는 한국어 문장이다. 점수만 주면
// "위험도 72점"이 되고, 사장님은 그걸로 승인할지 전화를 걸지 판단할 수 없다.
type RiskReason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RiskInput은 평가에 필요한 사실들이다. 저장 계층이 채워서 넘긴다.
type RiskInput struct {
	AmountKRW             int
	PhoneClaims30d        int       // 이 번호의 30일 내 신고 건수 (이번 건 포함)
	PhonePaidTotal30d     int       // 이 번호에 30일간 실제 지급된 금액
	AccountDistinctPhones int       // 이 계좌에 연결된 서로 다른 전화번호 수
	ManualStatus          string    // ContactNormal | ContactWatch | ContactBlocked
	LastSameMachineAt     time.Time // 같은 번호 + 같은 기계의 직전 신고 시각
	Now                   time.Time
}

// RiskResult는 평가 결과다.
type RiskResult struct {
	Score           int
	Reasons         []RiskReason
	Status          Status // pending | needs_review | on_hold
	RejectDuplicate bool   // true면 접수 자체를 거부한다
}

// rule은 규칙 하나를 평가한다. 걸리지 않으면 ok=false.
type rule func(RiskInput, Policy) (reason RiskReason, status Status, weight int, ok bool)

// rules는 평가할 규칙 전부다. 새 규칙 추가가 기존 규칙을 건드리지 않게 분리했다.
var rules = []rule{ruleManual, ruleRepeat, ruleAccountSharing, rulePayoutCeiling, ruleHighAmount}

// Evaluate는 신고 건의 리스크를 평가해 초기 상태와 사유를 정한다.
//
// 순수 함수다. 같은 입력이면 항상 같은 결과가 나오고, DB나 시계에 접근하지
// 않으므로 경계값을 전수로 테스트할 수 있다.
func Evaluate(in RiskInput, p Policy) RiskResult {
	res := RiskResult{Status: StatusPending}

	if isRapidDuplicate(in, p) {
		res.RejectDuplicate = true
	}

	score := 0
	for _, r := range rules {
		reason, status, weight, ok := r(in, p)
		if !ok {
			continue
		}
		res.Reasons = append(res.Reasons, reason)
		res.Status = strongerStatus(res.Status, status)
		score += weight
	}

	if score > 100 {
		score = 100
	}
	res.Score = score
	return res
}

// statusRank는 상태의 강도다. 여러 규칙이 걸리면 가장 강한 상태가 이긴다.
var statusRank = map[Status]int{StatusPending: 0, StatusNeedsReview: 1, StatusOnHold: 2}

func strongerStatus(a, b Status) Status {
	if statusRank[b] > statusRank[a] {
		return b
	}
	return a
}

// isRapidDuplicate는 제출 버튼 두 번 누름이나 새로고침 재제출을 잡는다.
func isRapidDuplicate(in RiskInput, p Policy) bool {
	if in.LastSameMachineAt.IsZero() {
		return false
	}
	return in.Now.Sub(in.LastSameMachineAt) < p.RapidDuplicateWindow
}

func ruleManual(in RiskInput, _ Policy) (RiskReason, Status, int, bool) {
	switch in.ManualStatus {
	case ContactBlocked:
		return RiskReason{ReasonManualBlock, "사장님이 차단한 연락처입니다 (수동 지정 1건)"}, StatusOnHold, 60, true
	case ContactWatch:
		return RiskReason{ReasonManualWatch, "사장님이 관찰 대상으로 지정한 연락처입니다 (수동 지정 1건)"}, StatusNeedsReview, 20, true
	}
	return RiskReason{}, "", 0, false
}

func ruleRepeat(in RiskInput, p Policy) (RiskReason, Status, int, bool) {
	if in.PhoneClaims30d >= p.RepeatHoldCount {
		msg := fmt.Sprintf("이 번호로 30일간 %d번째 신고입니다 (보류 기준 %d번)", in.PhoneClaims30d, p.RepeatHoldCount)
		return RiskReason{ReasonRepeatHold, msg}, StatusOnHold, 50, true
	}
	if in.PhoneClaims30d >= p.RepeatWatchCount {
		msg := fmt.Sprintf("이 번호로 30일간 %d번째 신고입니다 (검토 기준 %d번)", in.PhoneClaims30d, p.RepeatWatchCount)
		return RiskReason{ReasonRepeatWatch, msg}, StatusNeedsReview, 25, true
	}
	return RiskReason{}, "", 0, false
}

func ruleAccountSharing(in RiskInput, p Policy) (RiskReason, Status, int, bool) {
	if in.AccountDistinctPhones < p.AccountSharingPhones {
		return RiskReason{}, "", 0, false
	}
	msg := fmt.Sprintf("이 계좌가 서로 다른 전화번호 %d개와 연결되어 있습니다", in.AccountDistinctPhones)
	return RiskReason{ReasonAccountSharing, msg}, StatusNeedsReview, 30, true
}

func rulePayoutCeiling(in RiskInput, p Policy) (RiskReason, Status, int, bool) {
	if in.PhonePaidTotal30d <= p.PayoutCeilingKRW {
		return RiskReason{}, "", 0, false
	}
	msg := fmt.Sprintf("이 번호에 30일간 %s원이 지급됐습니다 (한도 %s원)",
		comma(in.PhonePaidTotal30d), comma(p.PayoutCeilingKRW))
	return RiskReason{ReasonPayoutCeiling, msg}, StatusNeedsReview, 20, true
}

func ruleHighAmount(in RiskInput, p Policy) (RiskReason, Status, int, bool) {
	if in.AmountKRW < p.ReviewThresholdKRW {
		return RiskReason{}, "", 0, false
	}
	msg := fmt.Sprintf("요청 금액이 %s원입니다 (검토 기준 %s원)", comma(in.AmountKRW), comma(p.ReviewThresholdKRW))
	return RiskReason{ReasonHighAmount, msg}, StatusNeedsReview, 15, true
}

// comma는 금액에 천 단위 구분기호를 넣는다. 50000보다 50,000이 읽힌다.
func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := ""
	if n < 0 {
		neg, s = "-", s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return neg + string(out)
}
