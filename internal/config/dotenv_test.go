package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDotEnv_기본형식(t *testing.T) {
	path := writeEnv(t, `
# 주석은 무시한다
DATABASE_URL=postgres://localhost/db

  SPACED_KEY  =  value with spaces
export EXPORTED=yes
QUOTED="따옴표 안"
SINGLE='홑따옴표'
EMPTY=
WITH_EQUALS=a=b=c
`)
	got, found := loadDotEnv(path)
	if !found {
		t.Fatal("파일이 있는데 found=false")
	}

	want := map[string]string{
		"DATABASE_URL": "postgres://localhost/db",
		"SPACED_KEY":   "value with spaces",
		"EXPORTED":     "yes",
		"QUOTED":       "따옴표 안",
		"SINGLE":       "홑따옴표",
		"EMPTY":        "",
		"WITH_EQUALS":  "a=b=c",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("키 %d개, want %d: %v", len(got), len(want), got)
	}
}

func TestLoadDotEnv_JSON값이_깨지지_않는다(t *testing.T) {
	// 셸로 source 하면 & 와 {} 때문에 깨지는 값이다.
	tpl := `{"toss":"supertoss://send?bank={bankShort}&accountNo={account}&amount={amount}"}`
	path := writeEnv(t, "PAYOUT_DEEPLINK_TEMPLATES="+tpl+"\n")

	got, _ := loadDotEnv(path)
	if got["PAYOUT_DEEPLINK_TEMPLATES"] != tpl {
		t.Errorf("= %q\nwant %q", got["PAYOUT_DEEPLINK_TEMPLATES"], tpl)
	}
}

func TestLoadDotEnv_윈도우_줄바꿈과_BOM(t *testing.T) {
	// 윈도우 메모장으로 .env 를 편집하면 둘 다 붙는다. 그대로 두면
	// base64 키 끝에 \r 이 남아 "32바이트여야 합니다" 로 죽는다.
	path := writeEnv(t, bom+"DATA_ENCRYPTION_KEY=abc123\r\nHASH_PEPPER=def456\r\n")

	got, _ := loadDotEnv(path)
	if got["DATA_ENCRYPTION_KEY"] != "abc123" {
		t.Errorf("DATA_ENCRYPTION_KEY = %q — BOM 또는 \\r 이 남았다", got["DATA_ENCRYPTION_KEY"])
	}
	if got["HASH_PEPPER"] != "def456" {
		t.Errorf("HASH_PEPPER = %q", got["HASH_PEPPER"])
	}
}

func TestLoadDotEnv_주석처리(t *testing.T) {
	path := writeEnv(t, `
A=value # 뒤에 붙은 주석
B=value#해시가붙은값
C="따옴표 안의 # 은 유지"
`)
	got, _ := loadDotEnv(path)

	if got["A"] != "value" {
		t.Errorf("A = %q, want %q", got["A"], "value")
	}
	// 공백 없는 # 는 URL 조각일 수 있으므로 건드리지 않는다.
	if got["B"] != "value#해시가붙은값" {
		t.Errorf("B = %q", got["B"])
	}
	if got["C"] != "따옴표 안의 # 은 유지" {
		t.Errorf("C = %q", got["C"])
	}
}

func TestLoadDotEnv_파일이_없으면_found_false(t *testing.T) {
	got, found := loadDotEnv(filepath.Join(t.TempDir(), "없음"))
	if found {
		t.Error("없는 파일인데 found=true")
	}
	if len(got) != 0 {
		t.Errorf("빈 맵이어야 한다: %v", got)
	}
}

func TestEnvWithDotEnv_진짜_환경변수가_이긴다(t *testing.T) {
	// 도커 compose 의 environment 와 CI 비밀값이 파일을 덮어야 한다.
	// 반대면 서버에 남은 개발용 .env 가 운영 설정을 조용히 이긴다.
	t.Setenv("CLAWHUB_TEST_KEY", "환경변수")

	get := envWithDotEnv(map[string]string{
		"CLAWHUB_TEST_KEY": "파일",
		"ONLY_IN_FILE":     "파일값",
	})

	if got := get("CLAWHUB_TEST_KEY"); got != "환경변수" {
		t.Errorf("= %q, want 환경변수", got)
	}
	if got := get("ONLY_IN_FILE"); got != "파일값" {
		t.Errorf("= %q, want 파일값", got)
	}
	if got := get("아무데도없음"); got != "" {
		t.Errorf("= %q, want 빈 문자열", got)
	}
}
