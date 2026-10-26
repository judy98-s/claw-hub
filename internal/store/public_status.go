package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// PublicStatus는 손님이 접수번호로 조회할 수 있는 것 전부다.
//
// 전화번호·계좌·예금주·카드 뒷자리·설명·리스크 점수는 **없다**. 손님이
// 자기 정보를 다시 볼 이유가 없고, 이 응답은 중간 캐시와 로그에 남는다.
// 조회에 필요한 건 "어디까지 진행됐나" 뿐이다.
type PublicStatus struct {
	Status        domain.Status
	IssueType     domain.IssueType
	AmountKRW     int
	PaymentMethod domain.PaymentMethod
	CreatedAt     time.Time
	// ResolvedAt 은 사장님이 승인·거절한 시각, PaidAt 은 송금·취소 시각이다.
	// 둘 다 zero 면 아직 손도 안 댄 것이다.
	ResolvedAt time.Time
	PaidAt     time.Time
	StoreName  string
	StorePhone string
	StoreType  domain.StoreType
}

// PublicStatusByReceipt는 접수번호와 기계 코드로 처리 상태를 찾는다.
//
// 기계 코드를 함께 받는 이유는 두 가지다.
//
//  1. 접수번호는 UUID 앞 8자(32비트)라서 유일하지 않다. 7만 건쯤에서
//     충돌 확률이 50% 를 넘는다. 기계로 좁히면 사실상 사라진다.
//  2. 접수번호만으로 조회되면 32비트 공간을 긁어 다른 매장 손님의
//     금액과 상태를 모을 수 있다.
//
// 손님은 QR 을 찍고 들어왔으므로 기계 코드를 이미 갖고 있다. 받는 비용이
// 0 인데 두 문제가 같이 닫힌다.
//
// 그래도 충돌이 나면(같은 기계에 앞 8자가 같은 두 건) ErrNotFound 다.
// 남의 건을 보여주는 것보다 못 찾는 게 낫다.
func (s *Store) PublicStatusByReceipt(ctx context.Context, machineCode, receiptCode string) (PublicStatus, error) {
	prefix := strings.ToLower(strings.TrimSpace(receiptCode))
	if len(prefix) != 8 {
		return PublicStatus{}, ErrNotFound
	}
	for _, c := range prefix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return PublicStatus{}, ErrNotFound
		}
	}

	var p PublicStatus
	var status, issueType, paymentMethod, storeType string
	var resolvedAt, paidAt *time.Time

	// LIMIT 2 로 뽑아 두 건이면 거절한다. LIMIT 1 이면 충돌을 모르고
	// 아무거나 돌려준다.
	rows, err := s.pool.Query(ctx, `
		SELECT c.status, c.issue_type, c.amount_krw, c.payment_method,
		       c.created_at, c.resolved_at, c.paid_at,
		       st.name, COALESCE(st.phone, ''), st.store_type
		  FROM claims c
		  JOIN machines m  ON m.id = c.machine_id
		  JOIN stores   st ON st.id = c.store_id
		 WHERE m.code = $1
		   AND substr(replace(c.id::text, '-', ''), 1, 8) = $2
		 LIMIT 2`, machineCode, prefix)
	if err != nil {
		return PublicStatus{}, fmt.Errorf("접수 상태 조회: %w", err)
	}
	defer rows.Close()

	found := 0
	for rows.Next() {
		found++
		if found > 1 {
			return PublicStatus{}, ErrNotFound
		}
		if err := rows.Scan(&status, &issueType, &p.AmountKRW, &paymentMethod,
			&p.CreatedAt, &resolvedAt, &paidAt,
			&p.StoreName, &p.StorePhone, &storeType); err != nil {
			return PublicStatus{}, fmt.Errorf("접수 상태 읽기: %w", err)
		}
	}
	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return PublicStatus{}, fmt.Errorf("접수 상태 조회: %w", err)
	}
	if found != 1 {
		return PublicStatus{}, ErrNotFound
	}

	p.Status = domain.Status(status)
	p.IssueType = domain.IssueType(issueType)
	p.PaymentMethod = domain.PaymentMethod(paymentMethod)
	p.StoreType = domain.NormalizeStoreType(storeType)
	p.ResolvedAt = deref(resolvedAt)
	p.PaidAt = deref(paidAt)
	return p, nil
}
