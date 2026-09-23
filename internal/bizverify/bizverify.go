// Package bizverify는 사업자등록번호가 쓸 만한 번호인지 확인한다.
//
// 인터페이스로 둔 이유: 국세청 사업자등록 상태조회 API가 공공데이터포털에
// 무료로 있지만(1일 100만건) 활용신청과 승인이 필요하다(3시간~3일). 매장
// 계정으로 신청해야 하는 것이라 지금은 붙일 수 없다.
//
// 그래서 1단계는 Checksum 하나로 간다. 키를 받으면 NTS 구현을 파일 하나
// 추가해서 갈아끼운다. 도메인 코드는 건드리지 않는다.
package bizverify

import (
	"context"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// Result는 확인 결과다.
type Result struct {
	// OK는 이 번호를 받아도 되는지다.
	OK bool
	// Checked는 실제로 존재하는 사업자인지까지 확인했는지다.
	//
	// 이 칸이 있는 이유: false 인데 OK 인 상태가 정상이고, 화면은 그
	// 차이를 사장님에게 있는 그대로 말해야 한다. "확인했습니다"와
	// "형식이 맞습니다"는 다른 말이다.
	Checked bool
	// Message는 사장님이 읽는 한 줄이다.
	Message string
}

// Verifier는 사업자등록번호 확인 경로다.
type Verifier interface {
	Verify(ctx context.Context, bizNo string) (Result, error)
}

// Checksum은 외부 호출 없이 검증 규칙만 본다.
//
// 오타와 아무렇게나 적은 숫자를 잡는다. 실존 여부는 모른다 — 그리고
// 모른다고 말한다.
type Checksum struct{}

func NewChecksum() Checksum { return Checksum{} }

func (Checksum) Verify(_ context.Context, bizNo string) (Result, error) {
	if err := domain.ValidateBizNo(bizNo); err != nil {
		return Result{OK: false, Message: err.Error()}, nil
	}
	return Result{
		OK:      true,
		Checked: false,
		Message: "형식이 맞습니다. 실제 사업자 여부는 확인하지 않았습니다.",
	}, nil
}
