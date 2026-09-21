package payout

import (
	"net/url"
	"strings"
	"testing"
)

func tossTemplate() map[string]string {
	return map[string]string{
		"toss": "supertoss://send?bank={bank}&accountNo={account}&amount={amount}&holder={holder}",
	}
}

func req() Request {
	return Request{BankCode: "090", AccountNo: "3333-01-1234567", Holder: "김민수", AmountKRW: 2000}
}

func TestLinks_템플릿_치환(t *testing.T) {
	links, err := NewDeeplink(tossTemplate()).Links(req())
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("링크 %d개, want 1", len(links))
	}

	u, err := url.Parse(links[0].URL)
	if err != nil {
		t.Fatalf("생성된 URL이 파싱 안 된다: %v", err)
	}
	q := u.Query()
	if q.Get("bank") != "090" {
		t.Errorf("bank = %q", q.Get("bank"))
	}
	if q.Get("accountNo") != "3333011234567" {
		t.Errorf("accountNo = %q, want 3333011234567 (하이픈 제거)", q.Get("accountNo"))
	}
	if q.Get("amount") != "2000" {
		t.Errorf("amount = %q", q.Get("amount"))
	}
	if q.Get("holder") != "김민수" {
		t.Errorf("holder = %q — URL 인코딩 왕복이 깨졌다", q.Get("holder"))
	}
	if links[0].Label != "토스로 보내기" {
		t.Errorf("Label = %q", links[0].Label)
	}
}

func TestLinks_한글과_특수문자가_URL인코딩된다(t *testing.T) {
	r := req()
	r.Holder = "김 민수&test=1"
	links, err := NewDeeplink(tossTemplate()).Links(r)
	if err != nil {
		t.Fatal(err)
	}
	// 인코딩이 안 되면 &test=1 이 별개 쿼리 파라미터가 되어 값이 잘린다.
	u, _ := url.Parse(links[0].URL)
	if got := u.Query().Get("holder"); got != "김 민수&test=1" {
		t.Errorf("holder = %q, want %q — 인코딩되지 않아 값이 깨졌다", got, r.Holder)
	}
}

func TestLinks_템플릿이_없으면_빈슬라이스_에러아님(t *testing.T) {
	// 딥링크 미설정은 정상 상태다. UI가 수동 기록 경로로 떨어진다.
	links, err := NewDeeplink(nil).Links(req())
	if err != nil {
		t.Fatalf("템플릿 없음이 에러가 됐다: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("링크 %d개, want 0", len(links))
	}

	links, err = NewDeeplink(map[string]string{"toss": "   "}).Links(req())
	if err != nil || len(links) != 0 {
		t.Errorf("공백 템플릿이 링크가 됐다: %v %v", links, err)
	}
}

func TestLinks_순서가_안정적이다(t *testing.T) {
	// 맵 순회는 무작위다. 버튼 순서가 매번 바뀌면 사장님이 헷갈린다.
	tpl := map[string]string{
		"toss":      "supertoss://send?a={amount}",
		"kakaobank": "kakaobank://transfer?a={amount}",
		"kbank":     "kbank://send?a={amount}",
	}
	d := NewDeeplink(tpl)
	var first []string
	for i := 0; i < 20; i++ {
		links, err := d.Links(req())
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(links))
		for j, l := range links {
			got[j] = l.Provider
		}
		if first == nil {
			first = got
			continue
		}
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("순서가 바뀐다: %v vs %v", first, got)
		}
	}
}

func TestLinks_입력검증(t *testing.T) {
	d := NewDeeplink(tossTemplate())
	tests := []struct {
		name string
		mut  func(*Request)
	}{
		{"금액 0", func(r *Request) { r.AmountKRW = 0 }},
		{"금액 음수", func(r *Request) { r.AmountKRW = -1000 }},
		{"알 수 없는 은행", func(r *Request) { r.BankCode = "999" }},
		{"빈 은행코드", func(r *Request) { r.BankCode = "" }},
		{"빈 계좌", func(r *Request) { r.AccountNo = "" }},
		{"숫자 없는 계좌", func(r *Request) { r.AccountNo = "---" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := req()
			tc.mut(&r)
			if _, err := d.Links(r); err == nil {
				t.Error("통과됐다")
			}
		})
	}
}

func TestLinks_알려지지_않은_제공자도_동작한다(t *testing.T) {
	// 새 앱이 생기면 환경변수만 추가하면 되어야 한다. 코드 배포 없이.
	links, err := NewDeeplink(map[string]string{"newbank": "newbank://send?a={amount}"}).Links(req())
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Provider != "newbank" {
		t.Fatalf("링크 = %+v", links)
	}
	if links[0].Label != "newbank" {
		t.Errorf("Label = %q, 제공자명으로 폴백해야 한다", links[0].Label)
	}
}

func TestBanks(t *testing.T) {
	list := Banks()
	if len(list) < 10 {
		t.Errorf("은행이 %d개뿐이다", len(list))
	}

	seen := map[string]bool{}
	for _, b := range list {
		if b.Code == "" || b.Name == "" {
			t.Errorf("빈 값이 있다: %+v", b)
		}
		if seen[b.Code] {
			t.Errorf("코드 중복: %s", b.Code)
		}
		seen[b.Code] = true
	}

	// 인터넷은행이 앞에 와야 한다 — 주 이용층이 가장 많이 쓴다.
	if list[0].Name != "토스뱅크" {
		t.Errorf("첫 은행 = %q, 인터넷은행이 앞에 와야 한다", list[0].Name)
	}

	b, ok := BankByCode("092")
	if !ok || b.Name != "카카오뱅크" {
		t.Errorf("BankByCode(092) = %+v, %v", b, ok)
	}
	if _, ok := BankByCode("999"); ok {
		t.Error("없는 코드가 조회됐다")
	}
}

func TestBanks_반환값을_바꿔도_원본이_안_바뀐다(t *testing.T) {
	list := Banks()
	list[0].Name = "바뀜"
	if Banks()[0].Name == "바뀜" {
		t.Error("내부 슬라이스가 그대로 노출됐다")
	}
}
