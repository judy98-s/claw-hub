package domain

import (
	"strings"
	"testing"
)

func TestGenerateMachineCode_형식과_중복(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		code, err := GenerateMachineCode()
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateMachineCode(code); err != nil {
			t.Fatalf("생성한 코드가 검증을 통과 못한다 %q: %v", code, err)
		}
		if seen[code] {
			t.Fatalf("2000개 안에 중복이 나왔다: %q", code)
		}
		seen[code] = true
	}
}

func TestCodeAlphabet_혼동문자가_없다(t *testing.T) {
	// 스티커가 닳으면 손님이 코드를 눈으로 읽어 입력한다.
	// O를 0으로 잘못 읽으면 "기계를 찾을 수 없다"로 끝난다.
	for _, c := range "01OIL" {
		if strings.ContainsRune(codeAlphabet, c) {
			t.Errorf("혼동되는 문자 %q 가 알파벳에 있다", c)
		}
	}
}

func TestValidateMachineCode(t *testing.T) {
	bad := []string{"", "ABC", "ABCDEFG", "ABCDE0", "ABCDEO", "abcdef", "ABC-DE"}
	for _, s := range bad {
		if err := ValidateMachineCode(s); err == nil {
			t.Errorf("ValidateMachineCode(%q) 가 통과됐다", s)
		}
	}
	if err := ValidateMachineCode("ABCD23"); err != nil {
		t.Errorf("정상 코드가 거부됐다: %v", err)
	}
}

func TestNormalizeMachineCode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abcd23", "ABCD23"},
		{"ABC-D23", "ABCD23"},
		{"ABC D23", "ABCD23"},
		{"QBCD23", "QBCD23"},
		{"0BCD23", "QBCD23"}, // 0 을 Q 로 읽어준다
		{"OBCD23", "QBCD23"}, // O 도 마찬가지
		{"1BCD23", "JBCD23"}, // 1 을 J 로
		{"lBCD23", "JBCD23"},
	}
	for _, tc := range tests {
		if got := NormalizeMachineCode(tc.in); got != tc.want {
			t.Errorf("NormalizeMachineCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
