// Command setup은 첫 매장과 사장님 계정을 만든다.
//
// 회원가입 화면을 만들지 않았다. 사장님 한 명이 쓰는 시스템에서 공개
// 가입 페이지는 공격 표면만 늘린다. 계정은 서버에서 한 번 만들면 된다.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/judy98-s/claw-hub/internal/app"
)

func main() {
	var (
		storeName  = flag.String("store", "", "매장 이름 (필수)")
		storePhone = flag.String("phone", "", "매장 대표번호 (손님 에러 화면에 안내됨)")
		email      = flag.String("email", "", "사장님 로그인 이메일 (필수)")
		password   = flag.String("password", "", "비밀번호, 8자 이상 (필수)")
		name       = flag.String("name", "사장님", "표시 이름 (로그인 후 바꿀 수 있습니다)")
	)
	flag.Parse()

	if *storeName == "" || *email == "" || *password == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	a, err := app.Build(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "기동 실패:", err)
		os.Exit(1)
	}
	defer a.Close()

	storeID, err := a.Store.CreateStore(ctx, *storeName, *storePhone)
	if err != nil {
		fmt.Fprintln(os.Stderr, "매장 생성 실패:", err)
		os.Exit(1)
	}
	user, err := a.Store.CreateUser(ctx, storeID, *email, *password, *name, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "계정 생성 실패:", err)
		os.Exit(1)
	}

	fmt.Printf("매장 생성 완료\n  매장 ID: %s\n  계정 ID: %s\n  이메일: %s\n\n", storeID, user.ID, user.Email)
	fmt.Println("이제 /admin/login 에서 로그인한 뒤 기계를 등록하고 QR을 인쇄하세요.")
}
