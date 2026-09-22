package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// Purchase는 사입 한 건이다.
type Purchase struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NameKey   string `json:"nameKey"`
	Vendor    string `json:"vendor"`
	QtyTotal  int    `json:"qty"`
	CreatedBy string `json:"createdBy"`

	UnitPriceKRW int `json:"unitPriceKrw"`
	ShippingKRW  int `json:"shippingKrw"`
	// UnitCostKRW는 배송비를 얹은 개당 실단가, TotalKRW는 실제 결제액이다.
	// 둘 다 저장하지 않고 읽을 때 계산한다 — 계산식이 바뀌면 과거 기록도
	// 같이 바뀌어야 맞다.
	UnitCostKRW int `json:"unitCostKrw"`
	TotalKRW    int `json:"totalKrw"`

	PurchasedAt time.Time `json:"purchasedAt"`
	ReceiptKey  string    `json:"-"`
	HasReceipt  bool      `json:"hasReceipt"`
}

// CreatePurchaseInput은 사입 등록 입력이다.
type CreatePurchaseInput struct {
	StoreID string
	UserID  string

	Name         string
	Vendor       string
	UnitPriceKRW int
	Qty          int
	ShippingKRW  int
	PurchasedAt  time.Time
	ReceiptKey   string

	IdempotencyKey string
}

// CreatePurchase는 사입을 기록한다.
//
// 두 번째 반환값이 true면 같은 멱등키의 기존 건을 그대로 돌려준 것이다.
// 제출 버튼을 두 번 누르면 60개가 120개가 되고, 그 오차는 재고를 세어보기
// 전까지 드러나지 않는다. 신고 접수와 같은 규약을 쓴다.
func (s *Store) CreatePurchase(ctx context.Context, in CreatePurchaseInput) (Purchase, bool, error) {
	nameKey := domain.NameKey(in.Name)

	var p Purchase
	err := s.pool.QueryRow(ctx, `
		INSERT INTO purchases (
			store_id, created_by, name, name_key, vendor,
			unit_price_krw, qty, shipping_krw, purchased_at, receipt_key, idempotency_key
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (store_id, idempotency_key) DO NOTHING
		RETURNING id`,
		in.StoreID, nilUUID(in.UserID), strings.TrimSpace(in.Name), nameKey, strings.TrimSpace(in.Vendor),
		in.UnitPriceKRW, in.Qty, in.ShippingKRW, in.PurchasedAt, in.ReceiptKey, in.IdempotencyKey,
	).Scan(&p.ID)

	if errors.Is(err, pgx.ErrNoRows) {
		existing, found, err := s.purchaseByIdempotencyKey(ctx, in.StoreID, in.IdempotencyKey)
		if err != nil {
			return Purchase{}, false, err
		}
		if !found {
			// 여기 오면 유니크 제약이 걸렸는데 행이 안 보인다는 뜻이다.
			// 경합 상대가 아직 커밋 전이다.
			return Purchase{}, false, ErrDuplicate
		}
		return existing, true, nil
	}
	if err != nil {
		return Purchase{}, false, fmt.Errorf("사입 저장: %w", err)
	}

	p.Name, p.NameKey, p.Vendor = strings.TrimSpace(in.Name), nameKey, strings.TrimSpace(in.Vendor)
	p.UnitPriceKRW, p.QtyTotal, p.ShippingKRW = in.UnitPriceKRW, in.Qty, in.ShippingKRW
	p.PurchasedAt, p.ReceiptKey = in.PurchasedAt, in.ReceiptKey
	fillCost(&p)
	return p, false, nil
}

func (s *Store) purchaseByIdempotencyKey(ctx context.Context, storeID, key string) (Purchase, bool, error) {
	rows, err := s.purchaseQuery(ctx,
		`WHERE p.store_id=$1 AND p.idempotency_key=$2`, storeID, key)
	if err != nil {
		return Purchase{}, false, err
	}
	if len(rows) == 0 {
		return Purchase{}, false, nil
	}
	return rows[0], true, nil
}

// ListPurchases는 사입 이력을 최신순으로 반환한다.
// nameKey가 비어 있지 않으면 그 품목만 본다.
func (s *Store) ListPurchases(ctx context.Context, storeID, nameKey string, limit int) ([]Purchase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := `WHERE p.store_id=$1 AND ($2 = '' OR p.name_key = $2)
	          ORDER BY p.purchased_at DESC, p.created_at DESC LIMIT $3`
	return s.purchaseQuery(ctx, where, storeID, nameKey, limit)
}

func (s *Store) purchaseQuery(ctx context.Context, where string, args ...any) ([]Purchase, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.name_key, p.vendor, p.unit_price_krw, p.qty,
		       p.shipping_krw, p.purchased_at, p.receipt_key, COALESCE(u.name, '')
		  FROM purchases p LEFT JOIN users u ON u.id = p.created_by `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("사입 조회: %w", err)
	}
	defer rows.Close()

	out := []Purchase{}
	for rows.Next() {
		var p Purchase
		if err := rows.Scan(&p.ID, &p.Name, &p.NameKey, &p.Vendor, &p.UnitPriceKRW,
			&p.QtyTotal, &p.ShippingKRW, &p.PurchasedAt, &p.ReceiptKey, &p.CreatedBy); err != nil {
			return nil, err
		}
		fillCost(&p)
		out = append(out, p)
	}
	return out, rows.Err()
}

func fillCost(p *Purchase) {
	p.UnitCostKRW = domain.UnitCostKRW(p.UnitPriceKRW, p.QtyTotal, p.ShippingKRW)
	p.TotalKRW = domain.TotalCostKRW(p.UnitPriceKRW, p.QtyTotal, p.ShippingKRW)
	p.HasReceipt = p.ReceiptKey != ""
}

// InventoryRow는 재고 목록 한 줄이다. 한 품목의 현재 상태 전부.
type InventoryRow struct {
	NameKey string `json:"nameKey"`
	Name    string `json:"name"`

	// QtyOnHand는 사입 합계에 조정을 반영한 현재 수량이다.
	QtyOnHand int `json:"qtyOnHand"`
	QtyBought int `json:"qtyBought"`

	// AvgUnitCostKRW는 수량 가중 평균 원가다. 단순 평균이 아니다 —
	// 2,000원 10개와 3,000원 90개의 평균은 2,500이 아니라 2,900이다.
	AvgUnitCostKRW int `json:"avgUnitCostKrw"`
	// ValueKRW는 재고 자산이다.
	//
	// 화면에 보이는 평균 원가(반올림된 값)를 곱하지 않는다. 그렇게 하면
	// 아무것도 안 팔린 상태에서도 "이번 달 인형값"과 "재고 자산"이 몇십 원
	// 어긋나고, 사장님은 둘 중 뭐가 틀렸는지 알 수 없다. 총 지출에서
	// 남은 비율만큼 떼는 방식이라, 안 팔렸으면 지출과 정확히 같다.
	ValueKRW int `json:"valueKrw"`

	LastVendor      string    `json:"lastVendor"`
	LastUnitCostKRW int       `json:"lastUnitCostKrw"`
	LastPurchasedAt time.Time `json:"lastPurchasedAt"`
	PurchaseCount   int       `json:"purchaseCount"`
}

// inventorySelect는 목록과 단건이 같은 식을 쓰게 한다. 두 벌로 나뉘면
// 목록의 숫자와 상세의 숫자가 어긋나는 날이 온다.
const inventorySelect = `
	WITH agg AS (
	  SELECT p.name_key,
	         SUM(p.qty)                                        AS qty_bought,
	         SUM(p.unit_price_krw * p.qty + p.shipping_krw)     AS spent,
	         COUNT(*)                                           AS purchase_count,
	         MAX(p.purchased_at)                                AS last_at
	    FROM purchases p
	   WHERE p.store_id = $1
	   GROUP BY p.name_key
	),
	last AS (
	  SELECT DISTINCT ON (p.name_key)
	         p.name_key, p.name, p.vendor, p.unit_price_krw, p.qty, p.shipping_krw
	    FROM purchases p
	   WHERE p.store_id = $1
	   ORDER BY p.name_key, p.purchased_at DESC, p.created_at DESC
	),
	-- 가장 최근 실사 한 건. 그 이전의 실사는 이미 덮였으므로 보지 않는다.
	counted AS (
	  SELECT DISTINCT ON (name_key) name_key, counted_qty, created_at
	    FROM inventory_adjustments
	   WHERE store_id = $1
	   ORDER BY name_key, created_at DESC, id DESC
	),
	-- 그 실사 이후에 들어온 입고. 세어본 뒤에 산 것은 당연히 더해져야 한다.
	since AS (
	  SELECT p.name_key, SUM(p.qty) AS qty
	    FROM purchases p JOIN counted c ON c.name_key = p.name_key
	   WHERE p.store_id = $1 AND p.created_at > c.created_at
	   GROUP BY p.name_key
	)
	SELECT agg.name_key, last.name,
	       -- 실사가 있으면 "세어본 수 + 그 뒤 입고", 없으면 산 수량 전부.
	       --
	       -- 차이(delta)를 누적하지 않는 이유: 그러면 실사를 기록할 때마다
	       -- 먼저 현재 수량을 읽어야 하고, 두 사람이 동시에 세면 두 번째
	       -- 사람의 차이가 틀린 기준에서 계산된다. 마지막 실사값을 기준으로
	       -- 삼으면 쓰기가 읽기에 의존하지 않아 그 경합이 아예 없다.
	       CASE WHEN counted.name_key IS NULL THEN agg.qty_bought
	            ELSE counted.counted_qty + COALESCE(since.qty, 0) END AS qty_on_hand,
	       agg.qty_bought, agg.spent, agg.purchase_count, agg.last_at,
	       last.vendor, last.unit_price_krw, last.qty, last.shipping_krw
	  FROM agg
	  JOIN last ON last.name_key = agg.name_key
	  LEFT JOIN counted ON counted.name_key = agg.name_key
	  LEFT JOIN since ON since.name_key = agg.name_key`

// InventoryFor는 매장의 재고 목록을 반환한다.
//
// 수량 0인 품목도 돌려준다. 다 팔렸다는 것도 정보이고, 목록에서 사라지면
// "내가 이거 산 적 있었나"를 확인할 길이 없어진다.
func (s *Store) InventoryFor(ctx context.Context, storeID string) ([]InventoryRow, error) {
	rows, err := s.pool.Query(ctx, inventorySelect+`
		 ORDER BY agg.last_at DESC`, storeID)
	if err != nil {
		return nil, fmt.Errorf("재고 조회: %w", err)
	}
	defer rows.Close()
	return scanInventory(rows)
}

// InventoryItem은 한 품목의 재고를 반환한다.
func (s *Store) InventoryItem(ctx context.Context, storeID, nameKey string) (InventoryRow, error) {
	rows, err := s.pool.Query(ctx, inventorySelect+`
		 WHERE agg.name_key = $2`, storeID, nameKey)
	if err != nil {
		return InventoryRow{}, fmt.Errorf("재고 조회: %w", err)
	}
	defer rows.Close()

	out, err := scanInventory(rows)
	if err != nil {
		return InventoryRow{}, err
	}
	if len(out) == 0 {
		return InventoryRow{}, ErrNotFound
	}
	return out[0], nil
}

func scanInventory(rows pgx.Rows) ([]InventoryRow, error) {
	out := []InventoryRow{}
	for rows.Next() {
		var r InventoryRow
		var spent int64
		var lastUnit, lastQty, lastShipping int
		if err := rows.Scan(&r.NameKey, &r.Name, &r.QtyOnHand, &r.QtyBought, &spent,
			&r.PurchaseCount, &r.LastPurchasedAt,
			&r.LastVendor, &lastUnit, &lastQty, &lastShipping); err != nil {
			return nil, err
		}
		if r.QtyBought > 0 {
			// 수량 가중 평균. 총 지출을 총 수량으로 나누면 자연히 가중된다.
			bought := int64(r.QtyBought)
			r.AvgUnitCostKRW = int((spent + bought/2) / bought)
			if r.QtyOnHand > 0 {
				// 반올림된 평균을 곱하지 않고 비율로 뗀다. 곱하면
				// 안 팔린 상태에서도 지출과 자산이 어긋난다.
				onHand := int64(r.QtyOnHand)
				r.ValueKRW = int((spent*onHand + bought/2) / bought)
			}
		}
		r.LastUnitCostKRW = domain.UnitCostKRW(lastUnit, lastQty, lastShipping)
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdjustInventory는 실사 결과를 기록한다.
//
// 세어본 수량 자체가 기준이 된다. 현재 수량은 "마지막 실사값 + 그 뒤 입고"로
// 계산되므로, 이 쓰기는 직전에 읽은 값에 의존하지 않는다. 두 사람이 동시에
// 같은 품목을 세어도 나중 기록이 그대로 진실이 된다.
//
// delta 는 화면에 "장부보다 18개 적었다"를 보여주기 위한 기록일 뿐이다.
// 경합에서 이 값이 조금 낡아도 현재 수량은 틀어지지 않는다.
//
// 행을 지우거나 고치지 않고 쌓기만 한다. 덮어쓰면 "언제 몇 개가 어디로
// 갔는지"가 사라지고, 장부에서 그건 없는 것만 못하다.
func (s *Store) AdjustInventory(ctx context.Context, storeID, userID, nameKey string, countedQty int, note string) error {
	cur, err := s.InventoryItem(ctx, storeID, nameKey)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO inventory_adjustments (store_id, created_by, name_key, delta, counted_qty, note)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		storeID, nilUUID(userID), nameKey, countedQty-cur.QtyOnHand, countedQty, strings.TrimSpace(note))
	if err != nil {
		return fmt.Errorf("재고 조정 기록: %w", err)
	}
	return nil
}

// Adjustment는 실사 기록 한 줄이다.
type Adjustment struct {
	Delta      int       `json:"delta"`
	CountedQty int       `json:"countedQty"`
	Note       string    `json:"note"`
	By         string    `json:"by"`
	At         time.Time `json:"at"`
}

// AdjustmentsFor는 한 품목의 실사 이력을 최신순으로 반환한다.
func (s *Store) AdjustmentsFor(ctx context.Context, storeID, nameKey string) ([]Adjustment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.delta, a.counted_qty, a.note, COALESCE(u.name, ''), a.created_at
		  FROM inventory_adjustments a LEFT JOIN users u ON u.id = a.created_by
		 WHERE a.store_id=$1 AND a.name_key=$2
		 ORDER BY a.created_at DESC LIMIT 20`, storeID, nameKey)
	if err != nil {
		return nil, fmt.Errorf("재고 조정 이력 조회: %w", err)
	}
	defer rows.Close()

	out := []Adjustment{}
	for rows.Next() {
		var a Adjustment
		if err := rows.Scan(&a.Delta, &a.CountedQty, &a.Note, &a.By, &a.At); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DollNameSuggestions는 자동완성 후보를 반환한다.
//
// 같은 매장의 과거 이름만 본다. 전체에서 찾아주면 편하지만, 그건
// "남의 매장이 요즘 뭘 들이는지"를 한 글자 입력으로 흘리는 것이다.
func (s *Store) DollNameSuggestions(ctx context.Context, storeID, q string, limit int) ([]string, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	// 검색도 name_key 로 한다. 사장님이 "쿠로미중형"이라 쳐도
	// "쿠로미 중형"이 나와야 한다 — 그게 이 기능의 존재 이유다.
	key := domain.NameKey(q)

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (p.name_key) p.name
		  FROM purchases p
		 WHERE p.store_id=$1 AND ($2 = '' OR p.name_key LIKE '%' || $2 || '%')
		 ORDER BY p.name_key, p.purchased_at DESC
		 LIMIT $3`, storeID, key, limit)
	if err != nil {
		return nil, fmt.Errorf("인형 이름 조회: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// VendorSuggestions는 최근에 쓴 거래처를 반환한다.
func (s *Store) VendorSuggestions(ctx context.Context, storeID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	rows, err := s.pool.Query(ctx, `
		SELECT vendor FROM purchases
		 WHERE store_id=$1 AND vendor <> ''
		 GROUP BY vendor ORDER BY MAX(purchased_at) DESC LIMIT $2`, storeID, limit)
	if err != nil {
		return nil, fmt.Errorf("거래처 조회: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// InventorySummary는 재고 화면 맨 위의 숫자들이다.
type InventorySummary struct {
	// SpentKRW는 기간 내 인형값 지출이다. 실제 결제액 기준.
	SpentKRW  int `json:"spentKrw"`
	ItemCount int `json:"itemCount"`
	QtyOnHand int `json:"qtyOnHand"`
	ValueKRW  int `json:"valueKrw"`
}

// InventorySummaryFor는 요약 숫자를 만든다.
// from/to는 지출 집계 기간이고, 재고 자산은 시점과 무관하게 현재값이다.
func (s *Store) InventorySummaryFor(ctx context.Context, storeID string, from, to time.Time) (InventorySummary, error) {
	var sum InventorySummary
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(unit_price_krw * qty + shipping_krw), 0)
		  FROM purchases
		 WHERE store_id=$1 AND purchased_at >= $2 AND purchased_at < $3`,
		storeID, from, to,
	).Scan(&sum.SpentKRW); err != nil {
		return InventorySummary{}, fmt.Errorf("지출 집계: %w", err)
	}

	// 재고 자산은 품목마다 평균 원가가 달라서 SQL 한 줄로 못 낸다.
	// 목록과 같은 식을 써야 화면의 두 숫자가 어긋나지 않는다.
	rows, err := s.InventoryFor(ctx, storeID)
	if err != nil {
		return InventorySummary{}, err
	}
	sum.ItemCount = len(rows)
	for _, r := range rows {
		if r.QtyOnHand > 0 {
			sum.QtyOnHand += r.QtyOnHand
			sum.ValueKRW += r.ValueKRW
		}
	}
	return sum, nil
}
