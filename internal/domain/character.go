package domain

import "fmt"

// CharacterGroup은 IP 묶음이다.
//
// 캐릭터와 따로 두는 이유: 신호의 출처가 다르다. 검색량은 캐릭터 단위로
// 오지만("피카츄"), 영화 개봉은 IP 단위로 온다("포켓몬 극장판"). 개봉
// 소식을 그 IP 아래 캐릭터 전부에 이어붙이려면 이 축이 필요하다.
type CharacterGroup struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var characterGroups = []CharacterGroup{
	{"sanrio", "산리오"},
	{"pokemon", "포켓몬"},
	{"crayon", "짱구"},
	{"korea", "국내 캐릭터"},
	{"kakao", "카카오프렌즈"},
	{"line", "라인프렌즈"},
	{"disney", "디즈니·픽사"},
	{"jpchar", "일본 캐릭터"},
	{"anime", "애니메이션"},
	{"popmart", "팝마트"},
	{"game", "게임"},
	{"etc", "그 외"},
}

func CharacterGroups() []CharacterGroup {
	out := make([]CharacterGroup, len(characterGroups))
	copy(out, characterGroups)
	return out
}

func CharacterGroupByCode(code string) (CharacterGroup, bool) {
	for _, g := range characterGroups {
		if g.Code == code {
			return g, true
		}
	}
	return CharacterGroup{}, false
}

// Character는 트렌드를 감시할 캐릭터 하나다.
type Character struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Group string `json:"group"`

	// Keywords는 네이버 데이터랩에 넘길 검색어다.
	//
	// 복수인 이유: 사람마다 다르게 검색한다. "짱구"로만 물어보면
	// "크레용 신짱"으로 찾는 사람이 통째로 빠진다. 데이터랩은 한 그룹에
	// 최대 20개까지 받고, 그 합을 한 줄로 돌려준다.
	Keywords []string `json:"keywords"`
}

// AnchorCharacterCode는 모든 요청에 끼워 넣는 기준 캐릭터다.
//
// 데이터랩이 주는 값은 **요청 안에서만 유효한 상대 지수**다. 한 번에 5개
// 그룹까지만 물어볼 수 있어서 캐릭터 70개를 보려면 열몇 번 나눠 불러야
// 하는데, 그 결과끼리는 서로 비교가 안 된다 — 각 요청마다 자기 안에서
// 제일 큰 값이 100이기 때문이다.
//
// 그래서 매 요청에 같은 캐릭터를 하나 끼워 넣고, 그 값으로 나눠 정규화한다.
// 이걸 안 하면 "포켓몬 100, 폼폼푸린 100" 같은 순위가 나온다.
//
// 헬로키티를 고른 이유: 50년 된 IP라 급등락이 적고, 검색량이 0으로
// 떨어지는 일이 없다. 기준이 흔들리면 정규화한 값 전부가 흔들린다.
// 실제 데이터를 보고 더 나은 후보가 보이면 바꾸면 된다.
const AnchorCharacterCode = "hello_kitty"

// naverGroupsPerRequest는 데이터랩이 한 번에 받는 검색어 그룹 수다.
const naverGroupsPerRequest = 5

// characters는 인형뽑기 매장에서 실제로 도는 캐릭터들이다.
//
// 이 목록은 **출발점이지 정답이 아니다.** 손님이 무엇을 집어 가는지는
// 사장님이 매일 보고 있고, 나는 안 보고 있다. 빠진 것과 넣을 필요 없는
// 것을 알려주면 고친다.
//
// 2026년 9월 기준으로 확인한 것:
//   - 산리오 캐릭터 대상 2026 순위: 폼폼푸린 1위, 시나모롤 2위,
//     포차코 3위, 쿠로미 4위, 헬로키티 5위
//   - 슈가바니즈가 2026년 라이징. 2025년의 한교동 자리다
var characters = []Character{
	// ── 산리오 ────────────────────────────────────────────────────────
	{"pompompurin", "폼폼푸린", "sanrio", []string{"폼폼푸린", "포무포무푸린"}},
	{"cinnamoroll", "시나모롤", "sanrio", []string{"시나모롤", "시나모롤 인형", "시나몬롤"}},
	{"pochacco", "포차코", "sanrio", []string{"포차코"}},
	{"kuromi", "쿠로미", "sanrio", []string{"쿠로미", "쿠로미 인형"}},
	{"hello_kitty", "헬로키티", "sanrio", []string{"헬로키티", "키티"}},
	{"my_melody", "마이멜로디", "sanrio", []string{"마이멜로디", "마이멜로디 인형", "멜로디"}},
	{"hangyodon", "한교동", "sanrio", []string{"한교동"}},
	{"sugarbunnies", "슈가바니즈", "sanrio", []string{"슈가바니즈", "슈가버니즈"}},
	{"keroppi", "케로케로케로피", "sanrio", []string{"케로케로케로피", "케로피"}},
	{"badtz_maru", "배드바츠마루", "sanrio", []string{"배드바츠마루", "바츠마루"}},
	{"little_twin_stars", "리틀트윈스타", "sanrio", []string{"리틀트윈스타", "키키라라"}},

	// ── 포켓몬 ────────────────────────────────────────────────────────
	{"pikachu", "피카츄", "pokemon", []string{"피카츄", "피카츄 인형"}},
	{"eevee", "이브이", "pokemon", []string{"이브이", "이브이 인형"}},
	{"snorlax", "잠만보", "pokemon", []string{"잠만보", "잠만보 인형"}},
	{"squirtle", "꼬부기", "pokemon", []string{"꼬부기"}},
	{"charmander", "파이리", "pokemon", []string{"파이리"}},
	{"bulbasaur", "이상해씨", "pokemon", []string{"이상해씨"}},

	// ── 짱구 ──────────────────────────────────────────────────────────
	{"shinchan", "짱구", "crayon", []string{"짱구", "크레용 신짱", "짱구 인형"}},
	{"shiro", "흰둥이", "crayon", []string{"흰둥이", "짱구 흰둥이"}},

	// ── 국내 캐릭터 ────────────────────────────────────────────────────
	{"loopy", "잔망루피", "korea", []string{"잔망루피", "루피 인형"}},
	{"choigosim", "최고심", "korea", []string{"최고심"}},
	{"bellygom", "벨리곰", "korea", []string{"벨리곰"}},
	{"manggom", "망그러진곰", "korea", []string{"망그러진곰", "망곰"}},
	{"pororo", "뽀로로", "korea", []string{"뽀로로"}},
	{"cookierun", "쿠키런", "korea", []string{"쿠키런", "쿠키런 인형"}},

	// ── 카카오프렌즈 ───────────────────────────────────────────────────
	{"ryan", "라이언", "kakao", []string{"라이언", "카카오 라이언"}},
	{"choonsik", "춘식이", "kakao", []string{"춘식이"}},
	{"apeach", "어피치", "kakao", []string{"어피치"}},
	{"muzi", "무지", "kakao", []string{"무지", "카카오 무지"}},

	// ── 라인프렌즈 ─────────────────────────────────────────────────────
	{"brown", "브라운", "line", []string{"브라운", "라인프렌즈 브라운"}},
	{"cony", "코니", "line", []string{"코니", "라인프렌즈 코니"}},
	{"sally", "샐리", "line", []string{"샐리", "라인프렌즈 샐리"}},
	{"bt21", "BT21", "line", []string{"BT21", "비티이십일", "타타"}},

	// ── 디즈니·픽사 ────────────────────────────────────────────────────
	{"mickey", "미키마우스", "disney", []string{"미키마우스", "미니마우스"}},
	{"winnie", "곰돌이푸", "disney", []string{"곰돌이푸", "위니더푸"}},
	{"stitch", "스티치", "disney", []string{"스티치", "스티치 인형"}},
	{"frozen", "겨울왕국", "disney", []string{"겨울왕국", "엘사", "올라프"}},
	{"toystory", "토이스토리", "disney", []string{"토이스토리", "우디", "버즈"}},
	{"insideout", "인사이드아웃", "disney", []string{"인사이드아웃", "빙봉"}},

	// ── 일본 캐릭터 ────────────────────────────────────────────────────
	{"chiikawa", "치이카와", "jpchar", []string{"치이카와", "하치와레", "우사기"}},
	{"rilakkuma", "리락쿠마", "jpchar", []string{"리락쿠마"}},
	{"sumikko", "스밋코구라시", "jpchar", []string{"스밋코구라시", "스미코구라시"}},
	{"doraemon", "도라에몽", "jpchar", []string{"도라에몽"}},
	{"totoro", "토토로", "jpchar", []string{"토토로", "이웃집 토토로"}},
	{"molang", "몰랑", "jpchar", []string{"몰랑", "몰랑이"}},

	// ── 애니메이션 ─────────────────────────────────────────────────────
	{"jujutsu", "주술회전", "anime", []string{"주술회전", "고죠 사토루"}},
	{"demonslayer", "귀멸의 칼날", "anime", []string{"귀멸의 칼날", "네즈코", "탄지로"}},
	{"chainsawman", "체인소맨", "anime", []string{"체인소맨", "파워", "포치타"}},
	{"onepiece", "원피스", "anime", []string{"원피스", "루피 원피스", "쵸파"}},
	{"naruto", "나루토", "anime", []string{"나루토"}},
	{"bluelock", "블루록", "anime", []string{"블루록"}},
	{"haikyuu", "하이큐", "anime", []string{"하이큐"}},

	// ── 팝마트 ────────────────────────────────────────────────────────
	{"labubu", "라부부", "popmart", []string{"라부부", "labubu"}},
	{"molly", "몰리", "popmart", []string{"팝마트 몰리"}},
	{"dimoo", "디무", "popmart", []string{"디무", "dimoo"}},
	{"skullpanda", "스컬판다", "popmart", []string{"스컬판다"}},
	{"pinojelly", "피노젤리", "popmart", []string{"피노젤리"}},
	{"crybaby", "크라이베이비", "popmart", []string{"크라이베이비", "crybaby"}},

	// ── 게임 ──────────────────────────────────────────────────────────
	{"mario", "슈퍼마리오", "game", []string{"슈퍼마리오", "마리오 인형"}},
	{"kirby", "커비", "game", []string{"커비", "별의커비"}},
	{"animalcrossing", "동물의 숲", "game", []string{"동물의 숲", "모동숲"}},
	{"minecraft", "마인크래프트", "game", []string{"마인크래프트", "마크 인형"}},

	// ── 그 외 ─────────────────────────────────────────────────────────
	{"snoopy", "스누피", "etc", []string{"스누피", "피너츠"}},
	{"minions", "미니언즈", "etc", []string{"미니언즈", "미니언"}},
	{"jellycat", "젤리캣", "etc", []string{"젤리캣", "jellycat"}},
	{"sonnyangel", "소니엔젤", "etc", []string{"소니엔젤"}},
}

// Characters는 감시 대상 전부를 선언 순서로 반환한다.
func Characters() []Character {
	out := make([]Character, len(characters))
	copy(out, characters)
	return out
}

func CharacterByCode(code string) (Character, bool) {
	for _, c := range characters {
		if c.Code == code {
			return c, true
		}
	}
	return Character{}, false
}

// AnchorCharacter는 정규화 기준 캐릭터를 반환한다.
func AnchorCharacter() Character {
	c, ok := CharacterByCode(AnchorCharacterCode)
	if !ok {
		// 목록에서 앵커를 지우면 트렌드 전체가 무의미해진다.
		// 테스트가 먼저 잡지만, 런타임에도 조용히 넘어가지 않게 둔다.
		panic("앵커 캐릭터가 목록에 없습니다: " + AnchorCharacterCode)
	}
	return c
}

// TrendBatches는 감시 목록을 데이터랩 요청 단위로 자른다.
//
// 각 묶음은 **앵커를 맨 앞에 두고** 최대 5개다. 앵커가 매번 들어가므로
// 실제로 새로 보는 캐릭터는 묶음당 4개다. 캐릭터 68개면 17번 부른다 —
// 하루 한 번 도는 데 월 한도 5만 건의 1% 정도다.
//
// 앵커 자신은 감시 목록에서 빠진다. 같은 요청에 두 번 넣으면 데이터랩이
// 같은 이름의 그룹을 두 개 받게 되고, 정규화 기준이 자기 자신이 된다.
func TrendBatches(list []Character) [][]Character {
	anchor := AnchorCharacter()

	rest := make([]Character, 0, len(list))
	for _, c := range list {
		if c.Code != anchor.Code {
			rest = append(rest, c)
		}
	}
	if len(rest) == 0 {
		return nil
	}

	perBatch := naverGroupsPerRequest - 1
	out := make([][]Character, 0, (len(rest)+perBatch-1)/perBatch)
	for i := 0; i < len(rest); i += perBatch {
		end := min(i+perBatch, len(rest))
		batch := make([]Character, 0, naverGroupsPerRequest)
		batch = append(batch, anchor)
		batch = append(batch, rest[i:end]...)
		out = append(out, batch)
	}
	return out
}

// NormalizeTrend는 한 묶음의 상대 지수를 앵커 기준으로 고친다.
//
// 데이터랩의 값은 요청 안에서만 유효하다. 앵커 값으로 나누면 묶음이 달라도
// 같은 자로 잰 값이 되어 서로 비교할 수 있다.
//
// 앵커가 0이면 나눌 수 없다. 그 요청은 통째로 버린다 — 0으로 나눠서 나온
// 숫자를 순위라고 보여주는 것보다 그날 데이터가 없는 게 낫다.
func NormalizeTrend(anchorRatio float64, ratios map[string]float64) (map[string]float64, error) {
	if anchorRatio <= 0 {
		return nil, fmt.Errorf("앵커 지수가 0이라 정규화할 수 없습니다")
	}
	out := make(map[string]float64, len(ratios))
	for code, r := range ratios {
		out[code] = r / anchorRatio
	}
	return out, nil
}
