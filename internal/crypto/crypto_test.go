package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func key(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func newCipher(t *testing.T) *Cipher {
	t.Helper()
	c, err := NewCipher(key(7))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

func TestCipher_왕복(t *testing.T) {
	c := newCipher(t)
	for _, plain := range []string{
		"01012345678",
		"110-123-456789",
		"김민수",
		"",
		strings.Repeat("가", 500),
	} {
		ct, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		got, err := c.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plain {
			t.Errorf("왕복 실패: got %q, want %q", got, plain)
		}
	}
}

func TestCipher_같은평문도_매번_다른_암호문(t *testing.T) {
	// nonce가 고정이면 같은 계좌번호가 같은 바이트로 저장되어,
	// DB만 봐도 "이 두 건은 같은 계좌"가 드러난다.
	c := newCipher(t)
	a, _ := c.Encrypt("110-123-456789")
	b, _ := c.Encrypt("110-123-456789")
	if bytes.Equal(a, b) {
		t.Fatal("같은 평문이 같은 암호문으로 나온다 — nonce가 재사용되고 있다")
	}
}

func TestCipher_변조된_암호문은_복호화_실패(t *testing.T) {
	c := newCipher(t)
	ct, _ := c.Encrypt("110-123-456789")
	ct[len(ct)-1] ^= 0xFF
	if _, err := c.Decrypt(ct); err == nil {
		t.Fatal("변조된 암호문이 복호화됐다 — 인증 태그가 검증되지 않는다")
	}
}

func TestCipher_너무짧은_암호문(t *testing.T) {
	c := newCipher(t)
	if _, err := c.Decrypt([]byte{1, 2, 3}); err == nil {
		t.Fatal("nonce보다 짧은 입력이 복호화됐다")
	}
	if _, err := c.Decrypt(nil); err == nil {
		t.Fatal("nil 입력이 복호화됐다")
	}
}

func TestCipher_다른키로는_복호화_실패(t *testing.T) {
	c1 := newCipher(t)
	c2, _ := NewCipher(key(8))
	ct, _ := c1.Encrypt("110-123-456789")
	if _, err := c2.Decrypt(ct); err == nil {
		t.Fatal("다른 키로 복호화가 됐다")
	}
}

func TestNewCipher_키길이_검증(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := NewCipher(make([]byte, n)); err == nil {
			t.Errorf("%d바이트 키가 통과됐다", n)
		}
	}
}

func TestHasher_결정적이고_pepper에_의존한다(t *testing.T) {
	h1, err := NewHasher(key(1))
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := NewHasher(key(2))

	a := h1.Hash("01012345678")
	b := h1.Hash("01012345678")
	if !bytes.Equal(a, b) {
		t.Fatal("같은 입력이 다른 해시를 낸다 — 조회 키로 쓸 수 없다")
	}
	if len(a) != 32 {
		t.Errorf("해시 길이 = %d, want 32", len(a))
	}
	if bytes.Equal(a, h2.Hash("01012345678")) {
		t.Fatal("pepper가 달라도 같은 해시가 나온다 — pepper가 적용되지 않았다")
	}
	if bytes.Equal(a, h1.Hash("01012345679")) {
		t.Fatal("다른 입력이 같은 해시를 낸다")
	}
}

func TestNewHasher_pepper길이_검증(t *testing.T) {
	for _, n := range []int{0, 16, 31} {
		if _, err := NewHasher(make([]byte, n)); err == nil {
			t.Errorf("%d바이트 pepper가 통과됐다", n)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	ok := []struct{ in, want string }{
		{"010-1234-5678", "01012345678"},
		{"01012345678", "01012345678"},
		{"+82 10-1234-5678", "01012345678"},
		{"+821012345678", "01012345678"},
		{"010 1234 5678", "01012345678"},
		{"0111234567", "0111234567"},   // 구 011 번호 10자리
		{"01112345678", "01112345678"}, // 011 11자리
	}
	for _, tc := range ok {
		got, err := NormalizePhone(tc.in)
		if err != nil {
			t.Errorf("NormalizePhone(%q) 에러: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	bad := []string{
		"",
		"0212345678",   // 유선 (서울)
		"0312345678",   // 유선 (경기)
		"010123456",    // 너무 짧음
		"010123456789", // 너무 김
		"abc",
		"010-1234-567a",
		"15881588", // 대표번호
	}
	for _, in := range bad {
		if got, err := NormalizePhone(in); err == nil {
			t.Errorf("NormalizePhone(%q) 가 %q 로 통과됐다 — 거부해야 한다", in, got)
		}
	}
}

func TestNormalizeAccount(t *testing.T) {
	tests := []struct{ in, want string }{
		{"110-123-456789", "110123456789"},
		{"110 123 456789", "110123456789"},
		{"110123456789", "110123456789"},
		{" 3333-01-1234567 ", "3333011234567"},
	}
	for _, tc := range tests {
		if got := NormalizeAccount(tc.in); got != tc.want {
			t.Errorf("NormalizeAccount(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct{ in, want string }{
		{"01012345678", "010-****-5678"},
		{"010-1234-5678", "010-****-5678"},
		{"0111234567", "011-***-4567"},
		{"", "***"},
		{"abc", "***"},
	}
	for _, tc := range tests {
		if got := MaskPhone(tc.in); got != tc.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskAccount(t *testing.T) {
	tests := []struct{ in, want string }{
		{"110123456789", "********6789"},
		{"110-123-456789", "********6789"},
		{"1234", "1234"}, // 4자리 이하는 마스킹할 여지가 없다
		{"123", "***"},
		{"", "***"},
	}
	for _, tc := range tests {
		if got := MaskAccount(tc.in); got != tc.want {
			t.Errorf("MaskAccount(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMask_원본을_유추할_정보를_남기지_않는다(t *testing.T) {
	// 마스킹 결과에 가운데 자리 숫자가 그대로 들어 있으면 마스킹이 아니다.
	if strings.Contains(MaskPhone("01098765432"), "9876") {
		t.Error("MaskPhone이 가운데 자리를 노출한다")
	}
	if strings.Contains(MaskAccount("110987654321"), "9876") {
		t.Error("MaskAccount가 앞자리를 노출한다")
	}
}
