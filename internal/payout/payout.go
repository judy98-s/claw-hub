// Package payout은 환불 송금 수단을 만든다.
//
// 국내 결제 환경에서 "버튼 하나로 서버가 계좌이체"는 1단계에서 불가능하다.
// 카카오페이·토스페이먼츠는 결제(수납) API이지 송금 API가 아니고, 실제
// 송금에는 지급대행·펌뱅킹·오픈뱅킹 출금이체 계약이 필요하다(사업자 심사,
// 수 주 소요).
//
// 그래서 1단계는 딥링크다. 사장님이 버튼을 누르면 토스/카카오뱅크 앱이
// 계좌와 금액이 채워진 송금 화면으로 열리고, 인증 한 번으로 끝난다.
// 나중에 지급대행 계약이 되면 이 인터페이스의 구현을 하나 추가하면 된다.
package payout

// Link는 사장님에게 보여줄 송금 수단 하나다.
type Link struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

// Request는 송금 한 건의 정보다.
type Request struct {
	BankCode  string
	AccountNo string
	Holder    string
	AmountKRW int
}

// Payout은 송금 수단 제공자다.
type Payout interface {
	// Links는 이 건에 쓸 수 있는 송금 링크들을 반환한다.
	//
	// 빈 슬라이스는 에러가 아니다. 설정된 템플릿이 없으면 UI가 자동으로
	// "계좌 복사 + 수동 송금 완료" 경로만 보여준다.
	Links(Request) ([]Link, error)
	Kind() string
}
