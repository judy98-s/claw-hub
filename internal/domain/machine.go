package domain

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Machine은 QR 스티커가 붙은 기계 한 대다.
type Machine struct {
	ID       string
	StoreID  string
	Code     string // QR에 인코딩되는 공개 코드
	Label    string // "3번 기계"
	Location string
	Active   bool
}

// codeAlphabet은 기계 코드에 쓰는 문자다.
//
// 0/O, 1/I/L 처럼 눈으로 구분이 안 되는 문자를 뺐다. 스티커가 닳거나 QR이
// 긁혀서 스캔이 안 될 때 손님이 코드를 눈으로 읽어 입력해야 하는데,
// 그때 O를 0으로 잘못 읽으면 "기계를 찾을 수 없다"로 끝난다.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// CodeLength는 기계 코드의 자릿수다.
// 31^6 ≈ 8.9억 가지라 한 매장 규모에서 충돌은 사실상 없고, 손으로 옮겨 적기도 짧다.
const CodeLength = 6

// GenerateMachineCode는 새 기계 코드를 만든다.
func GenerateMachineCode() (string, error) {
	max := big.NewInt(int64(len(codeAlphabet)))
	var b strings.Builder
	for i := 0; i < CodeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("기계 코드 생성: %w", err)
		}
		b.WriteByte(codeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// NormalizeMachineCode는 손님이 손으로 입력한 코드를 표준형으로 바꾼다.
// 소문자와 혼동 문자를 보정해서, 눈으로 읽어 친 코드도 최대한 살린다.
func NormalizeMachineCode(s string) string {
	r := strings.NewReplacer(" ", "", "-", "", "O", "0", "o", "0", "I", "1", "i", "1", "l", "1")
	up := strings.ToUpper(r.Replace(s))
	// 위에서 O→0, I→1 로 모았으니 알파벳에 존재하는 문자로 되돌린다.
	return strings.NewReplacer("0", "Q", "1", "J").Replace(up)
}

// ValidateMachineCode는 코드가 형식에 맞는지 본다.
func ValidateMachineCode(s string) error {
	if len(s) != CodeLength {
		return fmt.Errorf("기계 코드는 %d자리여야 합니다", CodeLength)
	}
	for _, c := range s {
		if !strings.ContainsRune(codeAlphabet, c) {
			return fmt.Errorf("기계 코드에 쓸 수 없는 문자가 있습니다: %q", c)
		}
	}
	return nil
}
