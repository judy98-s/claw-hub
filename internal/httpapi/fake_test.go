package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

// fakeStore는 Postgres 없이 핸들러를 테스트하기 위한 저장소다.
type fakeStore struct {
	mu sync.Mutex

	machines map[string]domain.Machine // code -> machine
	info     store.StoreInfo

	claims    map[string]store.ClaimDetail // id -> detail
	byIdemKey map[string]string            // storeID|key -> claimID
	nextID    int

	purchases     map[string]store.Purchase // id -> purchase
	purchaseStore map[string]string         // id -> storeID
	purchaseByKey map[string]string         // storeID|idemKey -> id
	purchaseOrder []string                  // 등록 순서
	lastCount     map[string]countedAt      // storeID|nameKey -> 마지막 실사
	boughtSince   map[string]int            // storeID|nameKey -> 실사 뒤 입고
	adjustments   map[string][]store.Adjustment

	facts domain.RiskInput

	user        store.User
	users       []store.User
	storeDetail store.StoreDetail

	// 주입 가능한 실패
	createErr error
	riskErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		machines:      map[string]domain.Machine{},
		claims:        map[string]store.ClaimDetail{},
		byIdemKey:     map[string]string{},
		purchases:     map[string]store.Purchase{},
		purchaseStore: map[string]string{},
		purchaseByKey: map[string]string{},
		lastCount:     map[string]countedAt{},
		boughtSince:   map[string]int{},
		adjustments:   map[string][]store.Adjustment{},
		info:          store.StoreInfo{Name: "테스트 매장", Phone: "0212345678"},
		facts:         domain.RiskInput{PhoneClaims30d: 1, AccountDistinctPhones: 1, ManualStatus: domain.ContactNormal},
		user:          owner,
		users:         []store.User{owner},
		storeDetail:   store.StoreDetail{ID: "store-1", Name: "테스트 매장", Phone: "0212345678"},
	}
}

// countedAt은 fake 가 기억하는 마지막 실사다.
type countedAt struct{ qty int }

// owner는 기본 로그인 계정이다.
var owner = store.User{
	ID: "user-1", StoreID: "store-1", Email: "owner@example.com",
	Name: "사장님", Phone: "01011112222", Active: true,
}

func (f *fakeStore) addMachine(code, label string) domain.Machine {
	m := domain.Machine{ID: "machine-" + code, StoreID: "store-1", Code: code, Label: label, Active: true}
	f.machines[code] = m
	return m
}

func (f *fakeStore) claimCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.claims)
}

func (f *fakeStore) MachineByCode(_ context.Context, code string) (domain.Machine, store.StoreInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.machines[code]
	if !ok || !m.Active {
		return domain.Machine{}, f.info, store.ErrNotFound
	}
	return m, f.info, nil
}

func (f *fakeStore) CreateClaim(_ context.Context, in store.CreateClaimInput) (store.CreateClaimResult, error) {
	if f.createErr != nil {
		return store.CreateClaimResult{}, f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	key := in.StoreID + "|" + in.IdempotencyKey
	if id, ok := f.byIdemKey[key]; ok {
		c := f.claims[id]
		return store.CreateClaimResult{ID: id, Status: c.Status, CreatedAt: c.CreatedAt, Existing: true}, nil
	}

	f.nextID++
	id := fmt.Sprintf("claim-%08d", f.nextID)
	photos := make([]store.Photo, 0, len(in.Photos))
	for i, p := range in.Photos {
		photos = append(photos, store.Photo{
			ID: fmt.Sprintf("%s-photo-%d", id, i), ClaimID: id,
			ObjectKey: p.ObjectKey, ContentType: p.ContentType, SizeBytes: p.SizeBytes,
		})
	}
	now := time.Now()
	f.claims[id] = store.ClaimDetail{
		Claim: domain.Claim{
			ID: id, StoreID: in.StoreID, MachineID: in.MachineID,
			IssueType: in.IssueType, AmountKRW: in.AmountKRW, Description: in.Description,
			PaymentMethod: in.PaymentMethod, CardLast4: in.CardLast4, PaidAtGuess: in.PaidAtGuess,
			Status: in.Status, RiskScore: in.RiskScore, CreatedAt: now,
		},
		MachineLabel: "3번 기계", MachineCode: "ABCD23",
		RiskReasons: in.RiskReasons,
		Phone:       in.Phone, BankCode: in.BankCode, Account: in.Account, Holder: in.Holder,
		Photos: photos,
	}
	f.byIdemKey[key] = id
	return store.CreateClaimResult{ID: id, Status: in.Status, CreatedAt: now}, nil
}

func (f *fakeStore) ClaimByIdempotencyKey(_ context.Context, storeID, key string) (store.CreateClaimResult, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byIdemKey[storeID+"|"+key]
	if !ok {
		return store.CreateClaimResult{}, false, nil
	}
	c := f.claims[id]
	return store.CreateClaimResult{ID: id, Status: c.Status, CreatedAt: c.CreatedAt, Existing: true}, true, nil
}

func (f *fakeStore) RiskFactsFor(_ context.Context, q store.RiskQuery) (domain.RiskInput, error) {
	if f.riskErr != nil {
		return domain.RiskInput{}, f.riskErr
	}
	out := f.facts
	out.Now = q.Now
	// 진짜 저장소와 같은 규칙이어야 한다. 카드 건은 계좌가 없다.
	out.HasAccount = q.Account != ""
	return out, nil
}

func (f *fakeStore) Authenticate(_ context.Context, email, password string) (store.User, error) {
	if email == f.user.Email && password == "secret123" {
		return f.user, nil
	}
	if email == "locked@example.com" {
		return store.User{}, store.ErrLocked
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeStore) UserByID(_ context.Context, id string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.ID == id && u.Active {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeStore) ListClaims(_ context.Context, fl store.ClaimFilter) ([]store.ClaimSummary, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ClaimSummary{}
	for _, c := range f.claims {
		if c.StoreID != fl.StoreID {
			continue
		}
		if len(fl.Statuses) > 0 {
			match := false
			for _, st := range fl.Statuses {
				if c.Status == st {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, store.ClaimSummary{
			ID: c.ID, MachineLabel: c.MachineLabel, IssueType: c.IssueType,
			AmountKRW: c.AmountKRW, Status: c.Status, PhoneMasked: "010-****-5678",
		})
	}
	return out, "", nil
}

func (f *fakeStore) ClaimByID(_ context.Context, storeID, id string, _ domain.Actor) (store.ClaimDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.claims[id]
	if !ok || c.StoreID != storeID {
		return store.ClaimDetail{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeStore) ApplyTransition(_ context.Context, storeID string, c *domain.Claim, ev domain.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.claims[c.ID]
	if !ok || cur.StoreID != storeID {
		return store.ErrNotFound
	}
	if cur.Status != ev.From {
		return store.ErrNotFound // 그 사이 남이 바꿨다
	}
	cur.Claim = *c
	// 진짜 저장소는 상태 변경과 감사 이벤트를 한 트랜잭션에 쓴다.
	// fake가 이벤트를 빼먹으면 "누가 승인했나", "언제 승인됐나"를
	// 읽는 코드가 테스트에서만 빈손이 된다.
	cur.Events = append(cur.Events, ev)
	f.claims[c.ID] = cur
	return nil
}

func (f *fakeStore) ListMachines(context.Context, string) ([]store.MachineStat, error) {
	out := []store.MachineStat{}
	for _, m := range f.machines {
		out = append(out, store.MachineStat{Machine: m, Daily7d: make([]int, 7)})
	}
	return out, nil
}

func (f *fakeStore) MachineByID(_ context.Context, storeID, id string) (domain.Machine, error) {
	for _, m := range f.machines {
		if m.ID == id && m.StoreID == storeID {
			return m, nil
		}
	}
	return domain.Machine{}, store.ErrNotFound
}

func (f *fakeStore) CreateMachine(_ context.Context, storeID, label, location string) (domain.Machine, error) {
	code, _ := domain.GenerateMachineCode()
	m := domain.Machine{ID: "machine-" + code, StoreID: storeID, Code: code,
		Label: label, Location: location, Active: true}
	f.machines[code] = m
	return m, nil
}

func (f *fakeStore) UpdateMachine(_ context.Context, storeID, id, label, location string, active bool) error {
	for code, m := range f.machines {
		if m.ID == id && m.StoreID == storeID {
			m.Label, m.Location, m.Active = label, location, active
			f.machines[code] = m
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) UpdateUser(_ context.Context, storeID, userID, name, phone string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, u := range f.users {
		if u.ID == userID && u.StoreID == storeID {
			f.users[i].Name, f.users[i].Phone = name, phone
			if u.ID == f.user.ID {
				f.user.Name, f.user.Phone = name, phone
			}
			return f.users[i], nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeStore) ListUsers(_ context.Context, storeID string) ([]store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.User{}
	for _, u := range f.users {
		if u.StoreID == storeID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeStore) CreateUser(_ context.Context, storeID, email, password, name, phone string) (store.User, error) {
	if len(password) < 8 {
		return store.User{}, errors.New("비밀번호는 8자 이상이어야 합니다")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Email == email {
			return store.User{}, errors.New("이미 등록된 이메일입니다")
		}
	}
	u := store.User{
		ID: fmt.Sprintf("user-%d", len(f.users)+1), StoreID: storeID,
		Email: email, Name: name, Phone: phone, Active: true,
	}
	f.users = append(f.users, u)
	return u, nil
}

func (f *fakeStore) SetUserActive(_ context.Context, storeID, userID string, active bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !active {
		remaining := 0
		for _, u := range f.users {
			if u.StoreID == storeID && u.Active && u.ID != userID {
				remaining++
			}
		}
		if remaining == 0 {
			return errors.New("마지막 계정은 비활성화할 수 없습니다")
		}
	}
	for i, u := range f.users {
		if u.ID == userID && u.StoreID == storeID {
			f.users[i].Active = active
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) StoreByID(_ context.Context, id string) (store.StoreDetail, error) {
	if id != "store-1" {
		return store.StoreDetail{}, store.ErrNotFound
	}
	return f.storeDetail, nil
}

func (f *fakeStore) UpdateStore(_ context.Context, id, name, phone string) (store.StoreDetail, error) {
	if id != "store-1" {
		return store.StoreDetail{}, store.ErrNotFound
	}
	f.storeDetail.Name, f.storeDetail.Phone = name, phone
	return f.storeDetail, nil
}

func (f *fakeStore) UpdatePayoutSettings(_ context.Context, id string, in store.PayoutSettings) (store.StoreDetail, error) {
	if id != "store-1" {
		return store.StoreDetail{}, store.ErrNotFound
	}
	f.storeDetail.PayoutProvider = in.Provider
	f.storeDetail.PayoutTemplate = in.Template
	f.storeDetail.PayoutBankCode = in.BankCode
	f.storeDetail.PayoutAccount = in.Account
	return f.storeDetail, nil
}

func (f *fakeStore) ListContacts(context.Context, string) ([]store.ContactSummary, error) {
	return []store.ContactSummary{}, nil
}

func (f *fakeStore) SetContactStatus(context.Context, string, []byte, string, string, string, string) error {
	return nil
}

func (f *fakeStore) DailyStats(context.Context, string, time.Time, time.Time) ([]store.DailyStat, error) {
	return []store.DailyStat{}, nil
}

// fakeMedia는 메모리에 사진을 담는다. 남아 있는 키를 셀 수 있다.
type fakeMedia struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
}

func newFakeMedia() *fakeMedia { return &fakeMedia{objects: map[string][]byte{}} }

func (m *fakeMedia) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

func (m *fakeMedia) Put(_ context.Context, key string, r io.Reader, _ string, _ int64) error {
	if m.putErr != nil {
		return m.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = b
	return nil
}

func (m *fakeMedia) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, "", errors.New("없음")
	}
	return io.NopCloser(bytes.NewReader(b)), "image/jpeg", nil
}

func (m *fakeMedia) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// fakeNotifier는 보낸 알림을 기록한다.
type fakeNotifier struct {
	mu      sync.Mutex
	claims  []notify.ClaimNotice
	sendErr error
}

func (n *fakeNotifier) ClaimCreated(_ context.Context, c notify.ClaimNotice) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.claims = append(n.claims, c)
	return n.sendErr
}

func (n *fakeNotifier) MachineAlert(context.Context, notify.MachineNotice) error { return n.sendErr }
func (n *fakeNotifier) DailyDigest(context.Context, notify.DigestNotice) error   { return n.sendErr }

func (n *fakeNotifier) sent() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.claims)
}

// failingCache는 모든 호출에 실패한다. Redis 장애를 흉내낸다.
type failingCache struct{}

func (failingCache) Incr(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("redis 연결 실패")
}
func (failingCache) SetNX(context.Context, string, string, time.Duration) (bool, error) {
	return false, errors.New("redis 연결 실패")
}
func (failingCache) Get(context.Context, string) (string, bool, error) {
	return "", false, errors.New("redis 연결 실패")
}
func (failingCache) Close() error { return nil }

// newEmptyPayout은 딥링크 템플릿이 하나도 없는 제공자를 만든다.
func newEmptyPayout() payout.Payout { return payout.NewDeeplink(nil) }

// HomeSummaryFor는 진짜 저장소와 같은 규칙으로 fake의 claims를 센다.
// 숫자를 고정값으로 돌려주면 홈 화면 테스트가 아무것도 검증하지 못한다.
func (f *fakeStore) HomeSummaryFor(_ context.Context, storeID string, now time.Time) (store.HomeSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var h store.HomeSummary
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, c := range f.claims {
		if c.StoreID != storeID {
			continue
		}
		switch c.Status {
		case domain.StatusPending:
			h.Pending++
		case domain.StatusNeedsReview:
			h.NeedsReview++
		case domain.StatusOnHold:
			h.OnHold++
			if c.CreatedAt.Before(now.Add(-24 * time.Hour)) {
				h.StaleOnHold++
			}
		case domain.StatusApproved:
			h.Approved++
			h.ApprovedAmountKRW += c.AmountKRW
			at := approvedAt(c)
			if !at.IsZero() && (h.OldestApprovedAt.IsZero() || at.Before(h.OldestApprovedAt)) {
				h.OldestApprovedAt = at
			}
		case domain.StatusPaid:
			if !c.PaidAt.Before(dayStart) {
				h.TodayPaid++
				h.TodayPaidKRW += c.PaidAmountKRW
			}
		}
		if !c.CreatedAt.Before(dayStart) {
			h.TodayClaims++
		}
	}
	return h, nil
}

func approvedAt(c store.ClaimDetail) time.Time {
	for _, e := range c.Events {
		if e.To == domain.StatusApproved {
			return e.At
		}
	}
	return time.Time{}
}

func (f *fakeStore) MachineAlertsFor(_ context.Context, storeID string, threshold int, now time.Time) ([]store.MachineAlert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := map[string]int{}
	label := map[string]string{}
	for _, c := range f.claims {
		if c.StoreID != storeID || c.CreatedAt.Before(now.Add(-24*time.Hour)) {
			continue
		}
		count[c.MachineID]++
		label[c.MachineID] = c.MachineLabel
	}

	out := []store.MachineAlert{}
	for id, n := range count {
		if n >= threshold {
			out = append(out, store.MachineAlert{MachineID: id, Label: label[id], Count: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

// ── 재고 장부 ──────────────────────────────────────────────────────────
//
// 진짜 저장소와 같은 규칙으로 계산한다. 고정값을 돌려주면 핸들러 테스트가
// 아무것도 검증하지 못한다.

func (f *fakeStore) CreatePurchase(_ context.Context, in store.CreatePurchaseInput) (store.Purchase, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := in.StoreID + "|" + in.IdempotencyKey
	if id, ok := f.purchaseByKey[key]; ok {
		return f.purchases[id], true, nil
	}

	f.nextID++
	p := store.Purchase{
		ID:   fmt.Sprintf("purchase-%08d", f.nextID),
		Name: strings.TrimSpace(in.Name), NameKey: domain.NameKey(in.Name),
		Vendor: strings.TrimSpace(in.Vendor), QtyTotal: in.Qty,
		UnitPriceKRW: in.UnitPriceKRW, ShippingKRW: in.ShippingKRW,
		UnitCostKRW: domain.UnitCostKRW(in.UnitPriceKRW, in.Qty, in.ShippingKRW),
		TotalKRW:    domain.TotalCostKRW(in.UnitPriceKRW, in.Qty, in.ShippingKRW),
		PurchasedAt: in.PurchasedAt, ReceiptKey: in.ReceiptKey,
		HasReceipt: in.ReceiptKey != "",
	}
	f.purchases[p.ID] = p
	f.purchaseStore[p.ID] = in.StoreID
	// 실사 뒤에 들어온 입고는 따로 센다. 세어본 다음에 산 것은 더해져야 한다.
	if _, counted := f.lastCount[in.StoreID+"|"+p.NameKey]; counted {
		f.boughtSince[in.StoreID+"|"+p.NameKey] += p.QtyTotal
	}
	f.purchaseByKey[key] = p.ID
	f.purchaseOrder = append(f.purchaseOrder, p.ID)
	return p, false, nil
}

// storePurchases는 한 매장의 사입을 최신순으로 모은다.
func (f *fakeStore) storePurchases(storeID, nameKey string) []store.Purchase {
	out := []store.Purchase{}
	for i := len(f.purchaseOrder) - 1; i >= 0; i-- {
		id := f.purchaseOrder[i]
		if f.purchaseStore[id] != storeID {
			continue
		}
		p := f.purchases[id]
		if nameKey != "" && p.NameKey != nameKey {
			continue
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PurchasedAt.After(out[j].PurchasedAt) })
	return out
}

func (f *fakeStore) ListPurchases(_ context.Context, storeID, nameKey string, limit int) ([]store.Purchase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.storePurchases(storeID, nameKey)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) inventoryRows(storeID string) []store.InventoryRow {
	byKey := map[string]*store.InventoryRow{}
	spent := map[string]int{}
	order := []string{}

	for _, p := range f.storePurchases(storeID, "") {
		r, ok := byKey[p.NameKey]
		if !ok {
			r = &store.InventoryRow{
				NameKey: p.NameKey, Name: p.Name,
				LastVendor: p.Vendor, LastUnitCostKRW: p.UnitCostKRW,
				LastPurchasedAt: p.PurchasedAt,
			}
			byKey[p.NameKey] = r
			order = append(order, p.NameKey)
		}
		r.QtyBought += p.QtyTotal
		r.PurchaseCount++
		spent[p.NameKey] += p.TotalKRW
	}

	out := []store.InventoryRow{}
	for _, k := range order {
		r := byKey[k]
		// 진짜 저장소와 같은 모델: 마지막 실사값 + 그 뒤 입고.
		if c, counted := f.lastCount[storeID+"|"+k]; counted {
			r.QtyOnHand = c.qty + f.boughtSince[storeID+"|"+k]
		} else {
			r.QtyOnHand = r.QtyBought
		}
		if r.QtyBought > 0 {
			r.AvgUnitCostKRW = (spent[k] + r.QtyBought/2) / r.QtyBought
			if r.QtyOnHand > 0 {
				// 진짜 저장소와 같은 식. 반올림된 평균을 곱하지 않는다.
				r.ValueKRW = (spent[k]*r.QtyOnHand + r.QtyBought/2) / r.QtyBought
			}
		}
		out = append(out, *r)
	}
	return out
}

func (f *fakeStore) InventoryFor(_ context.Context, storeID string) ([]store.InventoryRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inventoryRows(storeID), nil
}

func (f *fakeStore) InventoryItem(_ context.Context, storeID, nameKey string) (store.InventoryRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.inventoryRows(storeID) {
		if r.NameKey == nameKey {
			return r, nil
		}
	}
	return store.InventoryRow{}, store.ErrNotFound
}

func (f *fakeStore) InventorySummaryFor(_ context.Context, storeID string, from, to time.Time) (store.InventorySummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var sum store.InventorySummary
	for _, p := range f.storePurchases(storeID, "") {
		if !p.PurchasedAt.Before(from) && p.PurchasedAt.Before(to) {
			sum.SpentKRW += p.TotalKRW
		}
	}
	rows := f.inventoryRows(storeID)
	sum.ItemCount = len(rows)
	for _, r := range rows {
		if r.QtyOnHand > 0 {
			sum.QtyOnHand += r.QtyOnHand
			sum.ValueKRW += r.ValueKRW
		}
	}
	return sum, nil
}

func (f *fakeStore) AdjustInventory(ctx context.Context, storeID, userID, nameKey string, countedQty int, note string) error {
	cur, err := f.InventoryItem(ctx, storeID, nameKey)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delta := countedQty - cur.QtyOnHand
	k := storeID + "|" + nameKey
	f.lastCount[k] = countedAt{qty: countedQty}
	f.boughtSince[k] = 0
	f.adjustments[storeID+"|"+nameKey] = append(
		[]store.Adjustment{{Delta: delta, CountedQty: countedQty, Note: note, At: time.Now()}},
		f.adjustments[storeID+"|"+nameKey]...)
	return nil
}

func (f *fakeStore) AdjustmentsFor(_ context.Context, storeID, nameKey string) ([]store.Adjustment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.adjustments[storeID+"|"+nameKey]
	if out == nil {
		out = []store.Adjustment{}
	}
	return out, nil
}

func (f *fakeStore) DollNameSuggestions(_ context.Context, storeID, q string, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := domain.NameKey(q)
	seen := map[string]bool{}
	out := []string{}
	for _, p := range f.storePurchases(storeID, "") {
		if seen[p.NameKey] || (key != "" && !strings.Contains(p.NameKey, key)) {
			continue
		}
		seen[p.NameKey] = true
		out = append(out, p.Name)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) VendorSuggestions(_ context.Context, storeID string, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	seen := map[string]bool{}
	out := []string{}
	for _, p := range f.storePurchases(storeID, "") {
		if p.Vendor == "" || seen[p.Vendor] {
			continue
		}
		seen[p.Vendor] = true
		out = append(out, p.Vendor)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
