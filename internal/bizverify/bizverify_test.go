package bizverify

import (
	"context"
	"strings"
	"testing"
)

func TestChecksum_유효한_번호(t *testing.T) {
	res, err := NewChecksum().Verify(context.Background(), "220-81-62517")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("OK = false: %s", res.Message)
	}
	// 실존 확인을 했다고 말하면 안 된다. 그 거짓말 위에 사장님이
	// 거래 판단을 하게 된다.
	if res.Checked {
		t.Error("Checked = true — 체크섬만 봤는데 확인했다고 말하고 있다")
	}
	if !strings.Contains(res.Message, "확인하지 않았") {
		t.Errorf("한계를 숨기는 문구다: %q", res.Message)
	}
}

func TestChecksum_틀린_번호는_에러가_아니라_결과로_돌아온다(t *testing.T) {
	// 사장님이 오타를 낸 건 시스템 오류가 아니다. 에러로 던지면 호출부가
	// 500 을 내게 되고, 사장님은 "서버 문제"라고 읽는다.
	res, err := NewChecksum().Verify(context.Background(), "220-81-62518")
	if err != nil {
		t.Fatalf("오타에 에러를 던졌다: %v", err)
	}
	if res.OK {
		t.Error("틀린 번호가 통과했다")
	}
	if res.Message == "" {
		t.Error("사장님이 읽을 메시지가 없다")
	}
}
