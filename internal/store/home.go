package store

import (
	"context"
	"fmt"
	"time"
)

// HomeSummary는 홈 화면이 "지금 뭘 해야 하나"를 그리는 데 필요한 전부다.
//
// 화면마다 따로 세면 같은 숫자가 화면마다 다르게 나온다. 한 번의 조회로
// 한 시점의 사실을 만든다.
type HomeSummary struct {
	Pending     int // 접수됨 — 아직 아무도 안 봄
	NeedsReview int // 확인 필요 — 고액이거나 리스크 플래그
	OnHold      int // 보류
	Approved    int // 승인했는데 아직 안 보냄

	// ApprovedAmountKRW는 승인만 하고 아직 안 나간 돈이다.
	// 손님에게 약속해놓고 안 준 금액이라 홈에서 가장 위에 온다.
	ApprovedAmountKRW int
	// OldestApprovedAt은 그중 가장 오래된 건이 승인된 시각이다.
	// "3건"보다 "이틀째"가 사람을 움직인다.
	OldestApprovedAt time.Time

	// StaleOnHold는 접수된 지 24시간이 넘도록 보류에 머문 건이다.
	// "나중에 보기"가 영원이 되는 걸 막는 유일한 장치다.
	//
	// 보류로 바꾼 시각이 아니라 접수 시각을 기준으로 삼는다. 손님은
	// 사장님이 보류를 누른 순간이 아니라 접수한 순간부터 기다린다.
	StaleOnHold int

	TodayClaims  int
	TodayPaid    int
	TodayPaidKRW int
}

// HomeSummaryFor는 한 매장의 홈 요약을 한 번의 조회로 만든다.
func (s *Store) HomeSummaryFor(ctx context.Context, storeID string, now time.Time) (HomeSummary, error) {
	var h HomeSummary
	var oldest *time.Time

	// 오늘의 경계는 매장이 쓰는 시계 기준이다. 서버가 UTC로 돌면 한국의
	// 오전 9시 이전 접수가 "어제"로 집계된다.
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status='pending'),
		  COUNT(*) FILTER (WHERE status='needs_review'),
		  COUNT(*) FILTER (WHERE status='on_hold'),
		  COUNT(*) FILTER (WHERE status='approved'),
		  COALESCE(SUM(amount_krw) FILTER (WHERE status='approved'), 0),
		  COUNT(*) FILTER (WHERE status='on_hold' AND created_at < $2),
		  COUNT(*) FILTER (WHERE created_at >= $3),
		  COUNT(*) FILTER (WHERE status='paid' AND paid_at >= $3),
		  COALESCE(SUM(COALESCE(paid_amount_krw, amount_krw))
		           FILTER (WHERE status='paid' AND paid_at >= $3), 0)
		  FROM claims WHERE store_id=$1`,
		storeID, now.Add(-24*time.Hour), dayStart,
	).Scan(&h.Pending, &h.NeedsReview, &h.OnHold, &h.Approved,
		&h.ApprovedAmountKRW, &h.StaleOnHold,
		&h.TodayClaims, &h.TodayPaid, &h.TodayPaidKRW)
	if err != nil {
		return HomeSummary{}, fmt.Errorf("홈 요약 집계: %w", err)
	}

	// 승인 시각은 claims에 없다. approved는 종료 상태가 아니라서
	// resolved_at이 채워지지 않는다 — 감사 로그가 유일한 출처다.
	if h.Approved > 0 {
		if err := s.pool.QueryRow(ctx, `
			SELECT MIN(e.created_at)
			  FROM claim_events e JOIN claims c ON c.id = e.claim_id
			 WHERE c.store_id=$1 AND c.status='approved' AND e.to_status='approved'`,
			storeID,
		).Scan(&oldest); err != nil {
			return HomeSummary{}, fmt.Errorf("최초 승인 시각 조회: %w", err)
		}
		h.OldestApprovedAt = deref(oldest)
	}
	return h, nil
}

// MachineAlert는 오늘 신고가 몰린 기계 한 대다.
type MachineAlert struct {
	MachineID string `json:"machineId"`
	Label     string `json:"label"`
	Count     int    `json:"count"`
}

// MachineAlertsFor는 24시간 내 신고가 임계치 이상인 기계를 매장 범위로 찾는다.
//
// MachinesOverThreshold는 워커가 전 매장을 도는 용도라 매장 필터가 없다.
// 홈 화면에서 그걸 쓰면 남의 매장 기계가 섞인다.
func (s *Store) MachineAlertsFor(ctx context.Context, storeID string, threshold int, now time.Time) ([]MachineAlert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.label, COUNT(*)
		  FROM claims c JOIN machines m ON m.id = c.machine_id
		 WHERE c.store_id=$1 AND c.created_at >= $2
		 GROUP BY m.id, m.label
		HAVING COUNT(*) >= $3
		 ORDER BY COUNT(*) DESC`, storeID, now.Add(-24*time.Hour), threshold)
	if err != nil {
		return nil, fmt.Errorf("기계 알림 조회: %w", err)
	}
	defer rows.Close()

	out := []MachineAlert{}
	for rows.Next() {
		var a MachineAlert
		if err := rows.Scan(&a.MachineID, &a.Label, &a.Count); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
