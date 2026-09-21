package payout

// Bank는 송금 대상 은행이다.
// Code는 금융결제원 기관코드로, 딥링크 템플릿과 계좌 표기에 함께 쓴다.
type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// banks는 손님이 고를 수 있는 은행이다.
// 인터넷은행을 앞에 뒀다 — 인형뽑기 주 이용층이 가장 많이 쓴다.
var banks = []Bank{
	{"090", "토스뱅크"},
	{"089", "케이뱅크"},
	{"092", "카카오뱅크"},
	{"004", "국민은행"},
	{"088", "신한은행"},
	{"020", "우리은행"},
	{"081", "하나은행"},
	{"011", "농협은행"},
	{"003", "기업은행"},
	{"023", "SC제일은행"},
	{"027", "한국씨티은행"},
	{"031", "대구은행"},
	{"032", "부산은행"},
	{"034", "광주은행"},
	{"035", "제주은행"},
	{"037", "전북은행"},
	{"039", "경남은행"},
	{"045", "새마을금고"},
	{"048", "신협"},
	{"071", "우체국"},
	{"007", "수협은행"},
}

// Banks는 은행 목록을 반환한다.
func Banks() []Bank {
	out := make([]Bank, len(banks))
	copy(out, banks)
	return out
}

// BankByCode는 코드로 은행을 찾는다.
func BankByCode(code string) (Bank, bool) {
	for _, b := range banks {
		if b.Code == code {
			return b, true
		}
	}
	return Bank{}, false
}
