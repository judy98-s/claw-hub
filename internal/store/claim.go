package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// PhotoInput은 이미 저장소에 기록된 사진 한 장이다.
type PhotoInput struct {
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

// CreateClaimInput은 신고 접수에 필요한 전부다. 개인정보는 평문으로 받는다.
type CreateClaimInput struct {
	StoreID   string
	MachineID string

	IssueType   domain.IssueType
	AmountKRW   int
	Description string

	Phone    string // 평문. 저장 직전에 암호화된다
	BankCode string
	Account  string
	Holder   string

	Status      domain.Status
	RiskScore   int
	RiskReasons []domain.RiskReason

	IdempotencyKey string
	IP             string
	UserAgent      string

	Photos []PhotoInput
}

// CreateClaimResult는 접수 결과다.
type CreateClaimResult struct {
	ID        string
	Status    domain.Status
	CreatedAt time.Time
	// Existing이 true면 같은 멱등키의 기존 건을 그대로 돌려준 것이다.
	// 새 claim은 만들어지지 않았다.
	Existing bool
}

// CreateClaim은 신고 건과 사진, 생성 이벤트를 한 트랜잭션으로 저장한다.
//
// 같은 멱등키가 이미 있으면 기존 건을 Existing=true 로 돌려준다. 모바일
// 네트워크에서 제출 버튼이 두 번 눌리거나 재시도가 일어나도 환불이 두 번
// 나가면 안 된다. 캐시 기반 멱등성은 경합에서 뚫릴 수 있으므로, 실제 방어는
// claims(store_id, idempotency_key) 유니크 제약이 한다.
func (s *Store) CreateClaim(ctx context.Context, in CreateClaimInput) (CreateClaimResult, error) {
	phone, err := crypt3(s, in.Phone)
	if err != nil {
		return CreateClaimResult{}, err
	}
	account, err := crypt3(s, in.Account)
	if err != nil {
		return CreateClaimResult{}, err
	}
	holderEnc, err := s.cipher.Encrypt(in.Holder)
	if err != nil {
		return CreateClaimResult{}, fmt.Errorf("예금주 암호화: %w", err)
	}
	reasons, err := json.Marshal(orEmpty(in.RiskReasons))
	if err != nil {
		return CreateClaimResult{}, fmt.Errorf("리스크 사유 직렬화: %w", err)
	}
	var ipHash []byte
	if in.IP != "" {
		ipHash = s.hasher.Hash(in.IP)
	}

	// ON CONFLICT DO NOTHING 이 행을 안 돌려주면 경합에서 진 것이다. 이긴
	// 트랜잭션이 커밋될 때까지 기다렸다가 그 행을 읽는다. 커밋 직후 짧은
	// 창에서 아직 안 보일 수 있어 한 번 재시도한다.
	for attempt := 0; attempt < 2; attempt++ {
		var res CreateClaimResult
		err := s.tx(ctx, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, `
				INSERT INTO claims (
					store_id, machine_id, issue_type, amount_krw, description,
					phone_enc, phone_hash, bank_code, account_enc, account_hash, holder_enc,
					status, risk_score, risk_reasons, idempotency_key, ip_hash, user_agent
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
				ON CONFLICT (store_id, idempotency_key) DO NOTHING
				RETURNING id, status, created_at`,
				in.StoreID, in.MachineID, string(in.IssueType), in.AmountKRW, in.Description,
				phone.enc, phone.hash, in.BankCode, account.enc, account.hash, holderEnc,
				string(in.Status), in.RiskScore, reasons, in.IdempotencyKey, ipHash, in.UserAgent)

			var status string
			if err := row.Scan(&res.ID, &status, &res.CreatedAt); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrDuplicate
				}
				return fmt.Errorf("신고 저장: %w", err)
			}
			res.Status = domain.Status(status)

			for _, p := range in.Photos {
				if _, err := tx.Exec(ctx, `
					INSERT INTO claim_photos (claim_id, object_key, content_type, size_bytes, expires_at)
					VALUES ($1,$2,$3,$4,$5)`,
					res.ID, p.ObjectKey, p.ContentType, p.SizeBytes, p.ExpiresAt); err != nil {
					return fmt.Errorf("사진 저장: %w", err)
				}
			}

			ev := domain.NewEvent(res.ID, domain.Actor{Kind: domain.ActorCustomer},
				domain.ActionCreate, "", res.Status, "")
			return insertEvent(ctx, tx, ev)
		})

		if err == nil {
			return res, nil
		}
		if !errors.Is(err, ErrDuplicate) {
			return CreateClaimResult{}, err
		}

		existing, found, err := s.claimByIdempotencyKey(ctx, in.StoreID, in.IdempotencyKey)
		if err != nil {
			return CreateClaimResult{}, err
		}
		if found {
			existing.Existing = true
			return existing, nil
		}
		// 경합 상대가 아직 커밋 전이다. 한 번 더 돈다.
	}
	return CreateClaimResult{}, ErrDuplicate
}

// ClaimByIdempotencyKey는 멱등키로 기존 접수를 찾는다.
//
// 접수 핸들러가 리스크 평가보다 먼저 이걸 본다. 재시도를 리스크 규칙이
// 먼저 잡으면, 네트워크가 끊겨 다시 보낸 손님이 접수번호 대신 에러를
// 받는다. 멱등키가 존재하는 이유가 바로 그 상황이다.
func (s *Store) ClaimByIdempotencyKey(ctx context.Context, storeID, key string) (CreateClaimResult, bool, error) {
	return s.claimByIdempotencyKey(ctx, storeID, key)
}

func (s *Store) claimByIdempotencyKey(ctx context.Context, storeID, key string) (CreateClaimResult, bool, error) {
	var res CreateClaimResult
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, created_at FROM claims
		 WHERE store_id=$1 AND idempotency_key=$2`, storeID, key,
	).Scan(&res.ID, &status, &res.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateClaimResult{}, false, nil
	}
	if err != nil {
		return CreateClaimResult{}, false, fmt.Errorf("멱등키 조회: %w", err)
	}
	res.Status = domain.Status(status)
	return res, true, nil
}

// Photo는 저장된 사진 한 장이다.
type Photo struct {
	ID          string
	ClaimID     string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

// ClaimDetail은 사장님 상세 화면에 필요한 전부다.
// 개인정보가 복호화되어 담기므로 조회 자체가 감사 대상이다.
type ClaimDetail struct {
	domain.Claim
	MachineLabel string
	MachineCode  string
	RiskReasons  []domain.RiskReason

	Phone    string // 복호화된 평문
	BankCode string
	Account  string
	Holder   string

	Photos []Photo
	Events []domain.Event

	// 이 연락처의 과거 이력. 사장님이 승인 전에 보는 맥락이다.
	PhoneClaims30d    int
	PhonePaidTotal30d int
}

// ClaimByID는 신고 건 상세를 반환하고 열람 사실을 감사 로그에 남긴다.
//
// storeID로 범위를 제한한다. 남의 매장 건은 ErrNotFound다 — 403을 주면
// "그 ID는 존재한다"는 사실이 새어 나간다.
func (s *Store) ClaimByID(ctx context.Context, storeID, id string, by domain.Actor) (ClaimDetail, error) {
	var d ClaimDetail
	var issueType, status string
	var reasons []byte
	var phoneEnc, accountEnc, holderEnc []byte
	var phoneHash []byte
	var resolvedAt, paidAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.store_id, c.machine_id, c.issue_type, c.amount_krw, c.description,
		       c.status, c.risk_score, c.risk_reasons,
		       c.phone_enc, c.phone_hash, c.bank_code, c.account_enc, c.holder_enc,
		       c.created_at, c.resolved_at, c.paid_at, c.payout_method,
		       m.label, m.code
		  FROM claims c JOIN machines m ON m.id = c.machine_id
		 WHERE c.id=$1 AND c.store_id=$2`, id, storeID,
	).Scan(&d.ID, &d.StoreID, &d.MachineID, &issueType, &d.AmountKRW, &d.Description,
		&status, &d.RiskScore, &reasons,
		&phoneEnc, &phoneHash, &d.BankCode, &accountEnc, &holderEnc,
		&d.CreatedAt, &resolvedAt, &paidAt, &d.PayoutMethod,
		&d.MachineLabel, &d.MachineCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClaimDetail{}, ErrNotFound
	}
	if err != nil {
		return ClaimDetail{}, fmt.Errorf("신고 상세 조회: %w", err)
	}

	d.IssueType = domain.IssueType(issueType)
	d.Status = domain.Status(status)
	d.ResolvedAt = deref(resolvedAt)
	d.PaidAt = deref(paidAt)
	if err := json.Unmarshal(reasons, &d.RiskReasons); err != nil {
		return ClaimDetail{}, fmt.Errorf("리스크 사유 파싱: %w", err)
	}

	if d.Phone, err = s.cipher.Decrypt(phoneEnc); err != nil {
		return ClaimDetail{}, fmt.Errorf("전화번호 복호화: %w", err)
	}
	if d.Account, err = s.cipher.Decrypt(accountEnc); err != nil {
		return ClaimDetail{}, fmt.Errorf("계좌 복호화: %w", err)
	}
	if d.Holder, err = s.cipher.Decrypt(holderEnc); err != nil {
		return ClaimDetail{}, fmt.Errorf("예금주 복호화: %w", err)
	}

	if d.Photos, err = s.photosFor(ctx, id); err != nil {
		return ClaimDetail{}, err
	}
	if d.Events, err = s.eventsFor(ctx, id); err != nil {
		return ClaimDetail{}, err
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(amount_krw) FILTER (WHERE status='paid'), 0)
		  FROM claims
		 WHERE phone_hash=$1 AND created_at > now() - interval '30 days'`, phoneHash,
	).Scan(&d.PhoneClaims30d, &d.PhonePaidTotal30d); err != nil {
		return ClaimDetail{}, fmt.Errorf("연락처 이력 집계: %w", err)
	}

	// 개인정보 열람은 그 자체로 감사 대상이다. 실패해도 조회는 성공시킨다 —
	// 로그를 못 남겼다고 사장님이 일을 못 하면 안 된다.
	if by.Kind != "" {
		_ = s.AppendEvent(ctx, domain.NewEvent(id, by, domain.ActionViewContact, "", "", "계좌·연락처 열람"))
	}
	return d, nil
}

func (s *Store) photosFor(ctx context.Context, claimID string) ([]Photo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, claim_id, object_key, content_type, size_bytes, expires_at
		  FROM claim_photos WHERE claim_id=$1 ORDER BY created_at`, claimID)
	if err != nil {
		return nil, fmt.Errorf("사진 조회: %w", err)
	}
	defer rows.Close()
	var out []Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.ClaimID, &p.ObjectKey, &p.ContentType, &p.SizeBytes, &p.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) eventsFor(ctx context.Context, claimID string) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT claim_id, actor_kind, actor_id, action, from_status, to_status, note, created_at
		  FROM claim_events WHERE claim_id=$1 ORDER BY created_at, id`, claimID)
	if err != nil {
		return nil, fmt.Errorf("이벤트 조회: %w", err)
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var e domain.Event
		var from, to string
		if err := rows.Scan(&e.ClaimID, &e.Actor.Kind, &e.Actor.ID, &e.Action, &from, &to, &e.Note, &e.At); err != nil {
			return nil, err
		}
		e.From, e.To = domain.Status(from), domain.Status(to)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ApplyTransition은 상태 변경과 감사 이벤트를 한 트랜잭션으로 기록한다.
//
// 둘이 갈라지면 "언제 승인됐는지 모르는 승인 건"이 생긴다. 돈이 오가는
// 기록에서 그건 허용할 수 없다.
func (s *Store) ApplyTransition(ctx context.Context, storeID string, c *domain.Claim, ev domain.Event) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE claims
			   SET status=$1,
			       resolved_at = CASE WHEN $2::timestamptz IS NULL THEN resolved_at ELSE $2 END,
			       paid_at     = CASE WHEN $3::timestamptz IS NULL THEN paid_at     ELSE $3 END,
			       payout_method = CASE WHEN $4 = '' THEN payout_method ELSE $4 END,
			       resolved_by = COALESCE($5, resolved_by)
			 WHERE id=$6 AND store_id=$7 AND status=$8`,
			string(c.Status), nilTime(c.ResolvedAt), nilTime(c.PaidAt), c.PayoutMethod,
			nilUUID(ev.Actor.ID), c.ID, storeID, string(ev.From))
		if err != nil {
			return fmt.Errorf("상태 변경: %w", err)
		}
		if tag.RowsAffected() == 0 {
			// status=$8 조건이 어긋났다 = 그 사이 누가 먼저 바꿨다.
			return ErrNotFound
		}
		return insertEvent(ctx, tx, ev)
	})
}

// AppendEvent는 상태 변경 없는 감사 이벤트를 남긴다.
func (s *Store) AppendEvent(ctx context.Context, ev domain.Event) error {
	return insertEvent(ctx, s.pool, ev)
}

// execer는 연결 풀과 트랜잭션 양쪽을 받기 위한 최소 인터페이스다.
// 감사 이벤트는 단독으로도, 상태 변경과 같은 트랜잭션 안에서도 기록된다.
type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertEvent(ctx context.Context, q execer, ev domain.Event) error {
	_, err := q.Exec(ctx, `
		INSERT INTO claim_events (claim_id, actor_kind, actor_id, action, from_status, to_status, note, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		ev.ClaimID, ev.Actor.Kind, ev.Actor.ID, ev.Action,
		string(ev.From), string(ev.To), ev.Note, ev.At)
	if err != nil {
		return fmt.Errorf("감사 이벤트 기록: %w", err)
	}
	return nil
}

type encPair struct{ enc, hash []byte }

// crypt3은 평문 하나를 암호문과 조회용 해시로 만든다.
func crypt3(s *Store, plain string) (encPair, error) {
	enc, err := s.cipher.Encrypt(plain)
	if err != nil {
		return encPair{}, fmt.Errorf("암호화: %w", err)
	}
	return encPair{enc: enc, hash: s.hasher.Hash(plain)}, nil
}

func orEmpty(r []domain.RiskReason) []domain.RiskReason {
	if r == nil {
		return []domain.RiskReason{}
	}
	return r
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func nilTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nilUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
