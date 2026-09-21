package payout

// Provider는 사장님이 고를 수 있는 송금 앱이다.
type Provider struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Prefills가 true면 은행·계좌·금액이 앱에 자동으로 채워진다.
	Prefills bool `json:"prefills"`
	// Note는 설정 화면에 그대로 보이는 안내다.
	Note string `json:"note"`
	// Template은 딥링크 URL 형식이다. 비어 있으면 딥링크를 만들지 않는다.
	Template string `json:"-"`
}

// ProviderNone은 딥링크를 쓰지 않는다는 선택이다.
const ProviderNone = "none"

// ProviderCustom은 직접 입력한 템플릿을 쓴다.
const ProviderCustom = "custom"

// providers는 고를 수 있는 앱 목록이다.
//
// 토스 스킴은 토스가 공식 문서로 제공하는 규격이 아니라 커뮤니티가 관찰해
// 정리한 값이다. 예고 없이 바뀔 수 있다.
//
// 카카오뱅크는 계좌·금액을 채워주는 공개 스킴이 확인되지 않았다. 앱을
// 여는 것까지만 되고 나머지는 손으로 입력해야 한다. 그 사실을 감추면
// 사장님이 버튼을 누르고 "왜 안 채워지지" 하며 시간을 쓴다.
var providers = []Provider{
	{
		ID: ProviderNone, Label: "사용 안 함", Prefills: false,
		Note: "계좌번호 복사 버튼만 표시합니다. 은행 앱은 직접 여세요.",
	},
	{
		ID: "toss", Label: "토스", Prefills: true,
		Note:     "은행·계좌·금액이 자동으로 채워집니다.",
		Template: "supertoss://send?bank={bankShort}&accountNo={account}&amount={amount}",
	},
	{
		ID: "kakaobank", Label: "카카오뱅크", Prefills: false,
		Note:     "앱만 열립니다. 계좌와 금액은 직접 입력해야 합니다 — 자동 입력되는 공개 방식이 확인되지 않았습니다.",
		Template: "kakaobank://",
	},
	{
		ID: ProviderCustom, Label: "직접 입력", Prefills: true,
		Note: "딥링크 주소를 직접 넣습니다. 치환 토큰: {bankShort} {bankName} {bank} {account} {amount} {holder}",
	},
}

// Providers는 선택지 목록을 반환한다.
func Providers() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// ProviderByID는 id로 선택지를 찾는다.
func ProviderByID(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// TemplateFor는 설정에 맞는 딥링크 템플릿을 고른다.
//
// provider 가 custom 이면 사장님이 직접 넣은 값을 쓰고, 아니면 내장 값을
// 쓴다. 고르지 않았거나 '사용 안 함'이면 빈 문자열이고, 그러면 UI 가
// 계좌 복사 경로만 보여준다.
func TemplateFor(provider, custom string) string {
	if provider == ProviderCustom {
		return custom
	}
	p, ok := ProviderByID(provider)
	if !ok || p.ID == ProviderNone {
		return ""
	}
	return p.Template
}
