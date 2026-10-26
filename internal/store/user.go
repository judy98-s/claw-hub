package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/judy98-s/claw-hub/internal/domain"
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
	if utf8.RuneCountInString(password) < 8 {
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

// SetPassword는 비밀번호를 바꾼다.
//
// 화면에는 이 경로가 없고 서버에서만 부른다. 사장님이 비밀번호를 잊었을 때
// /setup -reset-password 로 되돌리는 길이다. 화면에 두려면 현재 비밀번호
// 확인과 세션 무효화가 함께 필요한데, 그건 별도 작업이다.
//
// 잠금(failed_logins, locked_until)도 같이 푼다. 비밀번호를 바꾼 뒤에도
// 15분을 기다려야 하면 바꾼 보람이 없다.
func (s *Store) SetPassword(ctx context.Context, email, password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return fmt.Errorf("비밀번호는 8자 이상이어야 합니다")
	}
	// bcrypt 는 72바이트를 넘으면 잘라 버린다. 조용히 자르면 사용자가 믿는
	// 비밀번호와 저장된 것이 달라진다. 여기는 글자 수가 아니라 바이트가
	// 맞다 — 잘리는 단위가 바이트다. 한글은 글자당 3바이트라 24자가 상한.
	if len(password) > 72 {
		return fmt.Errorf("비밀번호는 72자 이하여야 합니다")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("비밀번호 해싱: %w", err)
	}
	// 가입과 로그인은 이메일을 소문자로 내려 저장하고 비교한다. 여기서만
	// 날것으로 비교하면 대문자가 섞인 입력이 조용히 "찾을 수 없습니다"가
	// 되고, 원인이 비밀번호 쪽에 있는 줄 알고 한참 헤매게 된다.
	email = strings.TrimSpace(strings.ToLower(email))
	tag, err := s.pool.Exec(ctx, `
		UPDATE users
		   SET password_hash=$1, failed_logins=0, locked_until=NULL
		 WHERE email=$2`, string(hash), email)
	if err != nil {
		return fmt.Errorf("비밀번호 변경: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
	// Type은 업종이다. 손님이 보는 것 거의 전부가 이 값으로 갈린다.
	Type domain.StoreType

	// 송금 설정. 사장님이 화면에서 정한다.
	PayoutProvider string
	PayoutTemplate string
	PayoutBankCode string
	// PayoutAccount는 사장님이 돈을 보내는 계좌다. 평문으로 오간다.
	// 딥링크에 넣지 않는다 — 송금 앱은 출금 계좌를 URL 로 받지 않는다.
	PayoutAccount string

	// 장터 설정. 둘 다 있어야 글을 쓸 수 있다.
	//
	// BizNo 는 암호화하지 않는다. 개인정보가 아니라 공개된 사업자
	// 식별자이고, 거래 상대가 확인할 수 있어야 하는 값이다.
	// 전화번호·계좌와 성격이 반대다.
	RegionCode   string
	RegionDetail string
	BizNo        string
}

// CanPostListing은 장터에 글을 쓸 수 있는 상태인지 알려준다.
func (d StoreDetail) CanPostListing() bool {
	return d.RegionCode != "" && d.BizNo != ""
}

// StoreByID는 매장 정보를 읽는다.
func (s *Store) StoreByID(ctx context.Context, id string) (StoreDetail, error) {
	var d StoreDetail
	var accountEnc []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(phone,''), store_type,
		       payout_provider, payout_template, payout_bank_code, payout_account_enc,
		       region_code, region_detail, biz_no
		  FROM stores WHERE id=$1`, id,
	).Scan(&d.ID, &d.Name, &d.Phone, &d.Type,
		&d.PayoutProvider, &d.PayoutTemplate, &d.PayoutBankCode, &accountEnc,
		&d.RegionCode, &d.RegionDetail, &d.BizNo)
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

// StoreProfile은 사장님이 매장 설정 화면에서 바꾸는 값들이다.
type StoreProfile struct {
	Name  string
	Phone string
	// Type은 업종이다. 빈 문자열이면 바꾸지 않는다 — 장터 설정만 고치는
	// 화면이 업종을 기본값으로 덮어쓰지 않게 한다.
	Type domain.StoreType
	// 아래 셋은 장터용이다. 빈 문자열이면 "아직 안 적음"이고,
	// 그 상태에서는 글을 쓸 수 없다.
	RegionCode   string
	RegionDetail string
	BizNo        string
}

// UpdateStore는 매장 설정을 바꾼다.
// 대표번호는 손님이 기계를 못 찾았을 때 안내되는 번호다.
func (s *Store) UpdateStore(ctx context.Context, id string, in StoreProfile) (StoreDetail, error) {
	// 업종은 빈 문자열이면 건드리지 않는다. 장터 설정만 고치는 화면이
	// 업종을 기본값으로 덮어쓰지 않게 COALESCE 대신 NULLIF 로 받는다.
	tag, err := s.pool.Exec(ctx, `
		UPDATE stores SET name=$1, phone=$2,
		       region_code=$3, region_detail=$4, biz_no=$5,
		       store_type = COALESCE(NULLIF($6,''), store_type)
		 WHERE id=$7`,
		in.Name, in.Phone, in.RegionCode, in.RegionDetail, in.BizNo,
		string(in.Type), id)
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

// CreateStoreWithOwner는 매장과 첫 계정을 **한 트랜잭션으로** 만든다.
//
// 따로 만들면 안 된다. 매장을 먼저 만들고 계정 생성이 실패하면 — 중복
// 이메일이 가장 흔하다 — 주인 없는 빈 매장이 남는다. setup 은 돌릴 때마다
// 매장을 새로 만들기 때문에 실패할수록 쓰레기가 쌓이고, 치우려면 Postgres
// 에 직접 SQL 을 쳐야 한다. 실제로 겪었다.
//
// 비밀번호 검증과 해싱은 트랜잭션 **밖에서** 한다. bcrypt 는 100ms 가까이
// 걸리는데 그동안 트랜잭션을 열어 둘 이유가 없다.
func (s *Store) CreateStoreWithOwner(ctx context.Context, storeName, storePhone string, storeType domain.StoreType, email, password, name string) (string, User, error) {
	if utf8.RuneCountInString(password) < 8 {
		return "", User{}, fmt.Errorf("비밀번호는 8자 이상이어야 합니다")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", User{}, fmt.Errorf("비밀번호 해싱: %w", err)
	}

	var (
		storeID string
		u       = User{Email: email, Name: name, Active: true}
	)
	err = s.tx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO stores (name, phone, store_type) VALUES ($1,$2,$3) RETURNING id`,
			storeName, storePhone, string(domain.NormalizeStoreType(string(storeType)))).Scan(&storeID); err != nil {
			return fmt.Errorf("매장 생성: %w", err)
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO users (store_id, email, password_hash, name, phone)
			VALUES ($1,$2,$3,$4,'') RETURNING id`,
			storeID, email, string(hash), name).Scan(&u.ID)
		if isUniqueViolation(err) {
			// 롤백되므로 매장도 남지 않는다.
			return fmt.Errorf("이미 등록된 이메일입니다: %s", email)
		}
		if err != nil {
			return fmt.Errorf("계정 생성: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", User{}, err
	}
	u.StoreID = storeID
	return storeID, u, nil
}

// AdminUserRow는 /setup -list-users 가 보여 주는 한 줄이다.
//
// 비밀번호를 바꾸려는데 "찾을 수 없습니다"가 나올 때, 서버에 psql 을 치지
// 않고도 실제로 등록된 이메일을 확인할 수 있어야 한다. 해시는 담지 않는다.
type AdminUserRow struct {
	Email       string
	Name        string
	StoreName   string
	LockedUntil *time.Time
	CreatedAt   time.Time
}

// ListAllUsers는 모든 매장의 계정을 만든 순서로 돌려준다. 서버에서 직접
// 돌리는 관리 명령 전용이다. HTTP 핸들러에서 쓰면 매장 격리가 깨진다.
func (s *Store) ListAllUsers(ctx context.Context) ([]AdminUserRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.email, u.name, st.name, u.locked_until, u.created_at
		  FROM users u JOIN stores st ON st.id = u.store_id
		 ORDER BY u.created_at`)
	if err != nil {
		return nil, fmt.Errorf("계정 목록: %w", err)
	}
	defer rows.Close()

	var out []AdminUserRow
	for rows.Next() {
		var r AdminUserRow
		if err := rows.Scan(&r.Email, &r.Name, &r.StoreName, &r.LockedUntil, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("계정 목록 읽기: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
