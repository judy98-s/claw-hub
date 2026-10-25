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

	// 매장과 계정은 한 트랜잭션으로 만든다. 따로 만들면 계정 생성이
	// 실패했을 때 주인 없는 빈 매장이 남는데, 이 명령은 돌릴 때마다 매장을
	// 새로 만들기 때문에 실패할수록 쓰레기가 쌓인다.
	storeID, user, err := a.Store.CreateStoreWithOwner(ctx, *storeName, *storePhone, *email, *password, *name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "생성 실패:", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "매장은 만들어지지 않았다 (트랜잭션이 롤백된다).")
		fmt.Fprintln(os.Stderr, "이미 등록된 이메일이면 그 계정으로 로그인하거나,")
		fmt.Fprintln(os.Stderr, "다른 이메일을 쓰거나, 기존 매장을 지우고 다시 만든다.")
		os.Exit(1)
	}

	fmt.Printf("매장 생성 완료\n  매장 ID: %s\n  계정 ID: %s\n  이메일: %s\n\n", storeID, user.ID, user.Email)
	fmt.Println("이제 /admin/login 에서 로그인한 뒤 기계를 등록하고 QR을 인쇄하세요.")
	fmt.Println()
	fmt.Println("비밀번호와 이메일은 화면에서 바꿀 수 없습니다. 비밀번호 관리자에 저장해 두세요.")
}
