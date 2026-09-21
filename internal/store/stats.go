package store

import (
	"context"
	"fmt"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// DailyStat은 하루치 집계다.
type DailyStat struct {
	Day          string `json:"day"` // YYYY-MM-DD
	ClaimCount   int    `json:"claimCount"`
	PaidCount    int    `json:"paidCount"`
	PaidTotalKRW int    `json:"paidTotalKrw"`
}

// DailyStats는 기간 내 일별 집계를 반환한다.
//
// 접수가 없는 날도 0으로 채운다. 빈 날을 건너뛰면 그래프에서 한산한 주와
// 바쁜 주가 똑같아 보인다.
func (s *Store) DailyStats(ctx context.Context, storeID string, from, to time.Time) ([]DailyStat, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(d::date, 'YYYY-MM-DD'),
		       COUNT(c.id),
		       COUNT(c.id) FILTER (WHERE c.status='paid'),
		       COALESCE(SUM(c.amount_krw) FILTER (WHERE c.status='paid'), 0)
		  FROM generate_series($2::date, $3::date, interval '1 day') d
		  LEFT JOIN claims c
		         ON c.store_id = $1 AND c.created_at::date = d::date
		 GROUP BY d ORDER BY d`, storeID, from, to)
	if err != nil {
		return nil, fmt.Errorf("일별 집계: %w", err)
	}
	defer rows.Close()

	out := []DailyStat{}
	for rows.Next() {
		var d DailyStat
		if err := rows.Scan(&d.Day, &d.ClaimCount, &d.PaidCount, &d.PaidTotalKRW); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MachineActivity는 한 기계의 최근 활동이다. 점검 알림 판단에 쓴다.
type MachineActivity struct {
	MachineID string
	Label     string
	StoreID   string
	Count     int
	ByIssue   map[domain.IssueType]int
}

// MachinesOverThreshold는 24시간 내 신고가 임계치 이상인 기계를 찾는다.
//
// "3번 기계에 오늘 3건"은 사장님이 손님보다 먼저 알아야 하는 정보다.
// 기계가 계속 돈을 먹으면 신고가 쌓이기 전에 전원을 빼는 게 싸게 먹힌다.
func (s *Store) MachinesOverThreshold(ctx context.Context, threshold int, now time.Time) ([]MachineActivity, error) {
	since := now.Add(-24 * time.Hour)
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.label, m.store_id, c.issue_type, COUNT(*)
		  FROM claims c JOIN machines m ON m.id = c.machine_id
		 WHERE c.created_at >= $1
		 GROUP BY m.id, m.label, m.store_id, c.issue_type`, since)
	if err != nil {
		return nil, fmt.Errorf("기계 활동 집계: %w", err)
	}
	defer rows.Close()

	byMachine := map[string]*MachineActivity{}
	for rows.Next() {
		var id, label, storeID, issue string
		var n int
		if err := rows.Scan(&id, &label, &storeID, &issue, &n); err != nil {
			return nil, err
		}
		a, ok := byMachine[id]
		if !ok {
			a = &MachineActivity{MachineID: id, Label: label, StoreID: storeID,
				ByIssue: map[domain.IssueType]int{}}
			byMachine[id] = a
		}
		a.Count += n
		a.ByIssue[domain.IssueType(issue)] += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []MachineActivity{}
	for _, a := range byMachine {
		if a.Count >= threshold {
			out = append(out, *a)
		}
	}
	return out, nil
}

// DigestRow는 일일 요약 한 매장분이다.
type DigestRow struct {
	StoreID      string
	ClaimCount   int
	PaidCount    int
	PaidTotalKRW int
	TopMachines  []MachineCount
}

// MachineCount는 기계별 건수다.
type MachineCount struct {
	Label string
	Count int
}

// DailyDigest는 지정한 날짜의 매장별 요약을 만든다.
func (s *Store) DailyDigest(ctx context.Context, day time.Time) ([]DigestRow, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.Add(24 * time.Hour)

	rows, err := s.pool.Query(ctx, `
		SELECT store_id, COUNT(*),
		       COUNT(*) FILTER (WHERE status='paid'),
		       COALESCE(SUM(amount_krw) FILTER (WHERE status='paid'), 0)
		  FROM claims WHERE created_at >= $1 AND created_at < $2
		 GROUP BY store_id`, start, end)
	if err != nil {
		return nil, fmt.Errorf("일일 요약 집계: %w", err)
	}
	defer rows.Close()

	out := []DigestRow{}
	for rows.Next() {
		var d DigestRow
		if err := rows.Scan(&d.StoreID, &d.ClaimCount, &d.PaidCount, &d.PaidTotalKRW); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		top, err := s.topMachines(ctx, out[i].StoreID, start, end, 3)
		if err != nil {
			return nil, err
		}
		out[i].TopMachines = top
	}
	return out, nil
}

func (s *Store) topMachines(ctx context.Context, storeID string, start, end time.Time, n int) ([]MachineCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.label, COUNT(*) FROM claims c JOIN machines m ON m.id = c.machine_id
		 WHERE c.store_id=$1 AND c.created_at >= $2 AND c.created_at < $3
		 GROUP BY m.label ORDER BY COUNT(*) DESC LIMIT $4`, storeID, start, end, n)
	if err != nil {
		return nil, fmt.Errorf("상위 기계 조회: %w", err)
	}
	defer rows.Close()
	out := []MachineCount{}
	for rows.Next() {
		var mc MachineCount
		if err := rows.Scan(&mc.Label, &mc.Count); err != nil {
			return nil, err
		}
		out = append(out, mc)
	}
	return out, rows.Err()
}

// ExpiredPhotos는 보존 기간이 지난 사진을 반환한다.
func (s *Store) ExpiredPhotos(ctx context.Context, now time.Time, limit int) ([]Photo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, claim_id, object_key, content_type, size_bytes, expires_at
		  FROM claim_photos WHERE expires_at <= $1 ORDER BY expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("만료 사진 조회: %w", err)
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.ClaimID, &p.ObjectKey, &p.ContentType, &p.SizeBytes, &p.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePhotoRow는 사진 행을 지운다.
// 파일 삭제가 실패해도(이미 없어도) 행은 지운다 — 고아 행이 영원히 남으면
// 워커가 매번 같은 사진을 다시 시도하며 로그만 채운다.
func (s *Store) DeletePhotoRow(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM claim_photos WHERE id=$1`, id); err != nil {
		return fmt.Errorf("사진 행 삭제: %w", err)
	}
	return nil
}
