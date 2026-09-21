package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// MachineByCode는 QR 코드로 기계를 찾는다.
//
// 비활성 기계는 ErrNotFound다. 스티커가 떼여 다른 곳에 붙거나 폐기된 기계로
// 신고가 들어오면 받지 않는다 — 존재하지 않는 기계의 환불은 검증할 방법이 없다.
func (s *Store) MachineByCode(ctx context.Context, code string) (domain.Machine, StoreInfo, error) {
	var m domain.Machine
	var st StoreInfo
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.store_id, m.code, m.label, m.location, m.active,
		       s.name, COALESCE(s.phone, '')
		  FROM machines m JOIN stores s ON s.id = m.store_id
		 WHERE m.code = $1 AND m.active`, code,
	).Scan(&m.ID, &m.StoreID, &m.Code, &m.Label, &m.Location, &m.Active, &st.Name, &st.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Machine{}, StoreInfo{}, ErrNotFound
	}
	if err != nil {
		return domain.Machine{}, StoreInfo{}, fmt.Errorf("기계 조회: %w", err)
	}
	return m, st, nil
}

// StoreInfo는 손님 화면에 보여줄 매장 정보다. 기계를 못 찾았을 때
// 손님에게 안내할 연락처가 여기 있다.

type StoreInfo struct {
	Name  string
	Phone string
}

// MachineStat은 기계 한 대의 목록용 요약이다.
type MachineStat struct {
	domain.Machine
	Claims7d   int
	Claims24h  int
	OpenClaims int
	Daily7d    []int // 최근 7일 일별 건수, 오래된 날짜부터
}

// ListMachines는 매장의 기계를 최근 접수 추이와 함께 반환한다.
func (s *Store) ListMachines(ctx context.Context, storeID string) ([]MachineStat, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.store_id, m.code, m.label, m.location, m.active,
		       COUNT(c.id) FILTER (WHERE c.created_at > now() - interval '7 days')  AS c7,
		       COUNT(c.id) FILTER (WHERE c.created_at > now() - interval '24 hours') AS c24,
		       COUNT(c.id) FILTER (WHERE c.status IN ('pending','needs_review','on_hold','approved')) AS open
		  FROM machines m
		  LEFT JOIN claims c ON c.machine_id = m.id
		 WHERE m.store_id = $1
		 GROUP BY m.id
		 ORDER BY c24 DESC, m.label`, storeID)
	if err != nil {
		return nil, fmt.Errorf("기계 목록: %w", err)
	}
	defer rows.Close()

	var out []MachineStat
	for rows.Next() {
		var m MachineStat
		if err := rows.Scan(&m.ID, &m.StoreID, &m.Code, &m.Label, &m.Location, &m.Active,
			&m.Claims7d, &m.Claims24h, &m.OpenClaims); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		daily, err := s.machineDaily7d(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Daily7d = daily
	}
	return out, nil
}

// machineDaily7d는 최근 7일 일별 건수를 반환한다.
// 접수가 없는 날도 0으로 채운다 — 빈 날을 건너뛰면 스파크라인이 거짓말을 한다.
func (s *Store) machineDaily7d(ctx context.Context, machineID string) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d::date, COUNT(c.id)
		  FROM generate_series(now()::date - 6, now()::date, interval '1 day') d
		  LEFT JOIN claims c ON c.machine_id = $1 AND c.created_at::date = d::date
		 GROUP BY d ORDER BY d`, machineID)
	if err != nil {
		return nil, fmt.Errorf("기계 일별 집계: %w", err)
	}
	defer rows.Close()

	out := make([]int, 0, 7)
	for rows.Next() {
		var day any
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CreateMachine은 기계를 추가하고 고유 코드를 붙인다.
// 코드 충돌은 드물지만 가능하므로 몇 번 재시도한다.
func (s *Store) CreateMachine(ctx context.Context, storeID, label, location string) (domain.Machine, error) {
	const attempts = 5
	for i := 0; i < attempts; i++ {
		code, err := domain.GenerateMachineCode()
		if err != nil {
			return domain.Machine{}, err
		}
		m := domain.Machine{StoreID: storeID, Code: code, Label: label, Location: location, Active: true}
		err = s.pool.QueryRow(ctx, `
			INSERT INTO machines (store_id, code, label, location)
			VALUES ($1,$2,$3,$4) RETURNING id`, storeID, code, label, location).Scan(&m.ID)
		if err == nil {
			return m, nil
		}
		if !isUniqueViolation(err) {
			return domain.Machine{}, fmt.Errorf("기계 생성: %w", err)
		}
	}
	return domain.Machine{}, fmt.Errorf("기계 코드 생성에 %d번 실패했습니다", attempts)
}

// UpdateMachine은 라벨·위치·활성 여부를 바꾼다. 코드는 바뀌지 않는다
// (이미 인쇄되어 기계에 붙어 있다).
func (s *Store) UpdateMachine(ctx context.Context, storeID, id, label, location string, active bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE machines SET label=$1, location=$2, active=$3
		 WHERE id=$4 AND store_id=$5`, label, location, active, id, storeID)
	if err != nil {
		return fmt.Errorf("기계 수정: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MachineByID는 매장 범위 안에서 기계를 찾는다.
func (s *Store) MachineByID(ctx context.Context, storeID, id string) (domain.Machine, error) {
	var m domain.Machine
	err := s.pool.QueryRow(ctx, `
		SELECT id, store_id, code, label, location, active
		  FROM machines WHERE id=$1 AND store_id=$2`, id, storeID,
	).Scan(&m.ID, &m.StoreID, &m.Code, &m.Label, &m.Location, &m.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Machine{}, ErrNotFound
	}
	return m, err
}
