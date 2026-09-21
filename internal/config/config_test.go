package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

// 테스트용 32바이트 base64 키
func key32(b byte) string {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = b
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":        "postgres://localhost/clawhub",
		"DATA_ENCRYPTION_KEY": key32(1),
		"HASH_PEPPER":         key32(2),
		"SESSION_SECRET":      key32(3),
	}
}

func loadFrom(t *testing.T, env map[string]string) (Config, error) {
	t.Helper()
	return LoadFrom(func(k string) string { return env[k] })
}

func TestLoad_정상(t *testing.T) {
	cfg, err := loadFrom(t, validEnv())
	if err != nil {
		t.Fatalf("정상 환경인데 에러: %v", err)
	}
	if cfg.DatabaseURL != "postgres://localhost/clawhub" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if len(cfg.DataEncryptionKey) != 32 {
		t.Errorf("DataEncryptionKey 길이 = %d, want 32", len(cfg.DataEncryptionKey))
	}
	if len(cfg.HashPepper) != 32 {
		t.Errorf("HashPepper 길이 = %d, want 32", len(cfg.HashPepper))
	}
}

func TestLoad_필수키누락은_한번에_전부_보고한다(t *testing.T) {
	// 하나씩 고치며 재기동하는 건 시간 낭비다. 누락된 키를 모아서 알려줘야 한다.
	_, err := loadFrom(t, map[string]string{})
	if err == nil {
		t.Fatal("필수 키가 전부 없는데 에러가 없다")
	}
	for _, want := range []string{"DATABASE_URL", "DATA_ENCRYPTION_KEY", "HASH_PEPPER", "SESSION_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("에러 메시지에 %s 가 없다: %v", want, err)
		}
	}
}

func TestLoad_암호화키_길이검증(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"31바이트", base64.StdEncoding.EncodeToString(make([]byte, 31))},
		{"33바이트", base64.StdEncoding.EncodeToString(make([]byte, 33))},
		{"base64가 아님", "!!!not-base64!!!"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			env["DATA_ENCRYPTION_KEY"] = tc.value
			_, err := loadFrom(t, env)
			if err == nil {
				t.Fatal("잘못된 키인데 에러가 없다")
			}
			if !strings.Contains(err.Error(), "DATA_ENCRYPTION_KEY") {
				t.Errorf("어느 키가 문제인지 알려주지 않는다: %v", err)
			}
		})
	}
}

func TestLoad_REDIS_URL_없으면_인메모리_모드(t *testing.T) {
	// Redis 장애나 미설치가 접수 실패로 번지면 안 된다.
	cfg, err := loadFrom(t, validEnv())
	if err != nil {
		t.Fatalf("REDIS_URL 없이 로드 실패: %v", err)
	}
	if cfg.RedisURL != "" {
		t.Errorf("RedisURL = %q, want \"\" (인메모리)", cfg.RedisURL)
	}
}

func TestLoad_정책_기본값(t *testing.T) {
	cfg, err := loadFrom(t, validEnv())
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"ReviewThresholdKRW", cfg.Policy.ReviewThresholdKRW, 10000},
		{"RepeatWatchCount", cfg.Policy.RepeatWatchCount, 3},
		{"RepeatHoldCount", cfg.Policy.RepeatHoldCount, 5},
		{"MachineAlertCount", cfg.Policy.MachineAlertCount, 3},
		{"PhotoRetentionDays", cfg.Policy.PhotoRetentionDays, 90},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestLoad_정책_환경변수_오버라이드(t *testing.T) {
	env := validEnv()
	env["POLICY_REVIEW_THRESHOLD_KRW"] = "20000"
	env["POLICY_REPEAT_HOLD_COUNT"] = "7"
	cfg, err := loadFrom(t, env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Policy.ReviewThresholdKRW != 20000 {
		t.Errorf("ReviewThresholdKRW = %d, want 20000", cfg.Policy.ReviewThresholdKRW)
	}
	if cfg.Policy.RepeatHoldCount != 7 {
		t.Errorf("RepeatHoldCount = %d, want 7", cfg.Policy.RepeatHoldCount)
	}
}

func TestLoad_정책값이_숫자가_아니면_에러(t *testing.T) {
	env := validEnv()
	env["POLICY_REVIEW_THRESHOLD_KRW"] = "만원"
	_, err := loadFrom(t, env)
	if err == nil {
		t.Fatal("숫자가 아닌 정책값인데 에러가 없다")
	}
	if !strings.Contains(err.Error(), "POLICY_REVIEW_THRESHOLD_KRW") {
		t.Errorf("어느 키가 문제인지 알려주지 않는다: %v", err)
	}
}

func TestLoad_딥링크_템플릿_파싱(t *testing.T) {
	env := validEnv()
	env["PAYOUT_DEEPLINK_TEMPLATES"] = `{"toss":"supertoss://send?amount={amount}","kakaobank":"kakaobank://transfer"}`
	cfg, err := loadFrom(t, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PayoutDeeplinkTemplates) != 2 {
		t.Fatalf("템플릿 %d개, want 2", len(cfg.PayoutDeeplinkTemplates))
	}
	if cfg.PayoutDeeplinkTemplates["toss"] != "supertoss://send?amount={amount}" {
		t.Errorf("toss 템플릿 = %q", cfg.PayoutDeeplinkTemplates["toss"])
	}
}

func TestLoad_딥링크_템플릿_없어도_정상(t *testing.T) {
	// 템플릿이 없으면 수동 기록 경로로 자연히 떨어진다. 에러가 아니다.
	cfg, err := loadFrom(t, validEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PayoutDeeplinkTemplates) != 0 {
		t.Errorf("템플릿이 비어야 한다: %v", cfg.PayoutDeeplinkTemplates)
	}
}
