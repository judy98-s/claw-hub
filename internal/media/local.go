package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local은 파일시스템에 저장하는 Storage다.
type Local struct {
	root string
}

func NewLocal(dir string) (*Local, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("저장 경로 확인: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("저장 디렉터리 생성: %w", err)
	}
	return &Local{root: abs}, nil
}

// resolve는 키를 실제 경로로 바꾸면서 루트 밖으로 나가지 않는지 확인한다.
//
// 키는 서버가 만들지만, 저장소가 자기 경계를 스스로 지켜야 한다. 나중에
// 누군가 사용자 입력을 키로 넘기는 순간 이 검사가 유일한 방어선이 된다.
func (l *Local) resolve(key string) (string, error) {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\x00") {
		return "", fmt.Errorf("잘못된 저장 키입니다")
	}
	p := filepath.Join(l.root, filepath.Clean("/"+key))
	if p != l.root && !strings.HasPrefix(p, l.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("저장 경로를 벗어납니다")
	}
	return p, nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ string, size int64) error {
	p, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("디렉터리 생성: %w", err)
	}

	// 임시 파일에 쓴 뒤 rename 한다. 중간에 죽어도 절반만 쓰인 사진이
	// 정상 파일로 보이지 않는다.
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return fmt.Errorf("임시 파일 생성: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // rename 성공 시 이미 없다

	written, err := io.Copy(tmp, r)
	if err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("사진 쓰기: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("임시 파일 닫기: %w", err)
	}
	if size > 0 && written != size {
		return fmt.Errorf("사진 크기가 맞지 않습니다 (%d/%d 바이트)", written, size)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("권한 설정: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		return fmt.Errorf("사진 저장: %w", err)
	}
	return nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	p, err := l.resolve(key)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, "", fmt.Errorf("사진 열기: %w", err)
	}

	head := make([]byte, SniffLen)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close() //nolint:errcheck
		return nil, "", fmt.Errorf("사진 읽기: %w", err)
	}
	ct, err := DetectImageType(head[:n])
	if err != nil {
		ct = "application/octet-stream"
	}
	return f, ct, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("사진 삭제: %w", err)
	}
	return nil
}
