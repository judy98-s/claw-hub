package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/judy98-s/claw-hub/internal/store"
)

// homeAction은 홈 화면의 한 줄이다. "지금 손 대야 하는 것" 하나.
//
// 서버가 순서까지 정한다. 무엇이 급한지는 도메인 지식이고, 화면마다
// 다시 정렬하면 웹과 앱이 다른 우선순위를 말하게 된다.
type homeAction struct {
	Kind string `json:"kind"`
	// Severity는 stop | warn | neutral. 화면의 색과 위치를 정한다.
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Count    int    `json:"count"`
	Href     string `json:"href"`
}

type homeResponse struct {
	StoreName string       `json:"storeName"`
	Actions   []homeAction `json:"actions"`

	TodayClaims  int `json:"todayClaims"`
	TodayPaid    int `json:"todayPaid"`
	TodayPaidKrw int `json:"todayPaidKrw"`
}

// machineAlertThreshold는 홈에 "점검해보세요"를 띄우는 기준이다.
// 워커의 Slack 알림과 같은 값을 쓴다 — 둘이 다르면 알림은 왔는데
// 화면에는 없는 상황이 생긴다.
const machineAlertThreshold = 3

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	// 매장 시계로 넘긴다. UTC 자정은 한국 시간 오전 9시라, 그냥 넘기면
	// 새벽에 들어온 접수가 "오늘"에서 빠진다.
	now := s.nowKST()

	sum, err := s.store.HomeSummaryFor(r.Context(), u.StoreID, now)
	if err != nil {
		internalError(w, r, err, "홈 요약 조회 실패")
		return
	}
	alerts, err := s.store.MachineAlertsFor(r.Context(), u.StoreID, machineAlertThreshold, now)
	if err != nil {
		internalError(w, r, err, "기계 알림 조회 실패")
		return
	}

	res := homeResponse{
		TodayClaims:  sum.TodayClaims,
		TodayPaid:    sum.TodayPaid,
		TodayPaidKrw: sum.TodayPaidKRW,
		Actions:      buildHomeActions(sum, alerts, now),
	}
	if st, err := s.store.StoreByID(r.Context(), u.StoreID); err == nil {
		res.StoreName = st.Name
		// 송금 앱을 안 골랐으면 승인해도 계좌 복사밖에 못 한다.
		// 처음 한 번 알려주지 않으면 사장님은 그게 설정인 줄 모른다.
		if st.PayoutProvider == "" || st.PayoutProvider == "none" {
			res.Actions = append(res.Actions, homeAction{
				Kind: "setup_payout", Severity: "neutral",
				Title:  "송금 앱을 정해주세요",
				Detail: "토스나 카카오뱅크를 고르면 환불 버튼이 앱을 바로 엽니다.",
				Href:   "/admin/settings",
			})
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// buildHomeActions는 요약을 "할 일 목록"으로 바꾼다.
//
// 순서가 이 함수의 전부다. 가장 위는 승인해놓고 안 보낸 건이다 —
// 손님에게 돈을 약속하고 주지 않은 상태이고, 그게 이 시스템에서
// 가장 나쁜 상태다. 새 접수보다 먼저다.
func buildHomeActions(sum store.HomeSummary, alerts []store.MachineAlert, now time.Time) []homeAction {
	out := []homeAction{}

	if sum.Approved > 0 {
		out = append(out, homeAction{
			Kind: "payout_pending", Severity: "stop",
			Title:  fmt.Sprintf("환불 %d건이 아직 안 나갔어요", sum.Approved),
			Detail: payoutPendingDetail(sum, now),
			Count:  sum.Approved,
			Href:   "/admin/claims?status=approved",
		})
	}
	if sum.NeedsReview > 0 {
		out = append(out, homeAction{
			Kind: "needs_review", Severity: "warn",
			Title:  fmt.Sprintf("확인이 필요한 %d건", sum.NeedsReview),
			Detail: "고액이거나 반복 신고로 걸러진 건입니다.",
			Count:  sum.NeedsReview,
			Href:   "/admin/claims?status=needs_review",
		})
	}
	if sum.Pending > 0 {
		out = append(out, homeAction{
			Kind: "pending", Severity: "warn",
			Title:  fmt.Sprintf("새 신고 %d건", sum.Pending),
			Detail: "아직 아무도 열어보지 않았습니다.",
			Count:  sum.Pending,
			Href:   "/admin/claims?status=pending",
		})
	}
	if sum.StaleOnHold > 0 {
		out = append(out, homeAction{
			Kind: "stale_on_hold", Severity: "warn",
			Title:  fmt.Sprintf("하루 넘게 보류 중인 %d건", sum.StaleOnHold),
			Detail: "손님은 접수한 순간부터 기다리고 있습니다.",
			Count:  sum.StaleOnHold,
			Href:   "/admin/claims?status=on_hold",
		})
	}
	for _, a := range alerts {
		out = append(out, homeAction{
			Kind: "machine_alert", Severity: "warn",
			Title:  fmt.Sprintf("%s에 오늘 %d건", a.Label, a.Count),
			Detail: "기계를 한 번 봐주세요. 계속 먹으면 전원을 빼는 게 쌉니다.",
			Count:  a.Count,
			Href:   "/admin/claims?machineId=" + a.MachineID,
		})
	}
	return out
}

// payoutPendingDetail은 "얼마를, 얼마나 오래" 를 한 문장으로 만든다.
// 건수만 보여주면 3건이 3천원인지 30만원인지 모른다.
func payoutPendingDetail(sum store.HomeSummary, now time.Time) string {
	amount := fmt.Sprintf("합계 %s원", comma(sum.ApprovedAmountKRW))
	if sum.OldestApprovedAt.IsZero() {
		return amount
	}
	days := int(now.Sub(sum.OldestApprovedAt).Hours() / 24)
	switch {
	case days >= 1:
		return fmt.Sprintf("%s · 가장 오래된 건은 %d일째", amount, days)
	default:
		return amount
	}
}
