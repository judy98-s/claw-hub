package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// docs/stores.json 이 디자인 원본이다. 서버 표와 어긋나면 손님이 고른
// 증상이 400 으로 떨어진다 — 손님은 이유를 모르고 그냥 떠난다. 화면에서는
// 아무 에러도 안 보이므로, 어긋난 것을 알 수 있는 유일한 자리가 여기다.
//
// web/lib/stores.ts 도 같은 파일에서 나왔다. 셋 중 하나만 고치면 여기서 깨진다.
func TestIssues_디자인_원본과_같다(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "stores.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s 를 읽지 못했다: %v", path, err)
	}

	// "_note" 처럼 업종이 아닌 키가 섞여 있어서 한 번에 못 푼다.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("stores.json 파싱 실패: %v", err)
	}

	// issues 는 [코드, 라벨, 설명] 3칸 배열이다.
	type storeSpec struct {
		Label  string     `json:"label"`
		Issues [][]string `json:"issues"`
	}

	for _, st := range AllStoreTypes() {
		blob, ok := raw[string(st)]
		if !ok {
			t.Errorf("%s: stores.json 에 없다", st)
			continue
		}
		var row storeSpec
		if err := json.Unmarshal(blob, &row); err != nil {
			t.Errorf("%s: 파싱 실패: %v", st, err)
			continue
		}
		if got := StoreTypeLabel(st); got != row.Label {
			t.Errorf("%s: 업종 이름이 다르다 — Go %q, stores.json %q", st, got, row.Label)
		}

		want := IssuesFor(st)
		if len(want) != len(row.Issues) {
			t.Errorf("%s: 증상 개수가 다르다 — Go %d개, stores.json %d개", st, len(want), len(row.Issues))
			continue
		}
		for i, issue := range row.Issues {
			if len(issue) < 2 {
				t.Fatalf("%s[%d]: stores.json 의 issues 는 [코드, 라벨, 설명] 이어야 한다: %v", st, i, issue)
			}
			code, label := issue[0], issue[1]
			// 순서까지 본다. 손님 화면의 버튼 순서가 이 순서다.
			if string(want[i]) != code {
				t.Errorf("%s[%d]: 코드가 다르다 — Go %q, stores.json %q", st, i, want[i], code)
				continue
			}
			if got := want[i].Label(); got != label {
				t.Errorf("%s[%d] %s: 라벨이 다르다 — Go %q, stores.json %q", st, i, code, got, label)
			}
		}
	}
}

// 코드가 업종 사이에서 겹치면 겹친 쪽 라벨이 하나만 남는다. 그러면
// 집계와 Slack 에서 "no_start" 가 작동 안 함인지 촬영 안 됨인지 알 수 없다.
// 접두사를 붙이지 않기로 한 결정이 이 테스트에 의존한다.
func TestIssues_업종이_달라도_코드는_한_뜻이다(t *testing.T) {
	owner := map[IssueType]StoreType{}
	for _, st := range AllStoreTypes() {
		for _, it := range IssuesFor(st) {
			if it == IssueOther {
				continue // 기타는 공통이다
			}
			if prev, dup := owner[it]; dup {
				t.Errorf("%q 가 %s 와 %s 에 함께 있다. 뜻이 같지 않으면 코드를 나눠야 한다", it, prev, st)
				continue
			}
			owner[it] = st
		}
	}
}

func TestValidIssueFor_업종이_다른_증상을_거절한다(t *testing.T) {
	if ValidIssueFor(StoreClaw, IssueNotDry) {
		t.Error("뽑기방이 세탁기 증상(건조 덜 됨)을 받았다")
	}
	if ValidIssueFor(StoreLaundry, IssueDollStuck) {
		t.Error("세탁소가 인형 걸림을 받았다")
	}
	if !ValidIssueFor(StoreLaundry, IssueNotDry) {
		t.Error("세탁소가 자기 증상을 거절했다")
	}
	// 기타는 모든 업종에 있어야 한다. 손님이 고를 게 없으면 전화가 온다.
	for _, st := range AllStoreTypes() {
		if !ValidIssueFor(st, IssueOther) {
			t.Errorf("%s 에 기타가 없다", st)
		}
	}
	// 모르는 업종은 뽑기방으로 떨어진다 (NormalizeStoreType 과 같은 규칙).
	if !ValidIssueFor(StoreType("nonexistent"), IssueDollStuck) {
		t.Error("모르는 업종이 뽑기방으로 떨어지지 않았다")
	}
}

// 저장된 코드를 enum 에서 지운 뒤에도 과거 민원은 열려야 한다.
func TestIssueLabel_모르는_코드는_코드를_그대로_보여준다(t *testing.T) {
	if got := IssueType("retired_code").Label(); got != "retired_code" {
		t.Errorf("빈칸이 아니라 코드가 나와야 한다: %q", got)
	}
}

func TestAllIssueTypes_중복이_없고_전부_라벨이_있다(t *testing.T) {
	seen := map[IssueType]bool{}
	for _, it := range AllIssueTypes() {
		if seen[it] {
			t.Errorf("%q 가 두 번 나온다", it)
		}
		seen[it] = true
		if _, ok := issueLabels[it]; !ok {
			t.Errorf("%q 에 라벨이 없다", it)
		}
	}
	// 반대 방향 — 라벨만 있고 어느 업종에도 안 쓰이는 코드는 죽은 코드다.
	for it := range issueLabels {
		if !seen[it] {
			t.Errorf("%q 는 라벨만 있고 어느 업종에도 없다", it)
		}
	}
}
