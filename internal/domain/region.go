package domain

import (
	"fmt"
	"strings"
)

// Region은 장터의 지역 필터 단위다. 시·도 17개.
type Region struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// regions는 시·도 전부다.
//
// 코드로 행정표준코드(11, 26…) 대신 슬러그를 쓴다. 행정표준코드는 실제로
// 바뀐다 — 강원과 전북이 특별자치도가 되면서 번호가 옮겨갔다. 우리 DB가
// 그 변경을 따라다닐 이유가 없다.
//
// 시·군·구까지 코드로 나누지 않는다. 250개를 손으로 넣어야 하고, 매장이
// 스무 곳일 때 "강남구" 필터는 늘 0건이다. 시·군·구는 글자로 보여주기만
// 한다 — 거리를 가늠하는 데는 그걸로 충분하다.
var regions = []Region{
	{"seoul", "서울"},
	{"busan", "부산"},
	{"daegu", "대구"},
	{"incheon", "인천"},
	{"gwangju", "광주"},
	{"daejeon", "대전"},
	{"ulsan", "울산"},
	{"sejong", "세종"},
	{"gyeonggi", "경기"},
	{"gangwon", "강원"},
	{"chungbuk", "충북"},
	{"chungnam", "충남"},
	{"jeonbuk", "전북"},
	{"jeonnam", "전남"},
	{"gyeongbuk", "경북"},
	{"gyeongnam", "경남"},
	{"jeju", "제주"},
}

// Regions는 시·도 전부를 선언 순서로 반환한다.
func Regions() []Region {
	out := make([]Region, len(regions))
	copy(out, regions)
	return out
}

// RegionByCode는 코드로 지역을 찾는다.
func RegionByCode(code string) (Region, bool) {
	for _, r := range regions {
		if r.Code == code {
			return r, true
		}
	}
	return Region{}, false
}

const maxRegionDetailRunes = 20

// ValidateRegionDetail은 시·군·구 표기를 검증한다.
//
// 상세 주소는 받지 않는다. 장터에 필요한 건 "가까운가"뿐이고, 상세 주소는
// 받는 순간 지켜야 할 것이 하나 더 는다.
func ValidateRegionDetail(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("시·군·구를 입력해주세요 (예: 강남구)")
	}
	if len([]rune(s)) > maxRegionDetailRunes {
		return fmt.Errorf("시·군·구는 %d자 이내로 적어주세요", maxRegionDetailRunes)
	}
	return nil
}
