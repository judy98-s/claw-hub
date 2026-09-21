package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/domain"
)

// ClaimSummary는 접수함 목록 한 줄이다.
// 개인정보는 마스킹된 형태로만 나간다 — 목록에 계좌번호 전부를 띄울 이유가 없다.
type ClaimSummary struct {
	ID           string              `json:"id"`
	MachineLabel string              `json:"machineLabel"`
	MachineID    string              `json:"machineId"`
	IssueType    domain.IssueType    `json:"issueType"`
	IssueLabel   string              `json:"issueLabel"`
	AmountKRW    int                 `json:"amountKrw"`
	Status       domain.Status       `json:"status"`
	StatusLabel  string              `json:"statusLabel"`
	RiskScore    int                 `json:"riskScore"`
	RiskReasons  []domain.RiskReason `json:"riskReasons"`
	PhoneMasked  string              `json:"phoneMasked"`
	PhotoCount   int                 `json:"photoCount"`
	CreatedAt    time.Time           `json:"createdAt"`
}

// ClaimFilter는 접수함 조회 조건이다.
type ClaimFilter struct {
	StoreID   string
	Statuses  []domain.Status
	MachineID string
	From, To  time.Time
	Cursor    string
	Limit     int
}

const defaultLimit = 50

// ListClaims는 조건에 맞는 신고를 최신순으로 반환한다.
// 두 번째 반환값은 다음 페이지 커서이며, 더 없으면 빈 문자열이다.
func (s *Store) ListClaims(ctx context.Context, f ClaimFilter) ([]ClaimSummary, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultLimit
	}

	// 조건을 붙일 때마다 args에 쌓고 플레이스홀더 번호를 맞춘다.
	var where []string
	var args []any

	args = append(args, f.StoreID)
	where = append(where, fmt.Sprintf("c.store_id = $%d", len(args)))

	if len(f.Statuses) > 0 {
		ss := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			ss[i] = string(st)
		}
		args = append(args, ss)
		where = append(where, fmt.Sprintf("c.status = ANY($%d)", len(args)))
	}
	if f.MachineID != "" {
		args = append(args, f.MachineID)
		where = append(where, fmt.Sprintf("c.machine_id = $%d", len(args)))
	}
	if !f.From.IsZero() {
		args = append(args, f.From)
		where = append(where, fmt.Sprintf("c.created_at >= $%d", len(args)))
	}
	if !f.To.IsZero() {
		args = append(args, f.To)
		where = append(where, fmt.Sprintf("c.created_at < $%d", len(args)))
	}
	if f.Cursor != "" {
		at, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, at, id)
		where = append(where, fmt.Sprintf("(c.created_at, c.id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	// limit+1을 읽어서 다음 페이지 존재 여부를 판단한다.
	args = append(args, limit+1)
	q := fmt.Sprintf(`
		SELECT c.id, m.label, c.machine_id, c.issue_type, c.amount_krw,
		       c.status, c.risk_score, c.risk_reasons, c.phone_enc, c.created_at,
		       (SELECT COUNT(*) FROM claim_photos p WHERE p.claim_id = c.id)
		  FROM claims c JOIN machines m ON m.id = c.machine_id
		 WHERE %s
		 ORDER BY c.created_at DESC, c.id DESC
		 LIMIT $%d`, strings.Join(where, " AND "), len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("신고 목록 조회: %w", err)
	}
	defer rows.Close()

	out := make([]ClaimSummary, 0, limit)
	for rows.Next() {
		var c ClaimSummary
		var issueType, status string
		var reasons, phoneEnc []byte
		if err := rows.Scan(&c.ID, &c.MachineLabel, &c.MachineID, &issueType, &c.AmountKRW,
			&status, &c.RiskScore, &reasons, &phoneEnc, &c.CreatedAt, &c.PhotoCount); err != nil {
			return nil, "", err
		}
		c.IssueType = domain.IssueType(issueType)
		c.IssueLabel = c.IssueType.Label()
		c.Status = domain.Status(status)
		c.StatusLabel = c.Status.Label()
		if err := json.Unmarshal(reasons, &c.RiskReasons); err != nil {
			return nil, "", fmt.Errorf("리스크 사유 파싱: %w", err)
		}
		if c.RiskReasons == nil {
			c.RiskReasons = []domain.RiskReason{}
		}
		phone, err := s.cipher.Decrypt(phoneEnc)
		if err != nil {
			return nil, "", fmt.Errorf("전화번호 복호화: %w", err)
		}
		c.PhoneMasked = crypto.MaskPhone(phone)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(out) > limit {
		out = out[:limit]
		last := out[len(out)-1]
		next = encodeCursor(last.CreatedAt, last.ID)
	}
	return out, next, nil
}

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(c string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("잘못된 커서입니다")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("잘못된 커서입니다")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("잘못된 커서입니다")
	}
	return at, parts[1], nil
}

// RiskQuery는 리스크 평가에 필요한 사실을 모으는 조건이다.
type RiskQuery struct {
	StoreID   string
	MachineID string
	Phone     string // 평문
	Account   string // 평문
	Now       time.Time
}

// RiskFactsFor는 신고 접수 직전에 리스크 판단 근거를 모은다.
//
// 카운터 테이블을 따로 두지 않고 claims를 실시간 집계한다. 이 데이터 규모에서
// 인덱스 스캔은 밀리초 단위이고, 카운터를 두면 정합성이 깨질 여지만 생긴다.
//
// PhoneClaims30d는 "이번 건 포함"이다. 아직 저장 전이므로 기존 건수에 1을 더한다.
func (s *Store) RiskFactsFor(ctx context.Context, q RiskQuery) (domain.RiskInput, error) {
	phoneHash := s.hasher.Hash(q.Phone)
	accountHash := s.hasher.Hash(q.Account)
	since := q.Now.Add(-window30d)

	in := domain.RiskInput{Now: q.Now, ManualStatus: domain.ContactNormal}

	var prior int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount_krw) FILTER (WHERE status='paid'), 0)
		  FROM claims WHERE phone_hash=$1 AND created_at >= $2`, phoneHash, since,
	).Scan(&prior, &in.PhonePaidTotal30d); err != nil {
		return domain.RiskInput{}, fmt.Errorf("번호 이력 집계: %w", err)
	}
	in.PhoneClaims30d = prior + 1

	// 계좌 공유는 기간을 두지 않는다. 한 계좌에 여러 번호가 붙는 건
	// 시간이 지나도 사라지지 않는 신호다.
	var otherPhones int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT phone_hash) FROM claims
		 WHERE account_hash=$1 AND phone_hash <> $2`, accountHash, phoneHash,
	).Scan(&otherPhones); err != nil {
		return domain.RiskInput{}, fmt.Errorf("계좌 공유 집계: %w", err)
	}
	in.AccountDistinctPhones = otherPhones + 1 // 이번 번호 포함

	var last *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT MAX(created_at) FROM claims WHERE phone_hash=$1 AND machine_id=$2`,
		phoneHash, q.MachineID,
	).Scan(&last); err != nil {
		return domain.RiskInput{}, fmt.Errorf("직전 신고 조회: %w", err)
	}
	in.LastSameMachineAt = deref(last)

	// 번호와 계좌 중 더 강한 수동 상태가 이긴다. 번호를 바꿔도 계좌가
	// 차단돼 있으면 잡히고, 그 반대도 마찬가지다.
	status, err := s.strongestContactStatus(ctx, q.StoreID, phoneHash, accountHash)
	if err != nil {
		return domain.RiskInput{}, err
	}
	in.ManualStatus = status
	return in, nil
}

func (s *Store) strongestContactStatus(ctx context.Context, storeID string, phoneHash, accountHash []byte) (string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT manual_status FROM contact_flags
		 WHERE store_id=$1 AND ((identity_type='phone'   AND identity_hash=$2)
		                     OR (identity_type='account' AND identity_hash=$3))`,
		storeID, phoneHash, accountHash)
	if err != nil {
		return "", fmt.Errorf("연락처 상태 조회: %w", err)
	}
	defer rows.Close()

	rank := map[string]int{domain.ContactNormal: 0, domain.ContactWatch: 1, domain.ContactBlocked: 2}
	best := domain.ContactNormal
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return "", err
		}
		if rank[st] > rank[best] {
			best = st
		}
	}
	return best, rows.Err()
}
