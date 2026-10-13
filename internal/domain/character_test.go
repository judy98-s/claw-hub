package domain

import (
	"strings"
	"testing"
)

func TestCharacters_목록이_성립한다(t *testing.T) {
	list := Characters()
	if len(list) < 40 || len(list) > 150 {
		t.Fatalf("캐릭터 %d개 — 너무 적거나 많다", len(list))
	}

	seenCode := map[string]bool{}
	seenName := map[string]bool{}
	for _, c := range list {
		if seenCode[c.Code] {
			t.Errorf("코드 중복: %q", c.Code)
		}
		seenCode[c.Code] = true

		// 이름이 겹치면 화면에 같은 줄이 두 개 뜬다.
		if seenName[c.Name] {
			t.Errorf("이름 중복: %q", c.Name)
		}
		seenName[c.Name] = true

		if c.Code == "" || c.Name == "" {
			t.Errorf("빈 항목: %+v", c)
		}
		if _, ok := CharacterGroupByCode(c.Group); !ok {
			t.Errorf("%s: 없는 그룹 %q", c.Code, c.Group)
		}
	}
}

func TestCharacters_검색어가_데이터랩_제약을_지킨다(t *testing.T) {
	for _, c := range Characters() {
		if len(c.Keywords) == 0 {
			t.Errorf("%s: 검색어가 없다 — 물어볼 말이 없으면 감시할 수 없다", c.Code)
			continue
		}
		// 데이터랩은 한 그룹에 최대 20개를 받는다.
		if len(c.Keywords) > 20 {
			t.Errorf("%s: 검색어 %d개 (최대 20)", c.Code, len(c.Keywords))
		}
		for _, k := range c.Keywords {
			if strings.TrimSpace(k) == "" {
				t.Errorf("%s: 빈 검색어", c.Code)
			}
		}
		// 표시명으로 검색이 안 되면 목록과 데이터가 어긋난다.
		if !contains(c.Keywords, c.Name) && !anyContains(c.Keywords, c.Name) {
			t.Errorf("%s(%q): 표시명으로 찾을 검색어가 없다 — %v", c.Code, c.Name, c.Keywords)
		}
	}
}

func TestAnchorCharacter_목록에_있다(t *testing.T) {
	// 앵커가 사라지면 정규화가 불가능해지고 트렌드 전체가 무의미해진다.
	a := AnchorCharacter()
	if a.Code != AnchorCharacterCode {
		t.Fatalf("앵커 = %q", a.Code)
	}
}

func TestTrendBatches_앵커가_매_묶음에_맨앞으로_들어간다(t *testing.T) {
	batches := TrendBatches(Characters())
	if len(batches) == 0 {
		t.Fatal("묶음이 없다")
	}

	anchor := AnchorCharacter()
	seen := map[string]int{}
	for i, b := range batches {
		if len(b) > naverGroupsPerRequest {
			t.Errorf("%d번째 묶음이 %d개 — 데이터랩은 한 번에 5개까지다", i, len(b))
		}
		if len(b) == 0 || b[0].Code != anchor.Code {
			t.Fatalf("%d번째 묶음의 첫 항목이 앵커가 아니다: %+v", i, b)
		}
		// 앵커가 한 묶음에 두 번 들어가면 정규화 기준이 자기 자신이 된다.
		for _, c := range b[1:] {
			if c.Code == anchor.Code {
				t.Errorf("%d번째 묶음에 앵커가 두 번 들어갔다", i)
			}
			seen[c.Code]++
		}
	}

	// 앵커를 뺀 전부가 정확히 한 번씩 나와야 한다.
	for _, c := range Characters() {
		if c.Code == anchor.Code {
			if seen[c.Code] != 0 {
				t.Errorf("앵커가 감시 대상에도 들어갔다")
			}
			continue
		}
		if seen[c.Code] != 1 {
			t.Errorf("%s 가 %d번 들어갔다 (1번이어야 한다)", c.Code, seen[c.Code])
		}
	}
}

func TestTrendBatches_호출_횟수가_한도_안에_들어온다(t *testing.T) {
	// 데이터랩은 월 5만 건이다. 하루 한 번 도는 데 이 횟수면
	// 한 달에 얼마나 쓰는지 계산이 서야 한다.
	batches := TrendBatches(Characters())
	monthly := len(batches) * 31
	if monthly > 50000 {
		t.Fatalf("하루 %d번 × 31일 = 월 %d건 — 한도(5만)를 넘는다", len(batches), monthly)
	}
	t.Logf("하루 %d번 호출 → 월 %d건 (한도 5만의 %.1f%%)",
		len(batches), monthly, float64(monthly)/500)
}

func TestTrendBatches_빈_목록(t *testing.T) {
	if got := TrendBatches(nil); got != nil {
		t.Errorf("빈 목록에 묶음이 나왔다: %+v", got)
	}
	// 앵커 하나뿐이면 새로 볼 게 없다.
	if got := TrendBatches([]Character{AnchorCharacter()}); got != nil {
		t.Errorf("앵커만 있는데 묶음이 나왔다: %+v", got)
	}
}

func TestNormalizeTrend(t *testing.T) {
	got, err := NormalizeTrend(50, map[string]float64{"a": 100, "b": 25})
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != 2 || got["b"] != 0.5 {
		t.Errorf("정규화 = %+v, want a=2 b=0.5", got)
	}
}

func TestNormalizeTrend_앵커가_0이면_버린다(t *testing.T) {
	// 0으로 나눠서 나온 숫자를 순위라고 보여주는 것보다, 그날 데이터가
	// 없는 게 낫다.
	if _, err := NormalizeTrend(0, map[string]float64{"a": 100}); err == nil {
		t.Error("앵커 0인데 통과했다")
	}
}

func TestCharacterGroups_중복없이_있다(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range CharacterGroups() {
		if g.Code == "" || g.Name == "" {
			t.Errorf("빈 그룹: %+v", g)
		}
		if seen[g.Code] {
			t.Errorf("그룹 코드 중복: %q", g.Code)
		}
		seen[g.Code] = true
	}
	// 쓰이지 않는 그룹은 화면의 빈 탭이 된다.
	used := map[string]bool{}
	for _, c := range Characters() {
		used[c.Group] = true
	}
	for _, g := range CharacterGroups() {
		if !used[g.Code] {
			t.Errorf("그룹 %q 에 캐릭터가 하나도 없다", g.Code)
		}
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func anyContains(ss []string, want string) bool {
	for _, s := range ss {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestCharacters_검색어가_캐릭터끼리_겹치지_않는다(t *testing.T) {
	// 같은 검색어가 두 캐릭터에 들어 있으면 그 검색량이 양쪽에 다 더해져
	// 둘 다 실제보다 높게 나온다. "루피" 하나를 잔망루피와 원피스에 같이
	// 넣는 실수가 이 유형이다.
	owner := map[string]string{}
	for _, c := range Characters() {
		for _, k := range c.Keywords {
			if prev, dup := owner[k]; dup {
				t.Errorf("검색어 %q 가 %s 와 %s 에 겹친다", k, prev, c.Code)
				continue
			}
			owner[k] = c.Code
		}
	}
}

func TestCharacters_한국_통칭이_빠지지_않았다(t *testing.T) {
	// 데이터랩은 동의어를 모르고 문자열을 그대로 집계한다. 공식명만 넣으면
	// 사람들이 실제로 치는 말이 통째로 빠져서 그 캐릭터가 하위권으로 나온다.
	//
	// 아래는 공식명과 한국 통칭이 다른 것들이다. 실제로 치이카와는
	// "먼작귀" 없이 물어봤다면 극장판이 130억 엔을 넘긴 달에도 검색량이
	// 거의 0으로 보였을 것이다.
	want := map[string][]string{
		"chiikawa":       {"먼작귀"},
		"teenieping":     {"캐치티니핑"},
		"demonslayer":    {"귀칼"},
		"shinchan":       {"크레용 신짱"},
		"manggom":        {"망곰"},
		"animalcrossing": {"모동숲"},
	}
	for code, aliases := range want {
		c, ok := CharacterByCode(code)
		if !ok {
			t.Errorf("%s: 목록에서 사라졌다", code)
			continue
		}
		for _, a := range aliases {
			if !contains(c.Keywords, a) {
				t.Errorf("%s(%q): 통칭 %q 가 검색어에 없다 — %v", code, c.Name, a, c.Keywords)
			}
		}
	}
}

func TestAnchorCharacter_검색어가_고정되어_있다(t *testing.T) {
	// 앵커의 검색어를 늘리면 기준값이 커져서 다른 캐릭터 전부가 내려간
	// 것처럼 보이고, 과거 수치와 비교할 수 없게 된다. 바꿀 때는 과거
	// 데이터를 버린다는 뜻이므로 이 테스트를 의식적으로 고쳐야 한다.
	a := AnchorCharacter()
	want := []string{"헬로키티", "키티"}
	if len(a.Keywords) != len(want) {
		t.Fatalf("앵커 검색어 = %v, 기대 = %v", a.Keywords, want)
	}
	for i := range want {
		if a.Keywords[i] != want[i] {
			t.Fatalf("앵커 검색어 = %v, 기대 = %v", a.Keywords, want)
		}
	}
}
