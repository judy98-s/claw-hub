//go:build integration

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/domain"
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://clawhub@/clawhub?host=/var/run/postgresql"
}

// newStore는 깨끗한 스키마의 Store를 만든다.
func newStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	pepper := make([]byte, 32)
	for i := range pepper {
		pepper[i] = byte(255 - i)
	}
	c, err := crypto.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	h, err := crypto.NewHasher(pepper)
	if err != nil {
		t.Fatal(err)
	}

	s, err := Open(ctx, testDSN(), c, h)
	if err != nil {
		t.Skipf("Postgres에 연결할 수 없습니다 (%v). docker compose up -d postgres 후 재시도하세요.", err)
	}
	t.Cleanup(s.Close)

	// 테스트마다 스키마를 새로 만든다. 테스트 간 데이터가 새면
	// 30일 카운트 같은 집계 검증이 조용히 거짓 양성이 된다.
	if _, err := s.pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("스키마 초기화: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("마이그레이션: %v", err)
	}
	return s
}

// fixture는 매장 하나와 기계 하나를 만든다.
func fixture(t *testing.T, s *Store) (storeID string, machine domain.Machine) {
	t.Helper()
	ctx := context.Background()
	storeID, err := s.CreateStore(ctx, "테스트 매장", "0212345678")
	if err != nil {
		t.Fatal(err)
	}
	machine, err = s.CreateMachine(ctx, storeID, "3번 기계", "입구 왼쪽")
	if err != nil {
		t.Fatal(err)
	}
	return storeID, machine
}

func claimInput(storeID, machineID, key string) CreateClaimInput {
	return CreateClaimInput{
		StoreID: storeID, MachineID: machineID,
		IssueType: domain.IssueCashEaten, AmountKRW: 2000,
		Description:   "천원 두 번 넣었는데 안 나옴",
		PaymentMethod: domain.PaymentCash,
		Phone:         "01012345678", BankCode: "090", Account: "3333011234567", Holder: "김민수",
		Status: domain.StatusPending, IdempotencyKey: key,
		IP: "1.2.3.4", UserAgent: "test",
	}
}

// cardClaimInput은 카드로 결제한 손님의 신고다. 계좌 세 칸이 비어 있다.
func cardClaimInput(storeID, machineID, key string) CreateClaimInput {
	in := claimInput(storeID, machineID, key)
	in.PaymentMethod = domain.PaymentCard
	in.CardLast4 = "4821"
	in.PaidAtGuess = time.Date(2026, 9, 21, 15, 4, 0, 0, time.UTC)
	in.BankCode, in.Account, in.Holder = "", "", ""
	return in
}

func TestMigrate_두번_실행해도_안전하다(t *testing.T) {
	s := newStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("두 번째 마이그레이션 실패: %v", err)
	}
}

func TestCreateClaim_저장하고_평문으로_되읽는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "key-1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Existing {
		t.Error("첫 접수인데 Existing=true")
	}

	d, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{Kind: domain.ActorOwner, ID: ""})
	if err != nil {
		t.Fatal(err)
	}
	if d.Phone != "01012345678" {
		t.Errorf("Phone = %q, 복호화가 안 됐다", d.Phone)
	}
	if d.Account != "3333011234567" {
		t.Errorf("Account = %q", d.Account)
	}
	if d.Holder != "김민수" {
		t.Errorf("Holder = %q", d.Holder)
	}
	if d.MachineLabel != "3번 기계" {
		t.Errorf("MachineLabel = %q", d.MachineLabel)
	}
}

func TestCreateClaim_평문이_디스크에_없다(t *testing.T) {
	// 암호화 컬럼에 실수로 평문을 넣는 회귀를 막는다.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "key-1")); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM claims
		 WHERE encode(phone_enc,'escape')   LIKE '%01012345678%'
		    OR encode(account_enc,'escape') LIKE '%3333011234567%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("암호화 컬럼에 평문이 그대로 들어 있다")
	}
}

func TestCreateClaim_멱등키_중복은_기존건을_돌려준다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	first, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "same-key"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "same-key"))
	if err != nil {
		t.Fatalf("중복 접수가 에러가 됐다: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("다른 건이 만들어졌다: %s vs %s", first.ID, second.ID)
	}
	if !second.Existing {
		t.Error("Existing=false — 중복임을 알려주지 않는다")
	}

	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM claims`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("claim이 %d건 생성됐다, want 1 — 이중 환불로 이어진다", n)
	}
}

func TestCreateClaim_동시_멱등키_경합(t *testing.T) {
	// Review Focus #1. 모바일에서 제출 버튼 연타나 재시도가 동시에 도착한다.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	const n = 8
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "race-key"))
			ids[i], errs[i] = res.ID, err
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("고루틴 %d 실패: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if ids[i] != ids[0] {
			t.Errorf("서로 다른 claim ID가 나왔다: %s vs %s", ids[0], ids[i])
		}
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM claims`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("동시 요청 %d건에서 claim이 %d건 생성됐다, want 1", n, count)
	}
}

func TestCreateClaim_다른_매장이면_같은_멱등키도_따로_저장된다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeA, mA := fixture(t, s)
	storeB, err := s.CreateStore(ctx, "다른 매장", "")
	if err != nil {
		t.Fatal(err)
	}
	mB, err := s.CreateMachine(ctx, storeB, "1번", "")
	if err != nil {
		t.Fatal(err)
	}

	a, err := s.CreateClaim(ctx, claimInput(storeA, mA.ID, "shared-key"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateClaim(ctx, claimInput(storeB, mB.ID, "shared-key"))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Error("다른 매장의 같은 멱등키가 같은 건으로 합쳐졌다")
	}
}

func TestCreateClaim_0원은_DB가_거부한다(t *testing.T) {
	// 애플리케이션 검증이 뚫려도 CHECK 제약이 받아낸다.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	in := claimInput(storeID, m.ID, "zero")
	in.AmountKRW = 0
	if _, err := s.CreateClaim(ctx, in); err == nil {
		t.Fatal("0원 접수가 저장됐다")
	}
}

func TestMachineByCode_비활성은_찾지_못한다(t *testing.T) {
	// Review Focus #4. 스티커가 떼여 다른 곳에 붙거나 폐기된 기계.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	if _, _, err := s.MachineByCode(ctx, m.Code); err != nil {
		t.Fatalf("활성 기계를 못 찾는다: %v", err)
	}
	if err := s.UpdateMachine(ctx, storeID, m.ID, m.Label, m.Location, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.MachineByCode(ctx, m.Code); !errors.Is(err, ErrNotFound) {
		t.Errorf("비활성 기계가 조회됐다: %v", err)
	}
	if _, _, err := s.MachineByCode(ctx, "ZZZZZZ"); !errors.Is(err, ErrNotFound) {
		t.Errorf("없는 코드에 ErrNotFound가 아니다: %v", err)
	}
}

func TestMachineByCode_매장_연락처를_함께_준다(t *testing.T) {
	// 기계를 못 찾았을 때 손님에게 안내할 번호다.
	ctx := context.Background()
	s := newStore(t)
	_, m := fixture(t, s)
	_, info, err := s.MachineByCode(ctx, m.Code)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "테스트 매장" || info.Phone != "0212345678" {
		t.Errorf("매장 정보 = %+v", info)
	}
}

func TestRiskFactsFor_30일_경계를_정확히_센다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	now := time.Now()

	// 29일 전(포함), 31일 전(제외)에 각각 한 건씩 심는다.
	for i, age := range []time.Duration{29 * 24 * time.Hour, 31 * 24 * time.Hour} {
		res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, fmt.Sprintf("old-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE claims SET created_at=$1 WHERE id=$2`, now.Add(-age), res.ID); err != nil {
			t.Fatal(err)
		}
	}

	facts, err := s.RiskFactsFor(ctx, RiskQuery{
		StoreID: storeID, MachineID: m.ID,
		Phone: "01012345678", Account: "3333011234567", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 29일 전 1건 + 이번 건 1건 = 2. 31일 전 건은 빠져야 한다.
	if facts.PhoneClaims30d != 2 {
		t.Errorf("PhoneClaims30d = %d, want 2 (29일 전 1건 + 이번 건)", facts.PhoneClaims30d)
	}
}

func TestRiskFactsFor_계좌공유_탐지(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	// 같은 계좌에 서로 다른 번호 두 개가 이미 붙어 있다.
	for i, phone := range []string{"01011112222", "01033334444"} {
		in := claimInput(storeID, m.ID, fmt.Sprintf("shared-acct-%d", i))
		in.Phone = phone
		if _, err := s.CreateClaim(ctx, in); err != nil {
			t.Fatal(err)
		}
	}

	facts, err := s.RiskFactsFor(ctx, RiskQuery{
		StoreID: storeID, MachineID: m.ID,
		Phone: "01055556666", Account: "3333011234567", Now: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.AccountDistinctPhones != 3 {
		t.Errorf("AccountDistinctPhones = %d, want 3 (기존 2 + 이번 1)", facts.AccountDistinctPhones)
	}
}

func TestRiskFactsFor_수동상태는_더_강한쪽이_이긴다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	accountHash := s.hasher.Hash("3333011234567")
	if err := s.SetContactStatus(ctx, storeID, accountHash, "account", domain.ContactBlocked, "반복 허위신고", ""); err != nil {
		t.Fatal(err)
	}

	// 번호는 아무 상태도 없지만 계좌가 차단이면 차단으로 잡혀야 한다.
	facts, err := s.RiskFactsFor(ctx, RiskQuery{
		StoreID: storeID, MachineID: m.ID,
		Phone: "01099998888", Account: "3333011234567", Now: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.ManualStatus != domain.ContactBlocked {
		t.Errorf("ManualStatus = %q, want blocked — 번호를 바꿔도 계좌로 잡혀야 한다", facts.ManualStatus)
	}
}

func TestRiskFactsFor_직전_같은기계_신고시각(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	before, err := s.RiskFactsFor(ctx, RiskQuery{StoreID: storeID, MachineID: m.ID,
		Phone: "01012345678", Account: "3333011234567", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !before.LastSameMachineAt.IsZero() {
		t.Error("이력이 없는데 LastSameMachineAt이 채워졌다")
	}

	if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1")); err != nil {
		t.Fatal(err)
	}
	after, err := s.RiskFactsFor(ctx, RiskQuery{StoreID: storeID, MachineID: m.ID,
		Phone: "01012345678", Account: "3333011234567", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if after.LastSameMachineAt.IsZero() {
		t.Error("직전 신고가 있는데 LastSameMachineAt이 비었다")
	}
}

func TestApplyTransition_상태와_이벤트가_함께_기록된다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}

	c := &domain.Claim{ID: res.ID, StoreID: storeID, Status: domain.StatusPending}
	ev, err := c.Transition(domain.StatusApproved, domain.Actor{Kind: domain.ActorOwner}, "사진 확인")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}

	d, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != domain.StatusApproved {
		t.Errorf("Status = %s", d.Status)
	}
	var found bool
	for _, e := range d.Events {
		if e.Action == domain.ActionTransition && e.To == domain.StatusApproved && e.Note == "사진 확인" {
			found = true
		}
	}
	if !found {
		t.Errorf("전이 이벤트가 없다: %+v", d.Events)
	}
}

func TestApplyTransition_그사이_남이_바꿨으면_실패한다(t *testing.T) {
	// 사장님 두 명이 같은 건을 동시에 처리하는 경우.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}

	c1 := &domain.Claim{ID: res.ID, Status: domain.StatusPending}
	ev1, _ := c1.Transition(domain.StatusApproved, domain.Actor{Kind: domain.ActorOwner}, "")
	if err := s.ApplyTransition(ctx, storeID, c1, ev1); err != nil {
		t.Fatal(err)
	}

	// 두 번째 사장님은 아직 pending 이라고 알고 있다.
	c2 := &domain.Claim{ID: res.ID, Status: domain.StatusPending}
	ev2, _ := c2.Transition(domain.StatusRejected, domain.Actor{Kind: domain.ActorOwner}, "")
	if err := s.ApplyTransition(ctx, storeID, c2, ev2); !errors.Is(err, ErrNotFound) {
		t.Errorf("낡은 상태 기준 전이가 통과됐다: %v", err)
	}
}

func TestClaimByID_다른_매장건은_찾지_못한다(t *testing.T) {
	// 403이 아니라 404여야 한다. 403은 "그 ID는 존재한다"를 알려준다.
	ctx := context.Background()
	s := newStore(t)
	storeA, mA := fixture(t, s)
	storeB, err := s.CreateStore(ctx, "남의 매장", "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.CreateClaim(ctx, claimInput(storeA, mA.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimByID(ctx, storeB, res.ID, domain.Actor{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("남의 매장 건이 조회됐다: %v", err)
	}
}

func TestClaimByID_열람이_감사로그에_남는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{Kind: domain.ActorOwner}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM claim_events WHERE claim_id=$1 AND action=$2`,
		res.ID, domain.ActionViewContact).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("열람 이벤트 %d건, want 1", n)
	}
}

func TestListClaims_필터와_커서(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	for i := 0; i < 5; i++ {
		in := claimInput(storeID, m.ID, fmt.Sprintf("k%d", i))
		if i%2 == 0 {
			in.Status = domain.StatusNeedsReview
		}
		if _, err := s.CreateClaim(ctx, in); err != nil {
			t.Fatal(err)
		}
	}

	all, next, err := s.ListClaims(ctx, ClaimFilter{StoreID: storeID})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Errorf("전체 %d건, want 5", len(all))
	}
	if next != "" {
		t.Errorf("5건뿐인데 다음 커서가 있다: %q", next)
	}

	filtered, _, err := s.ListClaims(ctx, ClaimFilter{
		StoreID: storeID, Statuses: []domain.Status{domain.StatusNeedsReview}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 3 {
		t.Errorf("needs_review %d건, want 3", len(filtered))
	}

	page1, cur, err := s.ListClaims(ctx, ClaimFilter{StoreID: storeID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || cur == "" {
		t.Fatalf("1페이지 %d건, 커서 %q", len(page1), cur)
	}
	page2, _, err := s.ListClaims(ctx, ClaimFilter{StoreID: storeID, Limit: 2, Cursor: cur})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range append(page1, page2...) {
		if seen[c.ID] {
			t.Errorf("페이지 사이에 중복된 건: %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestListClaims_전화번호는_마스킹되어_나간다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1")); err != nil {
		t.Fatal(err)
	}
	list, _, err := s.ListClaims(ctx, ClaimFilter{StoreID: storeID})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].PhoneMasked != "010-****-5678" {
		t.Errorf("PhoneMasked = %q", list[0].PhoneMasked)
	}
}

func TestListContacts_건수_내림차순_마스킹(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	// 01011112222 는 3건, 01033334444 는 1건
	for i := 0; i < 3; i++ {
		in := claimInput(storeID, m.ID, fmt.Sprintf("a%d", i))
		in.Phone = "01011112222"
		if _, err := s.CreateClaim(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	in := claimInput(storeID, m.ID, "b0")
	in.Phone = "01033334444"
	if _, err := s.CreateClaim(ctx, in); err != nil {
		t.Fatal(err)
	}

	contacts, err := s.ListContacts(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 2 {
		t.Fatalf("연락처 %d개, want 2", len(contacts))
	}
	if contacts[0].ClaimCount != 3 {
		t.Errorf("첫 행 건수 = %d, want 3 (내림차순이 아니다)", contacts[0].ClaimCount)
	}
	if contacts[0].PhoneMasked != "010-****-2222" {
		t.Errorf("PhoneMasked = %q", contacts[0].PhoneMasked)
	}
	if contacts[0].ManualStatus != domain.ContactNormal {
		t.Errorf("ManualStatus = %q, want normal", contacts[0].ManualStatus)
	}
}

func TestDailyStats_빈날도_0으로_채운다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)
	now := time.Now()

	stats, err := s.DailyStats(ctx, storeID, now.AddDate(0, 0, -6), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 7 {
		t.Errorf("일수 = %d, want 7 — 빈 날을 건너뛰면 그래프가 거짓말한다", len(stats))
	}
	for _, d := range stats {
		if d.ClaimCount != 0 {
			t.Errorf("%s: 접수가 없는데 %d건", d.Day, d.ClaimCount)
		}
	}
}

func TestMachinesOverThreshold(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	for i := 0; i < 2; i++ {
		if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, fmt.Sprintf("k%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	over, err := s.MachinesOverThreshold(ctx, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(over) != 0 {
		t.Errorf("2건인데 임계 3에서 걸렸다: %+v", over)
	}

	if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k3")); err != nil {
		t.Fatal(err)
	}
	over, err = s.MachinesOverThreshold(ctx, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(over) != 1 {
		t.Fatalf("3건인데 임계 3에서 안 걸렸다: %+v", over)
	}
	if over[0].Count != 3 || over[0].Label != "3번 기계" {
		t.Errorf("활동 = %+v", over[0])
	}
	if over[0].ByIssue[domain.IssueCashEaten] != 3 {
		t.Errorf("증상별 집계 = %+v", over[0].ByIssue)
	}
}

func TestExpiredPhotos_그리고_행삭제(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	in := claimInput(storeID, m.ID, "k1")
	in.Photos = []PhotoInput{
		{ObjectKey: "a.jpg", ContentType: "image/jpeg", SizeBytes: 100, ExpiresAt: time.Now().Add(-time.Hour)},
		{ObjectKey: "b.jpg", ContentType: "image/jpeg", SizeBytes: 100, ExpiresAt: time.Now().Add(time.Hour)},
	}
	if _, err := s.CreateClaim(ctx, in); err != nil {
		t.Fatal(err)
	}

	expired, err := s.ExpiredPhotos(ctx, time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].ObjectKey != "a.jpg" {
		t.Fatalf("만료 사진 = %+v, want a.jpg 1장", expired)
	}
	if err := s.DeletePhotoRow(ctx, expired[0].ID); err != nil {
		t.Fatal(err)
	}
	again, err := s.ExpiredPhotos(ctx, time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("삭제 후에도 만료 사진이 남았다: %+v", again)
	}
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	if _, err := s.CreateUser(ctx, storeID, "owner@example.com", "secret123", "사장님", "01011112222"); err != nil {
		t.Fatal(err)
	}

	u, err := s.Authenticate(ctx, "owner@example.com", "secret123")
	if err != nil {
		t.Fatalf("올바른 비밀번호가 거부됐다: %v", err)
	}
	if u.StoreID != storeID {
		t.Errorf("StoreID = %q", u.StoreID)
	}

	// 없는 이메일과 틀린 비밀번호가 같은 에러여야 한다.
	_, errWrong := s.Authenticate(ctx, "owner@example.com", "wrong")
	_, errNoUser := s.Authenticate(ctx, "nobody@example.com", "secret123")
	if !errors.Is(errWrong, ErrNotFound) || !errors.Is(errNoUser, ErrNotFound) {
		t.Errorf("에러가 구분된다: wrong=%v noUser=%v — 가입 여부가 샌다", errWrong, errNoUser)
	}
}

func TestAuthenticate_반복실패하면_잠긴다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)
	if _, err := s.CreateUser(ctx, storeID, "owner@example.com", "secret123", "", ""); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < maxLoginFailures; i++ {
		if _, err := s.Authenticate(ctx, "owner@example.com", "wrong"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%d번째 실패 에러 = %v", i+1, err)
		}
	}
	// 이제 올바른 비밀번호여도 잠겨 있어야 한다.
	if _, err := s.Authenticate(ctx, "owner@example.com", "secret123"); !errors.Is(err, ErrLocked) {
		t.Errorf("%d번 실패 후에도 잠기지 않았다: %v", maxLoginFailures, err)
	}
}

func TestCreateMachine_코드는_전역_유니크(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		m, err := s.CreateMachine(ctx, storeID, fmt.Sprintf("%d번", i), "")
		if err != nil {
			t.Fatal(err)
		}
		if seen[m.Code] {
			t.Fatalf("코드 중복: %s", m.Code)
		}
		seen[m.Code] = true
		if err := domain.ValidateMachineCode(m.Code); err != nil {
			t.Fatalf("생성된 코드가 형식에 안 맞는다 %q: %v", m.Code, err)
		}
	}
}

func TestListMachines_7일_추이는_항상_7칸(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1")); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListMachines(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("기계 %d대", len(list))
	}
	if len(list[0].Daily7d) != 7 {
		t.Errorf("Daily7d 길이 = %d, want 7", len(list[0].Daily7d))
	}
	if list[0].Claims24h != 1 || list[0].OpenClaims != 1 {
		t.Errorf("Claims24h=%d OpenClaims=%d", list[0].Claims24h, list[0].OpenClaims)
	}
}

func TestUserProfile_수정과_직원추가(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	ownerAcct, err := s.CreateUser(ctx, storeID, "owner@example.com", "secret123", "사장님", "")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateUser(ctx, storeID, ownerAcct.ID, "손지영", "01099998888")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "손지영" || updated.Phone != "01099998888" {
		t.Errorf("수정 결과 = %+v", updated)
	}

	if _, err := s.CreateUser(ctx, storeID, "staff@example.com", "staffpass1", "김직원", ""); err != nil {
		t.Fatal(err)
	}
	users, err := s.ListUsers(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Errorf("계정 %d개, want 2", len(users))
	}
}

func TestSetUserActive_비활성_계정은_로그인도_조회도_안된다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	if _, err := s.CreateUser(ctx, storeID, "owner@example.com", "secret123", "사장님", ""); err != nil {
		t.Fatal(err)
	}
	staff, err := s.CreateUser(ctx, storeID, "staff@example.com", "staffpass1", "김직원", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetUserActive(ctx, storeID, staff.ID, false); err != nil {
		t.Fatal(err)
	}
	// 비밀번호가 맞아도 들어올 수 없고, 에러는 계정이 없을 때와 같다.
	if _, err := s.Authenticate(ctx, "staff@example.com", "staffpass1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("비활성 계정 로그인 에러 = %v, want ErrNotFound", err)
	}
	// 세션 쿠키가 남아 있어도 UserByID 에서 막힌다.
	if _, err := s.UserByID(ctx, staff.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("비활성 계정 조회 에러 = %v", err)
	}
}

func TestSetUserActive_마지막_계정은_막는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	only, err := s.CreateUser(ctx, storeID, "owner@example.com", "secret123", "사장님", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserActive(ctx, storeID, only.ID, false); err == nil {
		t.Fatal("마지막 계정이 비활성화됐다 — 아무도 로그인하지 못한다")
	}
}

func TestStoreDetail_수정(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	d, err := s.StoreByID(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "테스트 매장" || d.Phone != "0212345678" {
		t.Errorf("매장 = %+v", d)
	}

	upd, err := s.UpdateStore(ctx, storeID, "채현이 매장", "023334444")
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "채현이 매장" || upd.Phone != "023334444" {
		t.Errorf("수정 결과 = %+v", upd)
	}

	// 손님 화면에도 바뀐 정보가 나가야 한다.
	_, info, err := s.MachineByCode(ctx, mustMachineCode(t, s, storeID))
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "채현이 매장" || info.Phone != "023334444" {
		t.Errorf("손님 화면 매장 정보 = %+v", info)
	}
}

func mustMachineCode(t *testing.T, s *Store, storeID string) string {
	t.Helper()
	list, err := s.ListMachines(context.Background(), storeID)
	if err != nil || len(list) == 0 {
		t.Fatalf("기계를 찾을 수 없다: %v", err)
	}
	return list[0].Code
}

func TestClaimDetail_누가_승인하고_누가_보냈는지(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	boss, err := s.CreateUser(ctx, storeID, "boss@example.com", "secret123", "손지영", "")
	if err != nil {
		t.Fatal(err)
	}
	staff, err := s.CreateUser(ctx, storeID, "staff@example.com", "staffpass1", "김직원", "")
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}

	// 사장님이 승인하고
	c := &domain.Claim{ID: res.ID, StoreID: storeID, AmountKRW: 2000, Status: domain.StatusPending}
	ev, err := c.Transition(domain.StatusApproved, domain.Actor{Kind: domain.ActorOwner, ID: boss.ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}

	// 직원이 보낸다
	c.PayoutMethod = domain.PayoutDeeplink
	c.PaidAmountKRW = 1500
	ev, err = c.Transition(domain.StatusPaid, domain.Actor{Kind: domain.ActorOwner, ID: staff.ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}

	d, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if d.ApprovedBy != "손지영" {
		t.Errorf("ApprovedBy = %q, want 손지영", d.ApprovedBy)
	}
	if d.PaidBy != "김직원" {
		t.Errorf("PaidBy = %q, want 김직원", d.PaidBy)
	}
	if d.PaidAmountKRW != 1500 {
		t.Errorf("PaidAmountKRW = %d, want 1500", d.PaidAmountKRW)
	}
	if d.AmountKRW != 2000 {
		t.Errorf("AmountKRW = %d, want 2000 — 요청액이 덮어쓰였다", d.AmountKRW)
	}
}

func TestClaimDetail_링크로_처리하면_Slack링크로_표시된다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	c := &domain.Claim{ID: res.ID, StoreID: storeID, AmountKRW: 2000, Status: domain.StatusPending}
	ev, _ := c.Transition(domain.StatusApproved, domain.Actor{Kind: domain.ActorLink}, "")
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}

	d, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if d.ApprovedBy != "Slack 링크" {
		t.Errorf("ApprovedBy = %q, want \"Slack 링크\"", d.ApprovedBy)
	}
}

func TestPaidAmount_집계는_실제_송금액을_쓴다(t *testing.T) {
	// 요청 3,000원인데 2,000원만 보냈으면 통계도 2,000원이어야 한다.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)
	now := time.Now()

	in := claimInput(storeID, m.ID, "k1")
	in.AmountKRW = 3000
	res, err := s.CreateClaim(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	c := &domain.Claim{ID: res.ID, StoreID: storeID, AmountKRW: 3000, Status: domain.StatusPending}
	ev, _ := c.Transition(domain.StatusApproved, domain.Actor{Kind: domain.ActorOwner}, "")
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}
	c.PayoutMethod = domain.PayoutManual
	c.PaidAmountKRW = 2000
	ev, _ = c.Transition(domain.StatusPaid, domain.Actor{Kind: domain.ActorOwner}, "")
	if err := s.ApplyTransition(ctx, storeID, c, ev); err != nil {
		t.Fatal(err)
	}

	stats, err := s.DailyStats(ctx, storeID, now.AddDate(0, 0, -1), now)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, d := range stats {
		total += d.PaidTotalKRW
	}
	if total != 2000 {
		t.Errorf("일별 집계 = %d원, want 2000 — 요청액을 쓰고 있다", total)
	}

	contacts, err := s.ListContacts(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].PaidTotalKRW != 2000 {
		t.Errorf("연락처 집계 = %+v, want 2000원", contacts)
	}
}

func TestPayoutSettings_왕복(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, _ := fixture(t, s)

	got, err := s.StoreByID(ctx, storeID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PayoutProvider != "" {
		t.Errorf("기본 provider = %q, want 빈 문자열", got.PayoutProvider)
	}

	upd, err := s.UpdatePayoutSettings(ctx, storeID, PayoutSettings{
		Provider: "toss", BankCode: "090", Account: "1002123456789",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd.PayoutProvider != "toss" || upd.PayoutAccount != "1002123456789" {
		t.Errorf("설정 = %+v", upd)
	}

	// 출금 계좌도 암호화되어 저장돼야 한다. 사장님 계좌도 개인정보다.
	var leaked int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM stores
		 WHERE encode(payout_account_enc,'escape') LIKE '%1002123456789%'`).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Error("출금 계좌가 평문으로 저장됐다")
	}
}

// ── 카드 결제 건 ────────────────────────────────────────────────────────

func TestCreateClaim_카드건은_계좌없이_저장되고_되읽힌다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	res, err := s.CreateClaim(ctx, cardClaimInput(storeID, m.ID, "card-1"))
	if err != nil {
		t.Fatal(err)
	}

	d, err := s.ClaimByID(ctx, storeID, res.ID, domain.Actor{Kind: domain.ActorOwner})
	if err != nil {
		t.Fatal(err)
	}
	if d.PaymentMethod != domain.PaymentCard {
		t.Errorf("PaymentMethod = %q", d.PaymentMethod)
	}
	if d.CardLast4 != "4821" {
		t.Errorf("CardLast4 = %q — 단말기에서 거래를 찾을 단서다", d.CardLast4)
	}
	if !d.PaidAtGuess.Equal(time.Date(2026, 9, 21, 15, 4, 0, 0, time.UTC)) {
		t.Errorf("PaidAtGuess = %v", d.PaidAtGuess)
	}
	if d.Account != "" {
		t.Errorf("Account = %q — 카드 건에는 계좌가 없어야 한다", d.Account)
	}
}

func TestRiskFactsFor_계좌없는_카드건은_계좌공유로_묶이지_않는다(t *testing.T) {
	// 카드 건은 계좌가 빈 문자열이다. 그 해시가 서로 같다고 "같은 계좌를
	// 공유한다"고 판정하면 카드 신고가 몇 건만 쌓여도 전부 검토 대상이 된다.
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	for i, phone := range []string{"01011112222", "01033334444"} {
		in := cardClaimInput(storeID, m.ID, fmt.Sprintf("card-share-%d", i))
		in.Phone = phone
		if _, err := s.CreateClaim(ctx, in); err != nil {
			t.Fatal(err)
		}
	}

	facts, err := s.RiskFactsFor(ctx, RiskQuery{
		StoreID: storeID, MachineID: m.ID,
		Phone: "01055556666", Account: "", Now: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.HasAccount {
		t.Fatal("계좌가 없는데 HasAccount=true")
	}

	// 규칙이 실제로 안 걸리는지까지 본다. 집계 숫자만 보면 도메인이
	// 그걸로 무엇을 하는지는 모른 채 지나간다.
	facts.AmountKRW = 2000
	for _, r := range domain.Evaluate(facts, domain.DefaultPolicy()).Reasons {
		if strings.Contains(r.Message, "계좌") {
			t.Errorf("계좌 공유 사유가 붙었다: %q", r.Message)
		}
	}
}

// ── 홈 요약 ────────────────────────────────────────────────────────────

func TestHomeSummaryFor_상태별로_세고_승인시각을_감사로그에서_읽는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	// 접수 3건: 하나는 승인까지, 하나는 보류, 하나는 그대로 둔다.
	ids := make([]string, 3)
	for i := range ids {
		res, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, fmt.Sprintf("home-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = res.ID
	}

	by := domain.Actor{Kind: domain.ActorOwner}
	approve := func(id string, to domain.Status) {
		t.Helper()
		d, err := s.ClaimByID(ctx, storeID, id, domain.Actor{})
		if err != nil {
			t.Fatal(err)
		}
		c := d.Claim
		ev, err := c.Transition(to, by, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ApplyTransition(ctx, storeID, &c, ev); err != nil {
			t.Fatal(err)
		}
	}
	approve(ids[0], domain.StatusApproved)
	approve(ids[1], domain.StatusOnHold)

	h, err := s.HomeSummaryFor(ctx, storeID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if h.Approved != 1 || h.OnHold != 1 || h.Pending != 1 {
		t.Fatalf("approved=%d on_hold=%d pending=%d", h.Approved, h.OnHold, h.Pending)
	}
	if h.ApprovedAmountKRW != 2000 {
		t.Errorf("ApprovedAmountKRW = %d, want 2000", h.ApprovedAmountKRW)
	}
	// approved는 종료 상태가 아니라 resolved_at이 안 채워진다.
	// 감사 로그에서 읽어야만 나오는 값이라 회귀가 쉽게 난다.
	if h.OldestApprovedAt.IsZero() {
		t.Error("OldestApprovedAt이 비었다 — 감사 로그에서 못 읽고 있다")
	}
	if h.TodayClaims != 3 {
		t.Errorf("TodayClaims = %d, want 3", h.TodayClaims)
	}
	// 방금 만든 건들은 아직 하루가 안 됐다.
	if h.StaleOnHold != 0 {
		t.Errorf("StaleOnHold = %d, want 0", h.StaleOnHold)
	}
}

func TestHomeSummaryFor_다른_매장은_섞이지_않는다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	mine, m := fixture(t, s)
	theirs, theirMachine := fixture(t, s)

	if _, err := s.CreateClaim(ctx, claimInput(mine, m.ID, "mine-1")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := s.CreateClaim(ctx, claimInput(theirs, theirMachine.ID, fmt.Sprintf("theirs-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	h, err := s.HomeSummaryFor(ctx, mine, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if h.Pending != 1 {
		t.Errorf("Pending = %d, want 1 — 남의 매장 건이 섞였다", h.Pending)
	}

	alerts, err := s.MachineAlertsFor(ctx, mine, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Errorf("기계 알림 = %+v — 남의 매장 기계가 올라왔다", alerts)
	}
}

func TestMachineAlertsFor_임계치_이상만_돌려준다(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	storeID, m := fixture(t, s)

	for i := 0; i < 3; i++ {
		if _, err := s.CreateClaim(ctx, claimInput(storeID, m.ID, fmt.Sprintf("alert-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	if got, err := s.MachineAlertsFor(ctx, storeID, 4, time.Now()); err != nil || len(got) != 0 {
		t.Fatalf("임계치 4에서 %v (err=%v)", got, err)
	}
	got, err := s.MachineAlertsFor(ctx, storeID, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Count != 3 || got[0].Label != "3번 기계" {
		t.Fatalf("알림 = %+v", got)
	}
}
