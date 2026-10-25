package domain

// StoreType은 매장 업종이다.
//
// 손님이 보는 것 거의 전부가 이 값으로 갈린다 — 캐릭터, 증상 목록,
// 빠른 금액 버튼, QR 스티커 문구, 기계를 부르는 말("7번 기계" / "3번 세탁기").
// 그래서 매장을 만들 때 반드시 정하고, 나중에 설정에서 바꿀 수 있다.
//
// 코드가 아니라 짧은 영문 키를 쓰는 이유는 지역 코드와 같다. 업종 분류
// 코드는 기관이 바꾸고, 바뀌면 저장된 값이 통째로 틀어진다.
type StoreType string

const (
	StoreClaw    StoreType = "claw"    // 인형뽑기
	StoreLaundry StoreType = "laundry" // 무인세탁
	StoreKaraoke StoreType = "karaoke" // 코인노래방
	StorePhoto   StoreType = "photo"   // 무인사진관
	StoreStudy   StoreType = "study"   // 스터디카페
	StoreCafe    StoreType = "cafe"    // 무인카페
)

// storeTypeLabels는 선언 순서를 유지한다. 화면의 타일 순서가 이 순서다.
var storeTypeLabels = []struct {
	Type  StoreType
	Label string
}{
	{StoreClaw, "인형뽑기"},
	{StoreLaundry, "무인세탁"},
	{StoreKaraoke, "코인노래방"},
	{StorePhoto, "무인사진관"},
	{StoreStudy, "스터디카페"},
	{StoreCafe, "무인카페"},
}

// StoreTypes는 고를 수 있는 업종을 화면 순서대로 반환한다.
func StoreTypes() []struct {
	Type  StoreType
	Label string
} {
	out := make([]struct {
		Type  StoreType
		Label string
	}, len(storeTypeLabels))
	copy(out, storeTypeLabels)
	return out
}

// ValidStoreType은 아는 업종인지 본다.
func ValidStoreType(s string) bool {
	for _, t := range storeTypeLabels {
		if string(t.Type) == s {
			return true
		}
	}
	return false
}

// StoreTypeLabel은 화면에 쓸 이름이다. 모르는 값이면 빈 문자열이다.
func StoreTypeLabel(s StoreType) string {
	for _, t := range storeTypeLabels {
		if t.Type == s {
			return t.Label
		}
	}
	return ""
}

// NormalizeStoreType은 모르는 값을 인형뽑기로 떨어뜨린다.
//
// 화면이 깨지는 것보다 낫다. 이 프로젝트가 인형뽑기에서 시작했으므로
// 기본값도 그쪽이고, 마이그레이션의 DEFAULT 와 같은 값이다.
func NormalizeStoreType(s string) StoreType {
	if ValidStoreType(s) {
		return StoreType(s)
	}
	return StoreClaw
}
