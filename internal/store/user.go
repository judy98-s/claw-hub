package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrLocked는 로그인 시도가 너무 많아 잠긴 계정이다.
var ErrLocked = errors.New("로그인 시도가 너무 많습니다. 잠시 후 다시 시도해주세요")

// User는 매장 계정이다. 사장님과 직원이 같은 구조를 쓴다.
//
// 권한 구분을 두지 않았다. 한 매장에 두세 명이 쓰는 시스템에서 역할 체계는
// 관리 비용만 늘린다. 필요해지면 그때 넣는다.
type User struct {
	ID      string
	StoreID string
	Email   string
	Name    string
	Phone   string
	Active  bool
}

// maxLoginFailures와 lockDuration은 무차별 대입 속도를 떨어뜨린다.
// 완전히 막는 게 아니라 비현실적으로 느리게 만드는 게 목적이다.
const (
	maxLoginFailures = 5
	lockDuration     = 15 * time.Minute
)

// CreateUser는 매장 계정을 만든다.
func (s *Store) CreateUser(ctx context.Context, storeID, email, password, name, phone string) (User, error) {
	if len(password) < 8 {
		return User{}, fmt.Errorf("비밀번호는 8자 이상이어야 합니다")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("비밀번호 해싱: %w", err)
	}
	u := User{StoreID: storeID, Email: email, Name: name, Phone: phone, Active: true}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (store_id, email, password_hash, name, phone)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		storeID, email, string(hash), name, phone).Scan(&u.ID)
	if isUniqueViolation(err) {
		return User{}, fmt.Errorf("이미 등록된 이메일입니다")
	}
	if err != nil {
		return User{}, fmt.Errorf("계정 생성: %w", err)
	}
	return u, nil
}

// UpdateUser는 이름과 연락처를 바꾼다. 이메일과 비밀번호는 건드리지 않는다.
func (s *Store) UpdateUser(ctx context.Context, storeID, userID, name, phone string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		UPDATE users SET name=$1, phone=$2
		 WHERE id=$3 AND store_id=$4
		 RETURNING id, store_id, email, name, phone, active`,
		name, phone, userID, storeID,
	).Scan(&u.ID, &u.StoreID, &u.Email, &u.Name, &u.Phone, &u.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("계정 수정: %w", err)
	}
	return u, nil
}

// ListUsers는 매장의 계정을 반환한다. 비활성 계정도 포함한다.
func (s *Store) ListUsers(ctx context.Context, storeID string) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, store_id, email, name, phone, active
		  FROM users WHERE store_id=$1 ORDER BY active DESC, created_at`, storeID)
	if err != nil {
		return nil, fmt.Errorf("계정 목록: %w", err)
	}
	defer rows.Close()

	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.StoreID, &u.Email, &u.Name, &u.Phone, &u.Active); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserActive는 계정을 켜고 끈다.
//
// 지우지 않고 끄는 이유: claim_events 와 claims.resolved_by 가 이 계정을
// 가리키고 있다. 지우면 "누가 승인했나"를 나중에 따질 수 없다.
//
// 매장의 마지막 활성 계정은 끌 수 없다. 끄면 아무도 로그인하지 못한다.
func (s *Store) SetUserActive(ctx context.Context, storeID, userID string, active bool) error {
	if !active {
		var remaining int
		if err := s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM users
			 WHERE store_id=$1 AND active AND id <> $2`, storeID, userID,
		).Scan(&remaining); err != nil {
			return fmt.Errorf("활성 계정 수 확인: %w", err)
		}
		if remaining == 0 {
			return fmt.Errorf("마지막 계정은 비활성화할 수 없습니다")
		}
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET active=$1 WHERE id=$2 AND store_id=$3`, active, userID, storeID)
	if err != nil {
		return fmt.Errorf("계정 상태 변경: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Authenticate는 이메일과 비밀번호를 검증한다.
//
// 이메일이 없을 때와 비밀번호가 틀렸을 때 같은 에러를 반환한다. 구분해서
// 알려주면 "이 이메일은 가입되어 있다"는 사실이 새어 나간다.
func (s *Store) Authenticate(ctx context.Context, email, password string) (User, error) {
	var u User
	var hash string
	var failed int
	var lockedUntil *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, store_id, email, name, phone, active, password_hash, failed_logins, locked_until
		  FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.StoreID, &u.Email, &u.Name, &u.Phone, &u.Active, &hash, &failed, &lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		// 존재하지 않는 계정도 bcrypt 한 번 분량의 시간을 쓴다.
		// 응답 시간 차이로 가입 여부를 알아내는 걸 막는다.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("계정 조회: %w", err)
	}

	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		return User{}, ErrLocked
	}
	if !u.Active {
		// 그만둔 직원. 비밀번호가 맞아도 들어올 수 없다.
		// 계정이 없는 것과 같은 에러를 준다 — 구분하면 재직 여부가 샌다.
		return User{}, ErrNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if err := s.recordLoginFailure(ctx, u.ID, failed+1); err != nil {
			return User{}, err
		}
		return User{}, ErrNotFound
	}

	if failed > 0 {
		if _, err := s.pool.Exec(ctx,
			`UPDATE users SET failed_logins=0, locked_until=NULL WHERE id=$1`, u.ID); err != nil {
			return User{}, fmt.Errorf("로그인 실패 기록 초기화: %w", err)
		}
	}
	return u, nil
}

// dummyHash는 존재하지 않는 계정에서도 bcrypt 비용을 치르기 위한 값이다.
var dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

func (s *Store) recordLoginFailure(ctx context.Context, userID string, count int) error {
	var lockedUntil *time.Time
	if count >= maxLoginFailures {
		t := time.Now().Add(lockDuration)
		lockedUntil = &t
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE users SET failed_logins=$1, locked_until=$2 WHERE id=$3`,
		count, lockedUntil, userID); err != nil {
		return fmt.Errorf("로그인 실패 기록: %w", err)
	}
	return nil
}

// UserByID는 세션 검증에 쓴다.
func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, store_id, email, name, phone, active FROM users WHERE id=$1 AND active`, id,
	).Scan(&u.ID, &u.StoreID, &u.Email, &u.Name, &u.Phone, &u.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		// 비활성화된 계정의 세션 쿠키는 그 즉시 무효가 된다.
		return User{}, ErrNotFound
	}
	return u, err
}

// StoreDetail은 매장 정보다.
type StoreDetail struct {
	ID    string
	Name  string
	Phone string

	// 송금 설정. 사장님이 화면에서 정한다.
	PayoutProvider string
	PayoutTemplate string
	PayoutBankCode string
	// PayoutAccount는 사장님이 돈을 보내는 계좌다. 평문으로 오간다.
	// 딥링크에 넣지 않는다 — 송금 앱은 출금 계좌를 URL 로 받지 않는다.
	PayoutAccount string
}

// StoreByID는 매장 정보를 읽는다.
func (s *Store) StoreByID(ctx context.Context, id string) (StoreDetail, error) {
	var d StoreDetail
	var accountEnc []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(phone,''),
		       payout_provider, payout_template, payout_bank_code, payout_account_enc
		  FROM stores WHERE id=$1`, id,
	).Scan(&d.ID, &d.Name, &d.Phone,
		&d.PayoutProvider, &d.PayoutTemplate, &d.PayoutBankCode, &accountEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoreDetail{}, ErrNotFound
	}
	if err != nil {
		return StoreDetail{}, fmt.Errorf("매장 조회: %w", err)
	}
	if len(accountEnc) > 0 {
		if d.PayoutAccount, err = s.cipher.Decrypt(accountEnc); err != nil {
			return StoreDetail{}, fmt.Errorf("출금 계좌 복호화: %w", err)
		}
	}
	return d, nil
}

// UpdateStore는 매장 이름과 대표번호를 바꾼다.
// 대표번호는 손님이 기계를 못 찾았을 때 안내되는 번호다.
func (s *Store) UpdateStore(ctx context.Context, id, name, phone string) (StoreDetail, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE stores SET name=$1, phone=$2 WHERE id=$3`, name, phone, id)
	if err != nil {
		return StoreDetail{}, fmt.Errorf("매장 수정: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return StoreDetail{}, ErrNotFound
	}
	return s.StoreByID(ctx, id)
}

// PayoutSettings는 사장님이 고른 송금 방식이다.
type PayoutSettings struct {
	Provider string
	Template string
	BankCode string
	Account  string // 평문
}

// UpdatePayoutSettings는 송금 설정을 저장한다.
//
// 출금 계좌는 암호화해서 넣는다. 사장님 계좌도 개인정보이고, 손님 계좌와
// 같은 기준으로 다루지 않을 이유가 없다.
func (s *Store) UpdatePayoutSettings(ctx context.Context, id string, in PayoutSettings) (StoreDetail, error) {
	var accountEnc []byte
	if in.Account != "" {
		enc, err := s.cipher.Encrypt(in.Account)
		if err != nil {
			return StoreDetail{}, fmt.Errorf("출금 계좌 암호화: %w", err)
		}
		accountEnc = enc
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE stores
		   SET payout_provider=$1, payout_template=$2,
		       payout_bank_code=$3, payout_account_enc=$4
		 WHERE id=$5`, in.Provider, in.Template, in.BankCode, accountEnc, id)
	if err != nil {
		return StoreDetail{}, fmt.Errorf("송금 설정 저장: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return StoreDetail{}, ErrNotFound
	}
	return s.StoreByID(ctx, id)
}

// CreateStore는 매장을 만든다. 초기 설정에서 쓴다.
func (s *Store) CreateStore(ctx context.Context, name, phone string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO stores (name, phone) VALUES ($1,$2) RETURNING id`, name, phone).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("매장 생성: %w", err)
	}
	return id, nil
}
