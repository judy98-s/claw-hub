// Package store는 Postgres 영속화를 담당한다.
//
// 이 패키지가 개인정보의 암복호화 경계다. 호출자는 평문을 넘기고 평문을
// 돌려받으며, 평문이 디스크에 닿는 일은 없다.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/judy98-s/claw-hub/internal/crypto"
)

//go:embed all:migrations
var migrationFS embed.FS

var (
	ErrNotFound  = errors.New("찾을 수 없습니다")
	ErrDuplicate = errors.New("이미 접수된 요청입니다")
)

// Store는 DB 연결과 암호화 도구를 함께 들고 있다.
type Store struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
	hasher *crypto.Hasher
}

// Open은 연결 풀을 만든다.
func Open(ctx context.Context, dsn string, c *crypto.Cipher, h *crypto.Hasher) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL 파싱: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("DB 연결: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("DB 응답 없음: %w", err)
	}
	return &Store{pool: pool, cipher: c, hasher: h}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Pool은 통합 테스트에서 직접 쿼리할 때만 쓴다.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Migrate는 아직 적용되지 않은 마이그레이션을 번호순으로 적용한다.
// 여러 번 실행해도 안전하다.
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("마이그레이션 테이블 생성: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("마이그레이션 파일 읽기: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("%s 적용 여부 확인: %w", name, err)
		}
		if exists {
			continue
		}

		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("%s 읽기: %w", name, err)
		}

		// 마이그레이션 적용과 기록은 한 트랜잭션이어야 한다. 중간에 죽으면
		// 절반만 적용된 스키마가 "적용됨"으로 남는다.
		err = s.tx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("%s 실행: %w", name, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// tx는 함수를 트랜잭션 안에서 실행하고, 에러가 나면 롤백한다.
func (s *Store) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // 커밋 후 롤백은 무해하다

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// isUniqueViolation은 유니크 제약 위반인지 본다.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// window30d는 반복 신고 카운트의 관찰 기간이다.
const window30d = 30 * 24 * time.Hour
