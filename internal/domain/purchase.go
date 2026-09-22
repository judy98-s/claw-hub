package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// 사입 기록의 한계값. 화면과 DB 양쪽이 이 값을 따른다.
const (
	maxDollNameRunes = 80
	maxVendorRunes   = 80
	// 사입일은 5년 전까지만 받는다. 그보다 오래된 건 장부에 넣을 이유가 없고,
	// datetime 입력에서 연도를 잘못 친 결과일 가능성이 훨씬 높다.
	maxPurchaseAge = 5 * 365 * 24 * time.Hour
	// 매장과 서버의 시계가 몇 시간 어긋나는 건 흔하다. 하루까지는 봐준다.
	// 이 여유가 없으면 사장님이 방금 산 물건을 "미래"라며 못 적는다.
	clockSkewAllowance = 24 * time.Hour
)

// UnitCostKRW는 배송비를 얹은 개당 실단가다.
//
// 이 계산을 한 곳에 가두는 이유: 나눗셈이 들어간다. 화면과 서버가 각자
// 나누면 1원씩 어긋나고, 그 1원을 사장님이 먼저 본다.
//
// 원 단위로 반올림한다. 버림을 쓰면 개당 단가가 실제보다 싸 보이고,
// 단가를 싸 보이게 만드는 쪽으로 틀리는 건 장부에서 가장 나쁜 방향이다.
func UnitCostKRW(unitPriceKRW, qty, shippingKRW int) int {
	if qty <= 0 {
		return 0
	}
	total := TotalCostKRW(unitPriceKRW, qty, shippingKRW)
	return (total + qty/2) / qty
}

// TotalCostKRW는 실제 결제액이다.
//
// UnitCostKRW × 수량과 같지 않다. 반올림이 들어갔기 때문이다. 합계를
// 보여줄 때는 반드시 이쪽을 쓴다 — 개당 단가를 곱하면 영수증과 몇 원 어긋난다.
func TotalCostKRW(unitPriceKRW, qty, shippingKRW int) int {
	return unitPriceKRW*qty + shippingKRW
}

// NameKey는 같은 인형을 한 줄로 묶기 위한 비교용 키다.
//
// 공백을 "접는" 게 아니라 아예 지운다. 한국어에서 띄어쓰기는 사람마다
// 다르고, 같은 사장님도 그날그날 다르게 적는다. "쿠로미 중형"과
// "쿠로미중형"이 다른 품목으로 갈리면 장부가 두 줄이 되고, 사장님은
// 둘 중 어느 쪽이 진짜 재고인지 알 수 없게 된다.
//
// 대신 "중형"과 "대형"처럼 글자가 다르면 그대로 갈린다. 그건 실제로
// 다른 물건이다.
func NameKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// PurchaseInput은 사입 한 건의 입력값이다.
type PurchaseInput struct {
	Name         string
	Vendor       string
	UnitPriceKRW int
	Qty          int
	ShippingKRW  int
	PurchasedAt  time.Time
}

// ValidatePurchase는 사입 입력을 검증한다.
// 에러 메시지는 사장님이 그대로 읽는 문장이다 — 필드명을 넣지 않는다.
func ValidatePurchase(in PurchaseInput, now time.Time) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("인형 이름을 입력해주세요")
	}
	if len([]rune(in.Name)) > maxDollNameRunes {
		return fmt.Errorf("인형 이름이 너무 깁니다 (%d자 이내)", maxDollNameRunes)
	}
	if len([]rune(in.Vendor)) > maxVendorRunes {
		return fmt.Errorf("거래처 이름이 너무 깁니다 (%d자 이내)", maxVendorRunes)
	}
	if in.Qty <= 0 {
		return fmt.Errorf("수량을 1개 이상 입력해주세요")
	}
	if in.UnitPriceKRW < 0 {
		return fmt.Errorf("단가는 0원 이상이어야 합니다")
	}
	if in.ShippingKRW < 0 {
		return fmt.Errorf("배송비는 0원 이상이어야 합니다")
	}
	if in.PurchasedAt.IsZero() {
		return fmt.Errorf("사입한 날짜를 입력해주세요")
	}
	if in.PurchasedAt.After(now.Add(clockSkewAllowance)) {
		return fmt.Errorf("사입 날짜가 미래로 되어 있습니다")
	}
	if in.PurchasedAt.Before(now.Add(-maxPurchaseAge)) {
		return fmt.Errorf("사입 날짜가 너무 오래됐습니다. 연도를 확인해주세요")
	}
	return nil
}

// ValidateAdjustment는 실사 보정값을 검증한다.
//
// 0은 정상이다 — 다 팔렸다는 뜻이고, 그것도 기록할 가치가 있는 사실이다.
// 음수는 오타다. 세어서 음수가 나올 수는 없다.
func ValidateAdjustment(countedQty int) error {
	if countedQty < 0 {
		return fmt.Errorf("세어본 수량은 0개 이상이어야 합니다")
	}
	return nil
}
