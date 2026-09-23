package domain

import (
	"fmt"
	"strings"
)

// ── 사업자등록번호 ──────────────────────────────────────────────────────

// bizNoWeights는 사업자등록번호 검증에 쓰는 자릿수별 가중치다.
var bizNoWeights = [9]int{1, 3, 7, 1, 3, 7, 1, 3, 5}

// NormalizeBizNo는 하이픈과 공백을 걷어낸 10자리만 남긴다.
func NormalizeBizNo(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatBizNo는 000-00-00000 형태로 만든다.
// 10자리가 아니면 손대지 않는다 — 억지로 자르면 화면에 없는 번호가 뜬다.
func FormatBizNo(raw string) string {
	n := NormalizeBizNo(raw)
	if len(n) != 10 {
		return raw
	}
	return n[:3] + "-" + n[3:5] + "-" + n[5:]
}

// ValidateBizNo는 사업자등록번호의 검증 규칙을 확인한다.
//
// 이건 "이 번호가 실제로 존재하는가"를 확인하지 않는다. 그건 국세청
// 상태조회 API가 해야 하고, 그 API는 활용신청과 승인이 필요하다.
// 여기서 잡는 건 오타와 아무렇게나 적은 숫자다 — 그것만으로도 장터에
// 들어오는 쓰레기의 대부분이 걸러진다.
//
// 그 한계를 화면에서 숨기지 않는다. "확인했습니다"가 아니라
// "형식이 맞습니다"라고 말해야 한다.
func ValidateBizNo(raw string) error {
	n := NormalizeBizNo(raw)
	if n == "" {
		return fmt.Errorf("사업자등록번호를 입력해주세요")
	}
	if len(n) != 10 {
		return fmt.Errorf("사업자등록번호는 숫자 10자리입니다")
	}

	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(n[i]-'0') * bizNoWeights[i]
	}
	// 아홉 번째 자리는 가중치를 곱한 값의 십의 자리를 한 번 더 더한다.
	sum += int(n[8]-'0') * 5 / 10

	want := (10 - sum%10) % 10
	if int(n[9]-'0') != want {
		return fmt.Errorf("사업자등록번호를 다시 확인해주세요")
	}
	return nil
}

// ── 거래 방식 ──────────────────────────────────────────────────────────

// ListingKind는 글을 올린 사장님이 원하는 거래 방식이다.
type ListingKind string

const (
	ListingSell ListingKind = "sell"
	ListingSwap ListingKind = "swap"
	ListingBoth ListingKind = "both"
)

func AllListingKinds() []ListingKind {
	return []ListingKind{ListingSell, ListingSwap, ListingBoth}
}

var listingKindLabels = map[ListingKind]string{
	ListingSell: "판매",
	ListingSwap: "교환",
	ListingBoth: "판매·교환",
}

func (k ListingKind) Label() string { return listingKindLabels[k] }

// NeedsPrice는 가격을 받아야 하는지 알려준다.
//
// 교환만 원하는 글에는 가격 칸이 아예 없다. 0원으로 두면 "공짜로 준다"와
// 구분이 안 되고, 목록에서 0원짜리 글이 제일 싼 것처럼 위로 올라온다.
func (k ListingKind) NeedsPrice() bool { return k != ListingSwap }

func ParseListingKind(s string) (ListingKind, error) {
	for _, v := range AllListingKinds() {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("거래 방식을 선택해주세요")
}

// ── 거래글 ─────────────────────────────────────────────────────────────

const (
	maxListingNoteRunes = 300
	// ListingDays는 글이 목록에 남아 있는 기간이다.
	// 죽은 글이 쌓인 장터는 안 여는 장터가 된다.
	ListingDays = 30
	// ListingReportLimit는 이만큼 신고되면 글이 자동으로 내려간다.
	ListingReportLimit = 3
)

// ListingInput은 장터 글 하나의 입력값이다.
type ListingInput struct {
	Name         string
	Kind         ListingKind
	Qty          int
	UnitPriceKRW int
	Note         string
}

// ValidateListing은 장터 글을 검증한다.
//
// 보유 수량보다 많이 올리는 건 막지 않는다. 곧 들어올 물량을 미리 올리는
// 건 정상이고, 막으면 실제로 쓸 수 있는 쓰임 하나가 사라진다. 대신 화면에서
// 보유 수량을 같이 보여줘 사장님이 스스로 알게 한다.
func ValidateListing(in ListingInput) error {
	if _, err := ParseListingKind(string(in.Kind)); err != nil {
		return err
	}
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("인형 이름을 입력해주세요")
	}
	if len([]rune(in.Name)) > maxDollNameRunes {
		return fmt.Errorf("인형 이름이 너무 깁니다 (%d자 이내)", maxDollNameRunes)
	}
	if in.Qty <= 0 {
		return fmt.Errorf("내놓을 수량을 1개 이상 입력해주세요")
	}
	if in.UnitPriceKRW < 0 {
		return fmt.Errorf("희망 단가는 0원 이상이어야 합니다")
	}
	if in.Kind.NeedsPrice() && in.UnitPriceKRW <= 0 {
		return fmt.Errorf("희망 단가를 입력해주세요. 교환만 원하시면 거래 방식을 바꿔주세요")
	}
	if len([]rune(in.Note)) > maxListingNoteRunes {
		return fmt.Errorf("한마디가 너무 깁니다 (%d자 이내)", maxListingNoteRunes)
	}
	return nil
}
