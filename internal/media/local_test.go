package media

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x11}, 40)...)

func newLocal(t *testing.T) (*Local, string) {
	t.Helper()
	dir := t.TempDir()
	l, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	return l, dir
}

func TestLocal_왕복(t *testing.T) {
	ctx := context.Background()
	l, _ := newLocal(t)

	key := "claims/abc/1.jpg"
	if err := l.Put(ctx, key, bytes.NewReader(jpegBytes), TypeJPEG, int64(len(jpegBytes))); err != nil {
		t.Fatal(err)
	}
	rc, ct, err := l.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, jpegBytes) {
		t.Error("읽은 내용이 쓴 내용과 다르다")
	}
	if ct != TypeJPEG {
		t.Errorf("ContentType = %q, want %q", ct, TypeJPEG)
	}
}

func TestLocal_Delete(t *testing.T) {
	ctx := context.Background()
	l, _ := newLocal(t)
	key := "claims/abc/1.jpg"

	if err := l.Put(ctx, key, bytes.NewReader(jpegBytes), TypeJPEG, 0); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.Open(ctx, key); err == nil {
		t.Error("삭제된 사진이 열렸다")
	}
	// 없는 파일 삭제는 에러가 아니다 — 워커가 재시도할 때 로그만 채운다.
	if err := l.Delete(ctx, key); err != nil {
		t.Errorf("없는 파일 삭제가 에러다: %v", err)
	}
}

func TestLocal_경로탈출_차단(t *testing.T) {
	ctx := context.Background()
	l, dir := newLocal(t)

	canary := filepath.Join(filepath.Dir(dir), "canary.txt")
	if err := os.WriteFile(canary, []byte("원본"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"../canary.txt",
		"../../canary.txt",
		"claims/../../canary.txt",
		"/etc/passwd",
		"a/../../canary.txt",
	} {
		err := l.Put(ctx, key, bytes.NewReader(jpegBytes), TypeJPEG, 0)
		// Clean 이 루트 안으로 접어 넣는 경우는 통과할 수 있다.
		// 중요한 건 루트 밖 파일이 손상되지 않는 것이다.
		_ = err
	}

	content, err := os.ReadFile(canary)
	if err != nil {
		t.Fatalf("루트 밖 파일이 사라졌다: %v", err)
	}
	if string(content) != "원본" {
		t.Error("루트 밖 파일이 덮어쓰였다")
	}
}

func TestLocal_빈키_거부(t *testing.T) {
	ctx := context.Background()
	l, _ := newLocal(t)
	for _, key := range []string{"", "/abs.jpg", "a\x00b.jpg"} {
		if err := l.Put(ctx, key, bytes.NewReader(jpegBytes), TypeJPEG, 0); err == nil {
			t.Errorf("키 %q 가 통과됐다", key)
		}
	}
}

func TestLocal_크기가_다르면_실패(t *testing.T) {
	ctx := context.Background()
	l, _ := newLocal(t)
	err := l.Put(ctx, "a.jpg", bytes.NewReader(jpegBytes), TypeJPEG, int64(len(jpegBytes))+100)
	if err == nil {
		t.Fatal("선언한 크기와 다른데 통과됐다")
	}
	if !strings.Contains(err.Error(), "크기") {
		t.Errorf("에러가 원인을 말해주지 않는다: %v", err)
	}
}

func TestLocal_부분쓰기가_정상파일로_남지_않는다(t *testing.T) {
	ctx := context.Background()
	l, dir := newLocal(t)

	// 중간에 실패하는 리더
	r := io.MultiReader(bytes.NewReader(jpegBytes[:4]), errReader{})
	if err := l.Put(ctx, "a.jpg", r, TypeJPEG, 0); err == nil {
		t.Fatal("실패하는 리더인데 Put이 성공했다")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.jpg")); !os.IsNotExist(err) {
		t.Error("절반만 쓰인 파일이 남았다")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestDetectImageType(t *testing.T) {
	ok := []struct {
		name string
		head []byte
		want string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, TypeJPEG},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0}, TypePNG},
		{"webp", append([]byte("RIFF\x00\x00\x00\x00WEBP"), 0, 0), TypeWebP},
		{"heic", append([]byte("\x00\x00\x00\x18ftypheic"), 0, 0), TypeHEIC},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectImageType(tc.head)
			if err != nil {
				t.Fatalf("에러: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}

	// Review Focus #5. .jpg 로 위장한 비이미지 파일.
	bad := []struct {
		name string
		head []byte
	}{
		{"텍스트", []byte("안녕하세요 이건 사진이 아닙니다")},
		{"ELF 실행파일", []byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0}},
		{"PDF", []byte("%PDF-1.7\n%\xE2\xE3\xCF\xD3")},
		{"ZIP", []byte{'P', 'K', 0x03, 0x04, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"빈 파일", []byte{}},
		{"너무 짧음", []byte{0xFF, 0xD8}},
		{"HTML", []byte("<html><script>alert(1)</script>")},
		{"SVG (스크립트 실행 가능)", []byte("<svg xmlns=\"http://www.w3.org/2000/svg\">")},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := DetectImageType(tc.head); err == nil {
				t.Errorf("%q 로 통과됐다 — 확장자를 믿으면 안 된다", got)
			}
		})
	}
}

func TestExtFor(t *testing.T) {
	tests := map[string]string{
		TypeJPEG: ".jpg", TypePNG: ".png", TypeWebP: ".webp",
		TypeHEIC: ".heic", "알 수 없음": ".jpg",
	}
	for ct, want := range tests {
		if got := ExtFor(ct); got != want {
			t.Errorf("ExtFor(%q) = %q, want %q", ct, got, want)
		}
	}
}
