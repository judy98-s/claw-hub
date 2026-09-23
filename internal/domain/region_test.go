package domain

import "testing"

func TestRegions_17개_시도가_중복없이_있다(t *testing.T) {
	rs := Regions()
	if len(rs) != 17 {
		t.Fatalf("시·도 %d개, want 17", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if r.Code == "" || r.Name == "" {
			t.Errorf("빈 항목: %+v", r)
		}
		if seen[r.Code] {
			t.Errorf("코드 중복: %q", r.Code)
		}
		seen[r.Code] = true
	}
}

func TestRegionByCode(t *testing.T) {
	r, ok := RegionByCode("seoul")
	if !ok || r.Name != "서울" {
		t.Errorf("RegionByCode(seoul) = %+v, %v", r, ok)
	}
	if _, ok := RegionByCode("atlantis"); ok {
		t.Error("없는 코드가 통과했다")
	}
	if _, ok := RegionByCode(""); ok {
		t.Error("빈 코드가 통과했다")
	}
}

func TestValidateRegionDetail(t *testing.T) {
	if err := ValidateRegionDetail("강남구"); err != nil {
		t.Errorf("정상 입력이 거부됐다: %v", err)
	}
	for _, s := range []string{"", "   ", "강남구강남구강남구강남구강남구강남구강남구"} {
		if err := ValidateRegionDetail(s); err == nil {
			t.Errorf("ValidateRegionDetail(%q) 가 통과했다", s)
		}
	}
}
