//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// 기간 경계를 서버 한 곳에서만 정하기로 했으므로, 그 한 곳이 맞아야 한다.
// 월말과 연초에서 틀리기 쉬운 자리다.
func TestSettlementRange_기간_경계(t *testing.T) {
	cases := []struct {
		name             string
		now              string
		period           string
		wantFrom, wantTo string
	}{
		// 2026-10-26 은 월요일이다.
		{"이번주 월요일", "2026-10-26", "week", "2026-10-26", "2026-10-26"},
		{"이번주 일요일", "2026-11-01", "week", "2026-10-26", "2026-11-01"},
		{"이번달", "2026-10-26", "month", "2026-10-01", "2026-10-26"},
		{"지난달", "2026-10-26", "last_month", "2026-09-01", "2026-09-30"},
		// 1월에 지난달을 물으면 작년 12월이다.
		{"연초의 지난달", "2026-01-15", "last_month", "2025-12-01", "2025-12-31"},
		// 3월에 지난달을 물으면 2월이다. 윤년이 아닌 해는 28일까지.
		{"2월", "2026-03-10", "last_month", "2026-02-01", "2026-02-28"},
		// 모르는 값은 이번 주로 떨어진다.
		{"모르는 값", "2026-10-28", "nonsense", "2026-10-26", "2026-10-28"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now, _ := time.Parse("2006-01-02", c.now)
			from, to := SettlementRange(c.period, now)
			if got := from.Format("2006-01-02"); got != c.wantFrom {
				t.Errorf("from = %s, want %s", got, c.wantFrom)
			}
			if got := to.Format("2006-01-02"); got != c.wantTo {
				t.Errorf("to = %s, want %s", got, c.wantTo)
			}
		})
	}
}

func TestSettlementFor_집계하고_전기간과_비교한다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	// 이번 기간 3건(2건 송금), 직전 기간 1건(1건 송금).
	now := time.Now().UTC().Truncate(24 * time.Hour)
	from, to := now.AddDate(0, 0, -6), now

	mk := func(key string, at time.Time, amount int, paid bool) {
		in := claimInput(storeID, m.ID, key)
		in.AmountKRW = amount
		res, err := s.CreateClaim(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		status := "pending"
		if paid {
			status = "paid"
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE claims SET created_at=$1, status=$2 WHERE id=$3`, at, status, res.ID); err != nil {
			t.Fatal(err)
		}
	}
	mk("a", now.AddDate(0, 0, -1), 2000, true)
	mk("b", now.AddDate(0, 0, -1), 3000, true)
	mk("c", now.AddDate(0, 0, -3), 5000, false)
	mk("prev", now.AddDate(0, 0, -10), 1000, true) // 직전 기간

	st, err := s.SettlementFor(ctx, storeID, from, to, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClaimCount != 3 {
		t.Errorf("ClaimCount = %d, want 3", st.ClaimCount)
	}
	if st.PaidCount != 2 || st.PaidTotalKRW != 5000 {
		t.Errorf("Paid = %d건 %d원, want 2건 5000원", st.PaidCount, st.PaidTotalKRW)
	}
	if st.PrevClaimCount != 1 || st.PrevPaidTotalKRW != 1000 {
		t.Errorf("전기간 = %d건 %d원, want 1건 1000원", st.PrevClaimCount, st.PrevPaidTotalKRW)
	}
	if st.OpenCount != 1 {
		t.Errorf("OpenCount = %d, want 1", st.OpenCount)
	}

	// 빈 날이 빠지면 막대 그래프가 "없는 날"로 그려져 추세가 거짓말을 한다.
	if len(st.Days) != 7 {
		t.Fatalf("Days = %d개, want 7 (빈 날도 0 으로 채워야 한다)", len(st.Days))
	}
	var sum int
	for _, d := range st.Days {
		sum += d.PaidTotalKRW
	}
	if sum != st.PaidTotalKRW {
		t.Errorf("날짜별 합 %d != 기간 합계 %d", sum, st.PaidTotalKRW)
	}

	// 기계가 한 대뿐이므로 한 줄, 3건이라 임계치(3) 이상 → 점검 필요.
	if len(st.Machines) != 1 {
		t.Fatalf("Machines = %d개", len(st.Machines))
	}
	if st.Machines[0].ClaimCount != 3 || !st.Machines[0].NeedsCheck {
		t.Errorf("기계 = %d건 점검필요=%v", st.Machines[0].ClaimCount, st.Machines[0].NeedsCheck)
	}
}

// to 를 날짜로만 받으면 그날 오후 건이 통째로 빠진다. 사장님이 저녁에
// 열어 보면 오늘 처리한 게 하나도 없는 것처럼 보인다.
func TestSettlementFor_오늘_오후_건도_들어간다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "today"))
	if err != nil {
		t.Fatal(err)
	}
	// 오늘 23:30 에 접수된 건.
	if _, err := s.pool.Exec(ctx, `UPDATE claims SET created_at=$1 WHERE id=$2`,
		today.Add(23*time.Hour+30*time.Minute), res.ID); err != nil {
		t.Fatal(err)
	}

	st, err := s.SettlementFor(ctx, storeID, today, today, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClaimCount != 1 {
		t.Errorf("ClaimCount = %d, want 1 — 그날 끝까지 포함해야 한다", st.ClaimCount)
	}
}

// 다른 매장 숫자가 섞이면 정산이 통째로 틀린다.
func TestSettlementFor_매장_범위를_벗어나지_않는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeA, mA := fixture(t, s)

	storeB, _, err := createStore(ctx, s, "남의 매장", "0212345679")
	if err != nil {
		t.Fatal(err)
	}
	mB, err := s.CreateMachine(ctx, storeB, "남의 기계", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClaim(ctx, claimInput(storeA, mA.ID, "a")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClaim(ctx, claimInput(storeB, mB.ID, "b")); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	st, err := s.SettlementFor(ctx, storeA, now, now, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st.ClaimCount != 1 {
		t.Errorf("ClaimCount = %d, want 1 — 남의 매장 건이 섞였다", st.ClaimCount)
	}
	for _, m := range st.Machines {
		if m.Label == "남의 기계" {
			t.Error("남의 매장 기계가 목록에 있다")
		}
	}
}
