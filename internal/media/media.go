// Package media는 손님이 첨부한 사진을 저장한다.
//
// 1단계는 로컬 디스크다. 사진 I/O가 실제로 아파지면 Storage 인터페이스
// 뒤에 S3/R2 구현을 넣는다 — 호출자는 바뀌지 않는다.
package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// Storage는 사진 한 장의 수명주기다.
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, contentType string, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

// 허용하는 이미지 타입. HEIC는 아이폰 기본 포맷이라 빼면 아이폰 사용자가
// 사진을 못 올린다.
const (
	TypeJPEG = "image/jpeg"
	TypePNG  = "image/png"
	TypeWebP = "image/webp"
	TypeHEIC = "image/heic"
)

// magicPrefixes는 파일 앞부분의 매직바이트다.
var magicPrefixes = []struct {
	prefix []byte
	typ    string
}{
	{[]byte{0xFF, 0xD8, 0xFF}, TypeJPEG},
	{[]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, TypePNG},
}

// SniffLen은 타입 판정에 필요한 바이트 수다.
const SniffLen = 32

// DetectImageType은 파일 내용으로 이미지 타입을 판정한다.
//
// 확장자나 클라이언트가 보낸 Content-Type을 믿지 않는다. 둘 다 손님이
// 마음대로 정할 수 있고, .jpg 로 올라온 실행 파일을 그대로 저장하면
// 나중에 사장님이 그 링크를 여는 순간이 문제가 된다.
func DetectImageType(head []byte) (string, error) {
	for _, m := range magicPrefixes {
		if bytes.HasPrefix(head, m.prefix) {
			return m.typ, nil
		}
	}
	// WebP: "RIFF" + 4바이트 크기 + "WEBP"
	if len(head) >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")) {
		return TypeWebP, nil
	}
	// HEIC: 4바이트 박스 크기 + "ftyp" + 브랜드
	if len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")) {
		switch string(head[8:12]) {
		case "heic", "heix", "hevc", "mif1", "msf1":
			return TypeHEIC, nil
		}
	}
	return "", fmt.Errorf("이미지 파일이 아닙니다 (jpg, png, webp, heic 만 가능)")
}

// ExtFor는 타입에 맞는 확장자를 준다.
func ExtFor(contentType string) string {
	switch contentType {
	case TypePNG:
		return ".png"
	case TypeWebP:
		return ".webp"
	case TypeHEIC:
		return ".heic"
	default:
		return ".jpg"
	}
}
