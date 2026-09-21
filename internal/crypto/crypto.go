// Package crypto는 개인정보의 암호화 저장과 조회용 해시를 담당한다.
//
// 이 패키지가 개인정보 경계를 소유한다. 도메인 코드는 평문만 보고,
// 저장 계층만 이 패키지를 통과한 바이트를 다룬다.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// KeyLen은 암호화 키와 pepper에 요구하는 바이트 길이다.
const KeyLen = 32

var ErrCiphertextTooShort = errors.New("암호문이 nonce보다 짧습니다")

// Cipher는 AES-256-GCM으로 개인정보를 암복호화한다.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher는 32바이트 키로 Cipher를 만든다.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("암호화 키는 %d바이트여야 합니다 (현재 %d바이트)", KeyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 초기화: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM 초기화: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt는 평문을 암호화한다. 결과는 nonce + 암호문 + 인증태그다.
//
// nonce는 매번 새로 뽑는다. 고정 nonce를 쓰면 같은 계좌번호가 항상 같은
// 바이트로 저장되어, 복호화하지 않고도 "이 두 건은 같은 계좌"라는 사실이
// DB 덤프만으로 드러난다.
func (c *Cipher) Encrypt(plain string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce 생성: %w", err)
	}
	return c.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Decrypt는 Encrypt가 만든 바이트를 평문으로 되돌린다.
// 변조된 입력은 GCM 인증 태그 검증에서 실패한다.
func (c *Cipher) Decrypt(ct []byte) (string, error) {
	ns := c.aead.NonceSize()
	if len(ct) < ns {
		return "", ErrCiphertextTooShort
	}
	plain, err := c.aead.Open(nil, ct[:ns], ct[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("복호화 실패: %w", err)
	}
	return string(plain), nil
}

// Hasher는 조회용 결정적 해시를 만든다.
type Hasher struct {
	pepper []byte
}

// NewHasher는 32바이트 pepper로 Hasher를 만든다.
func NewHasher(pepper []byte) (*Hasher, error) {
	if len(pepper) != KeyLen {
		return nil, fmt.Errorf("pepper는 %d바이트여야 합니다 (현재 %d바이트)", KeyLen, len(pepper))
	}
	return &Hasher{pepper: pepper}, nil
}

// Hash는 값을 HMAC-SHA256으로 해시한다.
//
// 순수 SHA256을 쓰면 안 된다. 한국 휴대폰 번호는 010 + 8자리라 경우의 수가
// 1억 미만이고, DB가 유출되면 평범한 장비로 전수 대입해 전부 복원된다.
// pepper는 코드와 DB 밖(환경변수)에 있으므로 DB만 유출돼도 복원되지 않는다.
func (h *Hasher) Hash(value string) []byte {
	m := hmac.New(sha256.New, h.pepper)
	m.Write([]byte(value))
	return m.Sum(nil)
}

var nonDigit = regexp.MustCompile(`[^0-9]`)

// phoneShape는 전화번호에 허용되는 문자다. 숫자와 서식용 구분자뿐이다.
// 문자가 섞인 입력을 그냥 숫자만 남겨 통과시키면 "010-1234-567a" 같은 오타가
// 10자리 유효 번호로 둔갑해, 손님은 연락을 못 받고 사장님은 왜인지 모른다.
var phoneShape = regexp.MustCompile(`^[0-9+\-.() ]+$`)

// mobilePrefixes는 휴대폰으로 인정하는 국번이다.
// 유선번호(02, 031 등)와 대표번호(1588 등)는 본인 확인 수단이 못 되므로 받지 않는다.
var mobilePrefixes = []string{"010", "011", "016", "017", "018", "019"}

// NormalizePhone은 전화번호를 숫자만 남긴 표준 형태로 바꾼다.
//
// 같은 사람이 "010-1234-5678"과 "01012345678"을 다르게 입력해도 같은 해시가
// 나와야 반복 신고 카운트가 동작한다.
func NormalizePhone(s string) (string, error) {
	if s == "" || !phoneShape.MatchString(s) {
		return "", fmt.Errorf("전화번호에 숫자가 아닌 문자가 있습니다")
	}
	d := nonDigit.ReplaceAllString(s, "")

	// 국가번호 82를 0으로 되돌린다. +82 10-1234-5678 → 01012345678
	if strings.HasPrefix(d, "82") && len(d) >= 11 {
		d = "0" + d[2:]
	}

	if len(d) < 10 || len(d) > 11 {
		return "", fmt.Errorf("휴대폰 번호는 10~11자리여야 합니다")
	}
	// 010은 11자리로 통일됐다. 10자리 010 번호는 존재하지 않으므로 오타다.
	if strings.HasPrefix(d, "010") && len(d) != 11 {
		return "", fmt.Errorf("010 번호는 11자리여야 합니다")
	}
	for _, p := range mobilePrefixes {
		if strings.HasPrefix(d, p) {
			return d, nil
		}
	}
	return "", fmt.Errorf("휴대폰 번호를 입력해주세요 (유선번호는 받지 않습니다)")
}

// NormalizeAccount는 계좌번호에서 숫자만 남긴다.
// 은행마다 하이픈 위치가 달라 표기를 통일할 수 없으므로 숫자열로만 비교한다.
func NormalizeAccount(s string) string {
	return nonDigit.ReplaceAllString(s, "")
}

// MaskPhone은 로그와 화면에 쓸 마스킹된 전화번호를 만든다.
// 입력이 전화번호로 보이지 않으면 통째로 가린다 — 형식이 이상할수록 더 가린다.
func MaskPhone(s string) string {
	d := nonDigit.ReplaceAllString(s, "")
	if len(d) < 10 || len(d) > 11 {
		return "***"
	}
	mid := d[3 : len(d)-4]
	return d[:3] + "-" + strings.Repeat("*", len(mid)) + "-" + d[len(d)-4:]
}

// MaskAccount는 계좌번호의 뒤 4자리만 남긴다.
// 사장님이 "이 계좌 맞나" 확인할 최소 정보이면서, 그것만으로는 송금할 수 없다.
func MaskAccount(s string) string {
	d := nonDigit.ReplaceAllString(s, "")
	if len(d) < 4 {
		return "***"
	}
	return strings.Repeat("*", len(d)-4) + d[len(d)-4:]
}
