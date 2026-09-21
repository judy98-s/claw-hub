package payout

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// providerLabels는 딥링크 제공자의 표시 이름이다.
var providerLabels = map[string]string{
	"toss":      "토스로 보내기",
	"kakaobank": "카카오뱅크로 보내기",
	"kbank":     "케이뱅크로 보내기",
}

// Deeplink는 설정된 템플릿으로 송금 앱 링크를 만든다.
type Deeplink struct {
	templates map[string]string
}

// NewDeeplink는 {provider: template} 맵으로 제공자를 만든다.
//
// 템플릿을 코드가 아니라 설정에 두는 이유: 딥링크 URL 스킴은 벤더가 예고
// 없이 바꾼다. 하드코딩하면 스킴이 바뀔 때마다 배포해야 하고, 그 사이
// 사장님은 송금 버튼이 아무것도 안 하는 걸 보게 된다. 설정이면 환경변수만
// 고치면 되고, 어떤 스킴도 안 열리면 수동 기록 경로로 자연히 떨어진다.
func NewDeeplink(templates map[string]string) *Deeplink {
	m := make(map[string]string, len(templates))
	for k, v := range templates {
		if strings.TrimSpace(v) != "" {
			m[k] = v
		}
	}
	return &Deeplink{templates: m}
}

func (d *Deeplink) Kind() string { return "deeplink" }

var nonDigit = regexp.MustCompile(`[^0-9]`)

// Links는 설정된 각 제공자에 대해 송금 링크를 만든다.
func (d *Deeplink) Links(r Request) ([]Link, error) {
	if r.AmountKRW <= 0 {
		return nil, fmt.Errorf("송금 금액이 올바르지 않습니다")
	}
	bank, ok := BankByCode(r.BankCode)
	if !ok {
		return nil, fmt.Errorf("알 수 없는 은행 코드입니다: %q", r.BankCode)
	}
	account := nonDigit.ReplaceAllString(r.AccountNo, "")
	if account == "" {
		return nil, fmt.Errorf("계좌번호가 비어 있습니다")
	}

	// 템플릿이 없으면 빈 슬라이스. 에러가 아니다 — 수동 기록으로 떨어진다.
	if len(d.templates) == 0 {
		return []Link{}, nil
	}

	// 각 값을 URL 인코딩한다. 예금주는 한글이고 이름에 공백이 들어간다.
	repl := strings.NewReplacer(
		"{bank}", url.QueryEscape(bank.Code),
		"{bankName}", url.QueryEscape(bank.Name),
		"{account}", url.QueryEscape(account),
		"{amount}", url.QueryEscape(strconv.Itoa(r.AmountKRW)),
		"{holder}", url.QueryEscape(r.Holder),
	)

	providers := make([]string, 0, len(d.templates))
	for p := range d.templates {
		providers = append(providers, p)
	}
	// 맵 순회 순서는 무작위다. 버튼 순서가 매번 바뀌면 사장님이 헷갈린다.
	sort.Strings(providers)

	out := make([]Link, 0, len(providers))
	for _, p := range providers {
		label, ok := providerLabels[p]
		if !ok {
			label = p
		}
		out = append(out, Link{Provider: p, Label: label, URL: repl.Replace(d.templates[p])})
	}
	return out, nil
}
