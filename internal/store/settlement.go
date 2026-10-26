package store

import (
	"context"
	"fmt"
	"time"
)

// Settlement은 정산/통계 화면 한 기간분이다 (docs/DESIGN-V2.md §6.2 ④).
//
// 한 응답에 전기간까지 담는 이유: 프런트가 두 번 부르면 기간 경계 계산이
// 두 곳에 생기고, 월말·윤년에서 어긋난다. 경계는 한 곳에서만 정한다.
type Settlement struct {
	From, To time.Time
	// 직전 같은 길이의 기간. "이번 달"이면 지난달이다.
	PrevFrom, PrevTo time.Time

	ClaimCount   int
	PaidCount    int
	PaidTotalKRW int

	PrevClaimCount   int
	PrevPaidCount    int
	PrevPaidTotalKRW int

	// 날짜별 나간 금액. 빈 날도 0 으로 채워 들어간다 — 프런트가 막대를
	// 그릴 때 빠진 날을 추측하게 하면 안 된다.
	Days []SettlementDay
	// 기계별. 환불액 많은 순.
	Machines []SettlementMachine

	// 결제 수단별 송금 건수. 사장님이 실제로 해야 하는 행동이 다르다
	// (계좌이체 vs 단말기 취소).
	PaidByCash int
	PaidByCard int

	RejectedCount int
	// 아직 손대지 않은 건. pending·needs_review·on_hold 를 합친다.
	OpenCount int
}

type SettlementDay struct {
	Day          time.Time
	ClaimCount   int
	PaidTotalKRW int
}

type SettlementMachine struct {
	MachineID    string
	Label        string
	ClaimCount   int
	PaidTotalKRW int
	// NeedsCheck 는 이 기간에 신고가 임계치 이상 몰린 기계다.
	NeedsCheck bool
}

// SettlementFor는 기간 집계를 한 번에 읽는다.
//
// 쿼리를 여러 번 나눠 쏘지 않는다. 사장님 서버는 1GB 짜리이고, 이 화면은
// 한 번 열 때마다 전부 필요하다. 왕복이 늘면 그만큼 느려진다.
func (s *Store) SettlementFor(ctx context.Context, storeID string, from, to time.Time, alertCount int) (Settlement, error) {
	// to 는 "그날 끝"까지다. 날짜만 받아 반나절이 빠지면 오늘 건이
	// 집계에서 통째로 사라진다.
	fromD := from.Truncate(24 * time.Hour)
	toD := to.Truncate(24 * time.Hour).Add(24 * time.Hour)
	span := toD.Sub(fromD)
	prevFrom, prevTo := fromD.Add(-span), fromD

	out := Settlement{
		From: fromD, To: toD.Add(-time.Nanosecond),
		PrevFrom: prevFrom, PrevTo: prevTo.Add(-time.Nanosecond),
	}

	// 1) 기간 합계 — 이번 기간과 직전 기간을 한 번에.
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
		  COUNT(*) FILTER (WHERE status='paid' AND created_at >= $2 AND created_at < $3),
		  COALESCE(SUM(amount_krw) FILTER (WHERE status='paid' AND created_at >= $2 AND created_at < $3), 0),
		  COUNT(*) FILTER (WHERE created_at >= $4 AND created_at < $2),
		  COUNT(*) FILTER (WHERE status='paid' AND created_at >= $4 AND created_at < $2),
		  COALESCE(SUM(amount_krw) FILTER (WHERE status='paid' AND created_at >= $4 AND created_at < $2), 0),
		  COUNT(*) FILTER (WHERE status='rejected' AND created_at >= $2 AND created_at < $3),
		  COUNT(*) FILTER (WHERE status IN ('pending','needs_review','on_hold') AND created_at >= $2 AND created_at < $3),
		  COUNT(*) FILTER (WHERE status='paid' AND payment_method='cash' AND created_at >= $2 AND created_at < $3),
		  COUNT(*) FILTER (WHERE status='paid' AND payment_method='card' AND created_at >= $2 AND created_at < $3)
		  FROM claims WHERE store_id=$1`,
		storeID, fromD, toD, prevFrom,
	).Scan(&out.ClaimCount, &out.PaidCount, &out.PaidTotalKRW,
		&out.PrevClaimCount, &out.PrevPaidCount, &out.PrevPaidTotalKRW,
		&out.RejectedCount, &out.OpenCount, &out.PaidByCash, &out.PaidByCard)
	if err != nil {
		return Settlement{}, fmt.Errorf("기간 합계: %w", err)
	}

	// 2) 날짜별.
	byDay := map[time.Time]SettlementDay{}
	rows, err := s.pool.Query(ctx, `
		SELECT date_trunc('day', created_at) AS d,
		       COUNT(*),
		       COALESCE(SUM(amount_krw) FILTER (WHERE status='paid'), 0)
		  FROM claims
		 WHERE store_id=$1 AND created_at >= $2 AND created_at < $3
		 GROUP BY d`, storeID, fromD, toD)
	if err != nil {
		return Settlement{}, fmt.Errorf("날짜별 집계: %w", err)
	}
	for rows.Next() {
		var d SettlementDay
		if err := rows.Scan(&d.Day, &d.ClaimCount, &d.PaidTotalKRW); err != nil {
			rows.Close()
			return Settlement{}, err
		}
		byDay[d.Day.UTC().Truncate(24*time.Hour)] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Settlement{}, err
	}

	// 빈 날을 0 으로 채운다. 막대 그래프에서 빠진 날은 "0 인 날"이
	// 아니라 "없는 날"로 그려져 추세가 거짓말을 한다.
	for d := fromD; d.Before(toD); d = d.AddDate(0, 0, 1) {
		k := d.UTC().Truncate(24 * time.Hour)
		if v, ok := byDay[k]; ok {
			v.Day = d
			out.Days = append(out.Days, v)
		} else {
			out.Days = append(out.Days, SettlementDay{Day: d})
		}
	}

	// 3) 기계별.
	mrows, err := s.pool.Query(ctx, `
		SELECT m.id, m.label, COUNT(c.id),
		       COALESCE(SUM(c.amount_krw) FILTER (WHERE c.status='paid'), 0)
		  FROM machines m
		  LEFT JOIN claims c
		         ON c.machine_id = m.id AND c.created_at >= $2 AND c.created_at < $3
		 WHERE m.store_id=$1 AND m.active
		 GROUP BY m.id, m.label
		 ORDER BY 4 DESC, 3 DESC, m.label`, storeID, fromD, toD)
	if err != nil {
		return Settlement{}, fmt.Errorf("기계별 집계: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var m SettlementMachine
		if err := mrows.Scan(&m.MachineID, &m.Label, &m.ClaimCount, &m.PaidTotalKRW); err != nil {
			return Settlement{}, err
		}
		m.NeedsCheck = alertCount > 0 && m.ClaimCount >= alertCount
		out.Machines = append(out.Machines, m)
	}
	return out, mrows.Err()
}

// SettlementRange는 기간 이름을 실제 날짜로 바꾼다.
//
// 경계 계산을 여기 한 곳에 둔다. 프런트와 서버가 각자 계산하면 월말과
// 윤년에서 어긋나고, 어긋난 걸 알아챌 방법이 없다.
func SettlementRange(period string, now time.Time) (from, to time.Time) {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	switch period {
	case "month":
		return time.Date(y, m, 1, 0, 0, 0, 0, now.Location()), today
	case "last_month":
		first := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		prev := first.AddDate(0, -1, 0)
		return prev, first.AddDate(0, 0, -1)
	default: // week — 월요일 시작. 한국에서 "이번 주"는 월요일부터다.
		off := (int(today.Weekday()) + 6) % 7
		return today.AddDate(0, 0, -off), today
	}
}
