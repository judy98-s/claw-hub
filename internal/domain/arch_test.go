package domain

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestDomain은_다른_내부패키지를_import하지_않는다 는 이 프로젝트의 구조 규칙을
// 코드로 못 박는다.
//
// domain이 store나 httpapi를 참조하기 시작하면, 리스크 규칙을 테스트하려고
// Postgres를 띄워야 한다. 그러면 경계값 전수 테스트가 느려지고, 느려지면
// 결국 안 돌리게 되고, 안 돌리면 돈이 잘못 나간다. 그 연쇄를 여기서 끊는다.
func TestDomain은_다른_내부패키지를_import하지_않는다(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("패키지 파싱: %v", err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.Contains(path, "claw-hub/internal/") {
					t.Errorf("%s 가 내부 패키지를 import 한다: %s", name, path)
				}
			}
		}
	}
}
