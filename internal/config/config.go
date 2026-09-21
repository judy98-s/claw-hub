// Package config는 환경변수에서 설정을 읽고 검증한다.
//
// 필수 항목이 하나라도 비어 있으면 Load는 실패한다. 암호화 키 없이 개인정보를
// 받는 상태로 서버가 떠 있는 것보다, 아예 뜨지 않는 게 낫다.
package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// keyLen은 암호화 키·pepper·세션 시크릿에 요구하는 바이트 길이다.
// AES-256과 HMAC-SHA256 모두 32바이트를 쓴다.
const keyLen = 32

// Policy는 리스크 평가와 운영 임계값을 담는다.
// 현장 데이터가 쌓이면 재배포 없이 조정할 수 있도록 전부 환경변수로 뺐다.
type Policy struct {
	ReviewThresholdKRW   int // 이 금액 이상이면 사진 필수 + 수동 검토
	RepeatWatchCount     int // 30일 내 이 횟수 이상 신고하면 검토 대상
	RepeatHoldCount      int // 30일 내 이 횟수 이상이면 자동 보류
	AccountSharingPhones int // 한 계좌에 연결된 서로 다른 번호가 이 수 이상이면 검토
	PayoutCeilingKRW     int // 30일 누적 지급액이 이 금액을 넘으면 검토
	MaxAmountKRW         int // 이 금액을 넘는 신고는 접수 자체를 거부
	MachineAlertCount    int // 24시간 내 한 기계에 이 건수 이상이면 점검 알림
	PhotoRetentionDays   int // 사진 보존 기간
}

// Config는 프로세스 기동에 필요한 전체 설정이다.
type Config struct {
	Addr        string
	DatabaseURL string
	RedisURL    string // 빈 문자열이면 인메모리 캐시를 쓴다

	DataEncryptionKey []byte
	HashPepper        []byte
	SessionSecret     []byte

	SlackWebhookURL string // 빈 문자열이면 알림 비활성
	MediaDir        string
	PublicBaseURL   string

	PayoutDeeplinkTemplates map[string]string

	Policy Policy
}

// Getenv는 환경변수 조회 함수다. 테스트에서 주입할 수 있게 분리했다.
type Getenv func(string) string

// Load는 설정을 읽는다.
//
// 작업 디렉터리에 .env 가 있으면 함께 읽는다. 진짜 환경변수가 우선한다.
// 파일이 없어도 정상이다 — 도커나 CI 에서는 환경변수로 넘어온다.
func Load() (Config, error) {
	file, found := loadDotEnv(DotEnvPath)
	cfg, err := LoadFrom(envWithDotEnv(file))
	if err != nil && !found {
		return Config{}, fmt.Errorf("%w\n\n  이 디렉터리에 .env 가 없습니다. `cp .env.example .env` 후 `make keys` 로 값을 채우세요.", err)
	}
	return cfg, err
}

// LoadFrom은 주어진 조회 함수로 설정을 읽는다.
//
// 검증 실패는 첫 오류에서 멈추지 않고 전부 모아서 보고한다. 키를 하나 고칠
// 때마다 재기동해서 다음 오류를 확인하는 건 시간 낭비다.
func LoadFrom(get Getenv) (Config, error) {
	v := &loader{get: get}

	cfg := Config{
		Addr:            v.str("ADDR", ":8080"),
		DatabaseURL:     v.required("DATABASE_URL"),
		RedisURL:        v.str("REDIS_URL", ""),
		SlackWebhookURL: v.str("SLACK_WEBHOOK_URL", ""),
		MediaDir:        v.str("MEDIA_DIR", "./data/photos"),
		PublicBaseURL:   v.str("PUBLIC_BASE_URL", "http://localhost:3000"),

		DataEncryptionKey: v.key("DATA_ENCRYPTION_KEY"),
		HashPepper:        v.key("HASH_PEPPER"),
		SessionSecret:     v.key("SESSION_SECRET"),

		PayoutDeeplinkTemplates: v.jsonMap("PAYOUT_DEEPLINK_TEMPLATES"),

		Policy: Policy{
			ReviewThresholdKRW:   v.intVal("POLICY_REVIEW_THRESHOLD_KRW", 10000),
			RepeatWatchCount:     v.intVal("POLICY_REPEAT_WATCH_COUNT", 3),
			RepeatHoldCount:      v.intVal("POLICY_REPEAT_HOLD_COUNT", 5),
			AccountSharingPhones: v.intVal("POLICY_ACCOUNT_SHARING_PHONES", 3),
			PayoutCeilingKRW:     v.intVal("POLICY_PAYOUT_CEILING_KRW", 50000),
			MaxAmountKRW:         v.intVal("POLICY_MAX_AMOUNT_KRW", 1000000),
			MachineAlertCount:    v.intVal("POLICY_MACHINE_ALERT_COUNT", 3),
			PhotoRetentionDays:   v.intVal("PHOTO_RETENTION_DAYS", 90),
		},
	}

	if len(v.problems) > 0 {
		return Config{}, fmt.Errorf("설정 오류:\n  - %s", strings.Join(v.problems, "\n  - "))
	}
	return cfg, nil
}

// loader는 읽는 동안 발견한 문제를 모은다.
type loader struct {
	get      Getenv
	problems []string
}

func (l *loader) fail(format string, args ...any) {
	l.problems = append(l.problems, fmt.Sprintf(format, args...))
}

func (l *loader) str(name, def string) string {
	if s := l.get(name); s != "" {
		return s
	}
	return def
}

func (l *loader) required(name string) string {
	s := l.get(name)
	if s == "" {
		l.fail("%s 가 비어 있습니다 (필수)", name)
	}
	return s
}

// key는 base64로 인코딩된 32바이트 키를 읽는다.
func (l *loader) key(name string) []byte {
	raw := l.get(name)
	if raw == "" {
		l.fail("%s 가 비어 있습니다 (필수). `make keys` 로 생성하세요", name)
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		l.fail("%s 가 올바른 base64가 아닙니다", name)
		return nil
	}
	if len(b) != keyLen {
		l.fail("%s 는 %d바이트여야 합니다 (현재 %d바이트)", name, keyLen, len(b))
		return nil
	}
	return b
}

func (l *loader) intVal(name string, def int) int {
	raw := l.get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		l.fail("%s 는 정수여야 합니다 (현재 %q)", name, raw)
		return def
	}
	return n
}

// jsonMap은 JSON 객체를 문자열 맵으로 읽는다. 비어 있으면 빈 맵이다.
func (l *loader) jsonMap(name string) map[string]string {
	raw := l.get(name)
	if raw == "" {
		return map[string]string{}
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		l.fail("%s 가 올바른 JSON 객체가 아닙니다", name)
		return map[string]string{}
	}
	return m
}
