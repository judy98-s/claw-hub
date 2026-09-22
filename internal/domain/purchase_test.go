package domain

import (
	"strings"
	"testing"
	"time"
)

func TestUnitCostKRW(t *testing.T) {
	tests := []struct {
		name                     string
		unitPrice, qty, shipping int
		want                     int
	}{
		{"배송비를 수량으로 나눠 얹는다", 2300, 60, 3000, 2350},
		{"나누어떨어지지 않으면 반올림", 2000, 7, 3000, 2429}, // 17,000/7 = 2428.57
		{"배송비가 없으면 단가 그대로", 2000, 3, 0, 2000},
		{"한 개만 사도 같은 식", 2000, 1, 500, 2500},
		{"수량이 0이면 0", 2000, 0, 3000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UnitCostKRW(tc.unitPrice, tc.qty, tc.shipping); got != tc.want {
				t.Errorf("UnitCostKRW(%d,%d,%d) = %d, want %d",
					tc.unitPrice, tc.qty, tc.shipping, got, tc.want)
			}
		})
	}
}

func TestTotalCostKRW_개당단가곱하기수량과_다르다(t *testing.T) {
	// 이걸 모르면 "합계가 3원 더 나온다"는 버그가 난다. 실제 결제액은
	// 반올림 전의 값이다.
	const unit, qty, shipping = 2000, 7, 3000
	total := TotalCostKRW(unit, qty, shipping)
	if total != 17000 {
		t.Fatalf("TotalCostKRW = %d, want 17000", total)
	}
	if rounded := UnitCostKRW(unit, qty, shipping) * qty; rounded == total {
		t.Errorf("개당 단가 × 수량(%d)이 실제 결제액과 같다 — 이 테스트가 의미를 잃었다", rounded)
	}
}

func TestNameKey_같은_인형은_한_줄로_묶인다(t *testing.T) {
	same := [][]string{
		{"쿠로미 중형", "쿠로미  중형", " 쿠로미 중형 ", "쿠로미중형", "쿠로미\t중형"},
		{"KUROMI", "kuromi", "Kuromi"},
	}
	for _, group := range same {
		want := NameKey(group[0])
		for _, n := range group[1:] {
			if got := NameKey(n); got != want {
				t.Errorf("NameKey(%q) = %q, NameKey(%q) = %q — 같아야 한다",
					n, got, group[0], want)
			}
		}
	}
}

func TestNameKey_다른_인형은_묶이지_않는다(t *testing.T) {
	if NameKey("쿠로미 중형") == NameKey("쿠로미 대형") {
		t.Error("중형과 대형이 한 줄로 묶였다")
	}
	if NameKey("") != "" {
		t.Error("빈 이름의 키는 빈 문자열이어야 한다")
	}
}

func basePurchase() PurchaseInput {
	return PurchaseInput{
		Name: "쿠로미 중형 30cm", Vendor: "캐치돌",
		UnitPriceKRW: 2300, Qty: 60, ShippingKRW: 3000,
		PurchasedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
	}
}

func TestValidatePurchase(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	if err := ValidatePurchase(basePurchase(), now); err != nil {
		t.Fatalf("정상 입력이 거부됐다: %v", err)
	}

	tests := []struct {
		name string
		mut  func(*PurchaseInput)
	}{
		{"이름 없음", func(p *PurchaseInput) { p.Name = "   " }},
		{"이름 너무 김", func(p *PurchaseInput) { p.Name = strings.Repeat("가", 81) }},
		{"수량 0", func(p *PurchaseInput) { p.Qty = 0 }},
		{"수량 음수", func(p *PurchaseInput) { p.Qty = -1 }},
		{"단가 음수", func(p *PurchaseInput) { p.UnitPriceKRW = -1 }},
		{"배송비 음수", func(p *PurchaseInput) { p.ShippingKRW = -1 }},
		{"거래처 너무 김", func(p *PurchaseInput) { p.Vendor = strings.Repeat("가", 81) }},
		{"미래 날짜", func(p *PurchaseInput) { p.PurchasedAt = now.AddDate(0, 0, 2) }},
		{"5년보다 오래된 날짜", func(p *PurchaseInput) { p.PurchasedAt = now.AddDate(-6, 0, 0) }},
		{"날짜 없음", func(p *PurchaseInput) { p.PurchasedAt = time.Time{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := basePurchase()
			tc.mut(&in)
			err := ValidatePurchase(in, now)
			if err == nil {
				t.Fatal("거부되어야 한다")
			}
			// 사장님이 읽는 문장이다. 필드명이 그대로 나오면 안 된다.
			if strings.Contains(err.Error(), "KRW") || strings.Contains(err.Error(), "Qty") {
				t.Errorf("에러 메시지에 필드명이 새어 나온다: %v", err)
			}
		})
	}
}

func TestValidatePurchase_오늘_조금_뒤는_허용한다(t *testing.T) {
	// 매장과 서버의 시계가 몇 시간 어긋나는 건 흔하다. 몇 시간 앞선 시각을
	// "미래"라고 거부하면 사장님은 방금 산 물건을 못 적는다.
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	in := basePurchase()
	in.PurchasedAt = now.Add(6 * time.Hour)
	if err := ValidatePurchase(in, now); err != nil {
		t.Errorf("몇 시간 앞선 시각이 거부됐다: %v", err)
	}
}

func TestValidateAdjustment(t *testing.T) {
	// 실사에서 "지금 0개"는 정상이고 "-3개"는 오타다.
	if err := ValidateAdjustment(0); err != nil {
		t.Errorf("0개가 거부됐다: %v", err)
	}
	if err := ValidateAdjustment(42); err != nil {
		t.Errorf("42개가 거부됐다: %v", err)
	}
	if ValidateAdjustment(-1) == nil {
		t.Error("음수가 통과했다")
	}
}
