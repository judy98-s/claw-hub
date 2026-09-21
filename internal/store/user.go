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

// User는 사장님 계정이다.
type User struct {
	ID      string
	StoreID string
	Email   string
	Name    string
}

// maxLoginFailures와 lockDuration은 무차별 대입 속도를 떨어뜨린다.
// 완전히 막는 게 아니라 비현실적으로 느리게 만드는 게 목적이다.
const (
	maxLoginFailures = 5
	lockDuration     = 15 * time.Minute
)

// CreateUser는 사장님 계정을 만든다.
func (s *Store) CreateUser(ctx context.Context, storeID, email, password, name string) (User, error) {
	if len(password) < 8 {
		return User{}, fmt.Errorf("비밀번호는 8자 이상이어야 합니다")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("비밀번호 해싱: %w", err)
	}
	u := User{StoreID: storeID, Email: email, Name: name}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (store_id, email, password_hash, name)
		VALUES ($1,$2,$3,$4) RETURNING id`, storeID, email, string(hash), name).Scan(&u.ID)
	if isUniqueViolation(err) {
		return User{}, fmt.Errorf("이미 등록된 이메일입니다")
	}
	if err != nil {
		return User{}, fmt.Errorf("계정 생성: %w", err)
	}
	return u, nil
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
		SELECT id, store_id, email, name, password_hash, failed_logins, locked_until
		  FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.StoreID, &u.Email, &u.Name, &hash, &failed, &lockedUntil)
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
		`SELECT id, store_id, email, name FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.StoreID, &u.Email, &u.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
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
