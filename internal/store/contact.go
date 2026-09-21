package store

import (
	"context"
	"fmt"
	"time"

	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/domain"
)

// ContactSummary는 연락처 화면 한 줄이다.
// 전화번호는 마스킹되어 나간다 — 목록을 띄우는 것만으로 번호가 새면 안 된다.
type ContactSummary struct {
	IdentityHash string    `json:"identityHash"` // hex. 상태 변경 API의 키로 쓴다
	PhoneMasked  string    `json:"phoneMasked"`
	ClaimCount   int       `json:"claimCount"`
	PaidCount    int       `json:"paidCount"`
	PaidTotalKRW int       `json:"paidTotalKrw"`
	ManualStatus string    `json:"manualStatus"`
	Note         string    `json:"note"`
	LastClaimAt  time.Time `json:"lastClaimAt"`
}

// ListContacts는 신고 이력이 있는 연락처를 건수 내림차순으로 반환한다.
// 사장님이 "누가 제일 많이 신고했나"를 한눈에 보는 화면이다.
func (s *Store) ListContacts(ctx context.Context, storeID string) ([]ContactSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.phone_hash,
		       (ARRAY_AGG(c.phone_enc ORDER BY c.created_at DESC))[1] AS latest_phone,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE c.status='paid'),
		       COALESCE(SUM(COALESCE(c.paid_amount_krw, c.amount_krw)) FILTER (WHERE c.status='paid'), 0),
		       MAX(c.created_at),
		       COALESCE(f.manual_status, $2),
		       COALESCE(f.note, '')
		  FROM claims c
		  LEFT JOIN contact_flags f
		         ON f.identity_hash = c.phone_hash
		        AND f.identity_type = 'phone'
		        AND f.store_id = c.store_id
		 WHERE c.store_id = $1
		 GROUP BY c.phone_hash, f.manual_status, f.note
		 ORDER BY COUNT(*) DESC, MAX(c.created_at) DESC`, storeID, domain.ContactNormal)
	if err != nil {
		return nil, fmt.Errorf("연락처 목록: %w", err)
	}
	defer rows.Close()

	out := []ContactSummary{}
	for rows.Next() {
		var c ContactSummary
		var hash, phoneEnc []byte
		if err := rows.Scan(&hash, &phoneEnc, &c.ClaimCount, &c.PaidCount,
			&c.PaidTotalKRW, &c.LastClaimAt, &c.ManualStatus, &c.Note); err != nil {
			return nil, err
		}
		phone, err := s.cipher.Decrypt(phoneEnc)
		if err != nil {
			return nil, fmt.Errorf("전화번호 복호화: %w", err)
		}
		c.IdentityHash = fmt.Sprintf("%x", hash)
		c.PhoneMasked = crypto.MaskPhone(phone)
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetContactStatus는 연락처의 수동 상태를 바꾼다.
func (s *Store) SetContactStatus(ctx context.Context, storeID string, hash []byte, kind, status, note, by string) error {
	if _, err := domain.ParseContactStatus(status); err != nil {
		return err
	}
	if kind != "phone" && kind != "account" {
		return fmt.Errorf("알 수 없는 연락처 종류: %q", kind)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contact_flags (identity_hash, identity_type, store_id, manual_status, note, updated_by, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6, now())
		ON CONFLICT (identity_hash, identity_type, store_id)
		DO UPDATE SET manual_status=$4, note=$5, updated_by=$6, updated_at=now()`,
		hash, kind, storeID, status, note, nilUUID(by))
	if err != nil {
		return fmt.Errorf("연락처 상태 변경: %w", err)
	}
	return nil
}
