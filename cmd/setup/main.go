// Command setup은 첫 매장과 사장님 계정을 만들고, 비밀번호를 되돌린다.
//
// 회원가입 화면을 만들지 않았다. 사장님 한 명이 쓰는 시스템에서 공개
// 가입 페이지는 공격 표면만 늘린다. 계정은 서버에서 한 번 만들면 된다.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/judy98-s/claw-hub/internal/app"
	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

func main() {
	var (
		storeName  = flag.String("store", "", "매장 이름 (필수)")
		storePhone = flag.String("phone", "", "매장 대표번호 (손님 에러 화면에 안내됨)")
		email      = flag.String("email", "", "사장님 로그인 이메일 (필수)")
		password   = flag.String("password", "", "비밀번호, 8자 이상 (필수)")
		name       = flag.String("name", "사장님", "표시 이름 (로그인 후 바꿀 수 있습니다)")
		storeType  = flag.String("type", "claw", "업종: claw|laundry|karaoke|photo|study|cafe")
		reset      = flag.Bool("reset-password", false, "비밀번호만 바꾼다 (-email, -password 필요)")
		listUsers  = flag.Bool("list-users", false, "등록된 계정 이메일을 보여준다")
		pwStdin    = flag.Bool("password-stdin", false, "비밀번호를 표준입력에서 읽는다 (-password 대신)")
	)
	flag.Parse()

	if *listUsers {
		listAllUsers()
		return
	}

	// -password 로 넘기면 셸 히스토리와 ps 출력에 평문으로 남는다. 그리고
	// 안내문의 자리표시("새비밀번호" 같은 것)를 그대로 붙여 실행하면
	// 그게 진짜 비밀번호가 되어 버린다. 실제로 한 번 겪은 길이다.
	// -password-stdin 은 둘 다 막는다.
	if *pwStdin {
		if *password != "" {
			fmt.Fprintln(os.Stderr, "-password 와 -password-stdin 을 함께 쓸 수 없습니다.")
			os.Exit(2)
		}
		pw, err := readPasswordStdin()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		*password = pw
	}

	if *reset {
		if *email == "" || *password == "" {
			fmt.Fprintln(os.Stderr, "-reset-password 에는 -email 과, -password 또는 -password-stdin 이 필요합니다.")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "히스토리에 남기지 않으려면:")
			fmt.Fprintln(os.Stderr, `  echo -n "$PW" | /setup -reset-password -email ... -password-stdin`)
			os.Exit(2)
		}
		resetPassword(*email, *password)
		return
	}

	if *storeName == "" || *email == "" || *password == "" {
		flag.Usage()
		os.Exit(2)
	}
	// 업종은 손님 화면 거의 전부를 가른다. 오타를 조용히 기본값으로
	// 떨어뜨리면 왜 다른 캐릭터가 나오는지 찾기 어렵다.
	if !domain.ValidStoreType(*storeType) {
		fmt.Fprintf(os.Stderr, "-type 이 올바르지 않습니다: %q\n", *storeType)
		fmt.Fprintln(os.Stderr, "쓸 수 있는 값: claw laundry karaoke photo study cafe")
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
	storeID, user, err := a.Store.CreateStoreWithOwner(ctx, *storeName, *storePhone,
		domain.StoreType(*storeType), *email, *password, *name)
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

// resetPassword는 비밀번호만 바꾼다. 매장도 계정도 만들지 않는다.
//
// 화면에 변경 경로가 없어서 남겨 둔 길이다. 비밀번호를 잊었을 때
// /setup 을 다시 돌리면 **새 매장**이 생기고 기존 기계와 민원은 첫 매장에
// 남아 접근할 수 없게 된다. 그 사고를 막으려고 따로 뒀다.
func resetPassword(email, password string) {
	ctx := context.Background()
	a, err := app.Build(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "기동 실패:", err)
		os.Exit(1)
	}
	defer a.Close()

	if err := a.Store.SetPassword(ctx, email, password); err != nil {
		fmt.Fprintln(os.Stderr, "변경 실패:", err)
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "%s 로 등록된 계정이 없습니다. 등록된 이메일을 확인하세요:\n", email)
			fmt.Fprintln(os.Stderr, "  /setup -list-users")
		}
		os.Exit(1)
	}
	fmt.Printf("%s 의 비밀번호를 바꿨습니다.\n", email)
	fmt.Println("로그인 실패 잠금도 함께 풀었습니다.")
}

// listAllUsers는 등록된 계정 이메일을 보여 준다.
//
// 비밀번호를 바꾸려는데 이메일이 안 맞으면 막힌다. psql 을 치게 만들지
// 않는다. 비밀번호 해시는 출력하지 않는다.
func listAllUsers() {
	ctx := context.Background()
	a, err := app.Build(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "기동 실패:", err)
		os.Exit(1)
	}
	defer a.Close()

	rows, err := a.Store.ListAllUsers(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "조회 실패:", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Println("등록된 계정이 없습니다. /setup -store ... -email ... -password ... 로 먼저 만드세요.")
		return
	}
	for _, r := range rows {
		// active=false 면 비밀번호가 맞아도 로그인이 401 로 막힌다. 그런데
		// 에러 메시지는 "이메일 또는 비밀번호가 올바르지 않습니다" 뿐이라,
		// 여기서 보여주지 않으면 비밀번호만 계속 바꾸게 된다.
		flags := ""
		if !r.Active {
			flags += "  [비활성 — 로그인 불가]"
		}
		if r.LockedUntil != nil && r.LockedUntil.After(time.Now()) {
			flags += fmt.Sprintf("  [잠김: %s 까지]", r.LockedUntil.Local().Format("01-02 15:04"))
		}
		fmt.Printf("%-32s  %-12s  %s%s\n", r.Email, r.Name, r.StoreName, flags)
	}
}

// readPasswordStdin은 표준입력 전체를 비밀번호로 읽는다.
//
// 끝의 줄바꿈 하나만 떼어낸다. 앞뒤 공백은 남긴다 — 비밀번호에 일부러
// 넣은 공백을 조용히 지우면, 사용자가 믿는 비밀번호로 로그인이 안 된다.
func readPasswordStdin() (string, error) {
	b, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
	if err != nil {
		return "", fmt.Errorf("표준입력 읽기: %w", err)
	}
	pw := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
	if pw == "" {
		return "", errors.New(`표준입력이 비었습니다. echo -n "$PW" | ... 형태로 넘기세요.`)
	}
	return pw, nil
}
