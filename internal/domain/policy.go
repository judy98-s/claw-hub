package domain

import (
	"fmt"
	"time"
)

// 연락처의 수동 상태. 사장님이 직접 지정한다.
const (
	ContactNormal  = "normal"  // 기본
	ContactWatch   = "watch"   // 지켜보는 중 — 신고가 오면 검토 대상
	ContactBlocked = "blocked" // 차단 — 신고가 오면 자동 보류
)

// ParseContactStatus는 외부 입력을 연락처 상태로 변환한다.
func ParseContactStatus(s string) (string, error) {
	switch s {
	case ContactNormal, ContactWatch, ContactBlocked:
		return s, nil
	}
	return "", fmt.Errorf("알 수 없는 연락처 상태: %q", s)
}

// Policy는 리스크 판단 임계값이다.
//
// 전부 설정으로 뺀 이유: 이 숫자들은 현장 데이터 없이 정한 추정값이다.
// 실제로 운영해 보면 "3건은 너무 빡빡하다" 같은 게 드러나고, 그때 재배포 없이
// 조정할 수 있어야 한다.
type Policy struct {
	ReviewThresholdKRW   int
	RepeatWatchCount     int
	RepeatHoldCount      int
	AccountSharingPhones int
	PayoutCeilingKRW     int
	MaxAmountKRW         int
	RapidDuplicateWindow time.Duration
}

// DefaultPolicy는 스펙 §4.3~4.4의 기본 임계값이다.
func DefaultPolicy() Policy {
	return Policy{
		ReviewThresholdKRW:   10000,
		RepeatWatchCount:     3,
		RepeatHoldCount:      5,
		AccountSharingPhones: 3,
		PayoutCeilingKRW:     50000,
		MaxAmountKRW:         1000000,
		RapidDuplicateWindow: 10 * time.Minute,
	}
}

// ValidateAmount는 접수 자체를 받을 수 있는 금액인지 검사한다.
// 리스크 평가 이전 단계다 — 여기서 걸리면 claim이 만들어지지 않는다.
func ValidateAmount(krw int, p Policy) error {
	if krw <= 0 {
		return fmt.Errorf("금액을 입력해주세요")
	}
	if krw > p.MaxAmountKRW {
		return fmt.Errorf("%d원을 넘는 금액은 접수할 수 없습니다. 매장으로 직접 연락해주세요", p.MaxAmountKRW)
	}
	return nil
}

// RequiresPhoto는 이 금액에 사진 첨부가 필수인지 알려준다.
// 서버 검증과 손님 화면의 안내가 같은 함수를 봐야 어긋나지 않는다.
func RequiresPhoto(krw int, p Policy) bool { return krw >= p.ReviewThresholdKRW }
