package domain

import (
	"strings"
	"testing"
)

func TestValidateBizNo_실제_유효한_번호를_통과시킨다(t *testing.T) {
	// 공개된 실제 사업자등록번호 두 개. 체크섬 규칙을 잘못 구현하면
	// 둘 다 떨어지므로, 규칙 자체가 맞는지 확인하는 기준점이 된다.
	valid := []string{"220-81-62517", "120-81-47521", "2208162517"}
	for _, n := range valid {
		if err := ValidateBizNo(n); err != nil {
			t.Errorf("ValidateBizNo(%q) = %v, 통과해야 한다", n, err)
		}
	}
}

func TestValidateBizNo_한_자리만_틀려도_거부한다(t *testing.T) {
	// 체크섬의 존재 이유다. 오타 한 글자를 잡지 못하면 없는 것과 같다.
	if err := ValidateBizNo("220-81-62518"); err == nil {
		t.Error("마지막 자리가 틀린 번호가 통과했다")
	}
	if err := ValidateBizNo("220-81-62527"); err == nil {
		t.Error("중간 자리가 틀린 번호가 통과했다")
	}
}

func TestValidateBizNo_형식_오류(t *testing.T) {
	bad := []string{"", "  ", "22081625", "22081625170", "220-81-6251a", "abcdefghij"}
	for _, n := range bad {
		if err := ValidateBizNo(n); err == nil {
			t.Errorf("ValidateBizNo(%q) 가 통과했다", n)
		}
	}
}

func TestNormalizeBizNo_그리고_FormatBizNo(t *testing.T) {
	if got := NormalizeBizNo("220-81-62517"); got != "2208162517" {
		t.Errorf("NormalizeBizNo = %q", got)
	}
	if got := FormatBizNo("2208162517"); got != "220-81-62517" {
		t.Errorf("FormatBizNo = %q", got)
	}
	// 형식이 깨진 값은 건드리지 않고 그대로 돌려준다. 억지로 자르면
	// 화면에 "22-08-16251" 같은 없는 번호가 뜬다.
	if got := FormatBizNo("123"); got != "123" {
		t.Errorf("FormatBizNo(짧은 값) = %q", got)
	}
}

func TestParseListingKind(t *testing.T) {
	for _, s := range []string{"sell", "swap", "both"} {
		if _, err := ParseListingKind(s); err != nil {
			t.Errorf("ParseListingKind(%q) = %v", s, err)
		}
	}
	if _, err := ParseListingKind("give"); err == nil {
		t.Error("알 수 없는 거래 방식이 통과했다")
	}
	if ListingSell.Label() != "판매" || ListingSwap.Label() != "교환" || ListingBoth.Label() != "판매·교환" {
		t.Error("표기가 틀렸다")
	}
}

func TestListingKind_교환전용만_가격이_없다(t *testing.T) {
	// 가격 0원과 "가격을 안 적은 것"은 다르다. 교환은 가격 칸 자체가 없고,
	// 판매인데 0원이면 그건 입력 실수다.
	if !ListingSell.NeedsPrice() || !ListingBoth.NeedsPrice() {
		t.Error("판매·둘다는 가격이 필요하다")
	}
	if ListingSwap.NeedsPrice() {
		t.Error("교환에 가격을 요구하고 있다")
	}
}

func baseListing() ListingInput {
	return ListingInput{
		Name: "쿠로미 중형 30cm", Kind: ListingSell,
		Qty: 40, UnitPriceKRW: 1800, Note: "작년 물량이라 태그 있어요. 직거래만",
	}
}

func TestValidateListing(t *testing.T) {
	if err := ValidateListing(baseListing()); err != nil {
		t.Fatalf("정상 입력이 거부됐다: %v", err)
	}

	tests := []struct {
		name string
		mut  func(*ListingInput)
	}{
		{"이름 없음", func(l *ListingInput) { l.Name = "  " }},
		{"이름 너무 김", func(l *ListingInput) { l.Name = strings.Repeat("가", 81) }},
		{"수량 0", func(l *ListingInput) { l.Qty = 0 }},
		{"수량 음수", func(l *ListingInput) { l.Qty = -1 }},
		{"판매인데 0원", func(l *ListingInput) { l.UnitPriceKRW = 0 }},
		{"가격 음수", func(l *ListingInput) { l.UnitPriceKRW = -100 }},
		{"한마디 너무 김", func(l *ListingInput) { l.Note = strings.Repeat("가", 301) }},
		{"알 수 없는 거래 방식", func(l *ListingInput) { l.Kind = "give" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseListing()
			tc.mut(&in)
			if err := ValidateListing(in); err == nil {
				t.Fatal("거부되어야 한다")
			}
		})
	}
}

func TestValidateListing_교환은_가격없이_통과한다(t *testing.T) {
	in := baseListing()
	in.Kind, in.UnitPriceKRW = ListingSwap, 0
	if err := ValidateListing(in); err != nil {
		t.Errorf("교환 글이 거부됐다: %v", err)
	}
}

func TestValidateListing_보유수량보다_많아도_막지_않는다(t *testing.T) {
	// 곧 들어올 물량을 미리 올리는 건 정상이다. 막으면 실제로 쓸 수 있는
	// 쓰임 하나가 사라진다. 대신 화면에서 보유 수량을 같이 보여준다.
	in := baseListing()
	in.Qty = 10000
	if err := ValidateListing(in); err != nil {
		t.Errorf("많은 수량이 거부됐다: %v", err)
	}
}
