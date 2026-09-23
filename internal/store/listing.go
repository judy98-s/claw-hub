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

// Listing은 장터 글 하나다.
//
// 전화번호 칸이 없다. 의도적이다 — 목록이든 상세든 이 구조체로 나가는 한
// 번호가 실수로 새어 나갈 길이 없다. 번호는 ContactFor 만 돌려준다.
type Listing struct {
	ID        string             `json:"id"`
	StoreName string             `json:"storeName"`
	Name      string             `json:"name"`
	NameKey   string             `json:"nameKey"`
	Kind      domain.ListingKind `json:"kind"`
	KindLabel string             `json:"kindLabel"`

	Qty          int    `json:"qty"`
	UnitPriceKRW int    `json:"unitPriceKrw"`
	Note         string `json:"note"`

	RegionCode   string `json:"regionCode"`
	RegionName   string `json:"regionName"`
	RegionDetail string `json:"regionDetail"`

	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`

	// Mine은 보는 사람의 매장 글인지다. 자기 글에는 전화 버튼이 뜨면 안 된다.
	Mine bool `json:"mine"`
	// ReportCount는 내 글에서만 채운다. 남의 글의 신고 수를 보여줄 이유가 없다.
	ReportCount int `json:"reportCount"`
}

// CreateListingInput은 글 등록 입력이다.
type CreateListingInput struct {
	StoreID string
	UserID  string

	Name         string
	Kind         domain.ListingKind
	Qty          int
	UnitPriceKRW int
	Note         string

	// 게시 시점의 지역. 호출부가 매장 설정에서 읽어 넘긴다.
	RegionCode   string
	RegionDetail string

	Now time.Time
}

// CreateListing은 장터 글을 올린다.
func (s *Store) CreateListing(ctx context.Context, in CreateListingInput) (Listing, error) {
	// 교환만 원하는 글의 가격은 0으로 못 박는다. 화면이 실수로 값을
	// 보내도 목록에서 "0원짜리 제일 싼 글"로 올라오지 않게 한다.
	price := in.UnitPriceKRW
	if !in.Kind.NeedsPrice() {
		price = 0
	}

	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO listings (
			store_id, created_by, name, name_key, kind, qty, unit_price_krw, note,
			region_code, region_detail, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		in.StoreID, nilUUID(in.UserID), strings.TrimSpace(in.Name), domain.NameKey(in.Name),
		string(in.Kind), in.Qty, price, strings.TrimSpace(in.Note),
		in.RegionCode, in.RegionDetail,
		in.Now.AddDate(0, 0, domain.ListingDays),
	).Scan(&id)
	if err != nil {
		return Listing{}, fmt.Errorf("장터 글 저장: %w", err)
	}
	return s.ListingByID(ctx, id, in.StoreID, in.Now)
}

// ListingFilter는 장터 목록 조회 조건이다.
type ListingFilter struct {
	// ViewerStoreID는 보는 사람의 매장이다. Mine 을 채우는 데 쓴다.
	ViewerStoreID string
	RegionCode    string
	Kind          domain.ListingKind
	// MineOnly면 내 글만 본다. 내린 글과 만료된 글도 함께 나온다 —
	// 내 글 관리 화면에서는 그것도 봐야 한다.
	MineOnly bool
	Now      time.Time
	Limit    int
}

const listingSelect = `
	SELECT l.id, COALESCE(st.name, ''), l.name, l.name_key, l.kind,
	       l.qty, l.unit_price_krw, l.note,
	       l.region_code, l.region_detail,
	       l.status, l.created_at, l.expires_at, l.report_count,
	       l.store_id
	  FROM listings l JOIN stores st ON st.id = l.store_id`

// ListListings는 장터 목록을 최신순으로 반환한다.
//
// 만료·삭제된 글은 빠진다. 죽은 글이 쌓인 장터는 안 여는 장터가 된다.
func (s *Store) ListListings(ctx context.Context, f ListingFilter) ([]Listing, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var where []string
	var args []any

	if f.MineOnly {
		args = append(args, f.ViewerStoreID)
		// 내 글 관리에서는 내린 글도 보여준다. 다만 운영자가 내린
		// 글(removed)은 목록에서 빼지 않는다 — 왜 사라졌는지 알아야 한다.
		where = append(where, fmt.Sprintf("l.store_id = $%d", len(args)))
	} else {
		args = append(args, f.Now)
		where = append(where, "l.status = 'open'", fmt.Sprintf("l.expires_at > $%d", len(args)))

		if f.RegionCode != "" {
			args = append(args, f.RegionCode)
			where = append(where, fmt.Sprintf("l.region_code = $%d", len(args)))
		}
		if f.Kind != "" {
			// "판매"를 고르면 판매 전용과 "둘 다"가 같이 나와야 한다.
			// 둘 다 파는 사람이니까.
			args = append(args, string(f.Kind))
			where = append(where, fmt.Sprintf("(l.kind = $%d OR l.kind = 'both')", len(args)))
		}
	}

	args = append(args, limit)
	q := fmt.Sprintf("%s WHERE %s ORDER BY l.created_at DESC LIMIT $%d",
		listingSelect, strings.Join(where, " AND "), len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("장터 목록 조회: %w", err)
	}
	defer rows.Close()
	return scanListings(rows, f.ViewerStoreID)
}

// ListingByID는 글 하나를 읽는다.
//
// 만료·삭제된 글은 ErrNotFound 다. 단, 자기 글이면 보인다 — 내가 올린
// 글이 왜 안 보이는지 확인할 길은 있어야 한다.
func (s *Store) ListingByID(ctx context.Context, id, viewerStoreID string, now time.Time) (Listing, error) {
	rows, err := s.pool.Query(ctx, listingSelect+` WHERE l.id = $1`, id)
	if err != nil {
		return Listing{}, fmt.Errorf("장터 글 조회: %w", err)
	}
	defer rows.Close()

	out, err := scanListings(rows, viewerStoreID)
	if err != nil {
		return Listing{}, err
	}
	if len(out) == 0 {
		return Listing{}, ErrNotFound
	}
	l := out[0]
	if !l.Mine && (l.Status != "open" || !l.ExpiresAt.After(now)) {
		return Listing{}, ErrNotFound
	}
	return l, nil
}

func scanListings(rows pgx.Rows, viewerStoreID string) ([]Listing, error) {
	out := []Listing{}
	for rows.Next() {
		var l Listing
		var kind, ownerStoreID string
		if err := rows.Scan(&l.ID, &l.StoreName, &l.Name, &l.NameKey, &kind,
			&l.Qty, &l.UnitPriceKRW, &l.Note,
			&l.RegionCode, &l.RegionDetail,
			&l.Status, &l.CreatedAt, &l.ExpiresAt, &l.ReportCount,
			&ownerStoreID); err != nil {
			return nil, err
		}
		l.Kind = domain.ListingKind(kind)
		l.KindLabel = l.Kind.Label()
		if r, ok := domain.RegionByCode(l.RegionCode); ok {
			l.RegionName = r.Name
		}
		l.Mine = ownerStoreID == viewerStoreID && viewerStoreID != ""
		if !l.Mine {
			// 남의 글의 신고 수를 보여줄 이유가 없다. 보여주면 "신고가
			// 둘이나 쌓인 글"이라는 낙인이 되고, 그건 우리가 할 판단이 아니다.
			l.ReportCount = 0
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CloseListing은 내 글을 내린다. 남의 글이면 ErrNotFound 다.
func (s *Store) CloseListing(ctx context.Context, id, storeID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE listings SET status='closed', closed_at=now()
		 WHERE id=$1 AND store_id=$2 AND status='open'`, id, storeID)
	if err != nil {
		return fmt.Errorf("장터 글 내리기: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListingContact는 글 올린 사장님의 연락처다.
type ListingContact struct {
	StoreName string `json:"storeName"`
	Phone     string `json:"phone"`
	BizNo     string `json:"bizNo"`
}

// ContactFor는 연락처를 돌려주고 열람 사실을 기록한다.
//
// 번호는 이 함수로만 나간다. 목록·상세 구조체에는 칸 자체가 없어서,
// 실수로 응답에 섞여 나갈 길이 없다. 버튼을 눌러야 요청이 가고, 그때
// 누가 봤는지가 남는다 — 환불 건의 계좌 열람과 같은 원칙이다.
func (s *Store) ContactFor(ctx context.Context, listingID, viewerStoreID, viewerUserID string) (ListingContact, error) {
	var c ListingContact
	var ownerStoreID string
	err := s.pool.QueryRow(ctx, `
		SELECT st.id, st.name, COALESCE(st.phone,''), COALESCE(st.biz_no,'')
		  FROM listings l JOIN stores st ON st.id = l.store_id
		 WHERE l.id = $1`, listingID,
	).Scan(&ownerStoreID, &c.StoreName, &c.Phone, &c.BizNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListingContact{}, ErrNotFound
	}
	if err != nil {
		return ListingContact{}, fmt.Errorf("연락처 조회: %w", err)
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO listing_contacts (listing_id, viewer_store_id, viewer_user_id)
		VALUES ($1,$2,$3)`, listingID, viewerStoreID, nilUUID(viewerUserID)); err != nil {
		// 기록을 못 남겼으면 번호를 주지 않는다. 계좌 열람과 반대다 —
		// 저쪽은 사장님이 자기 손님 정보를 보는 것이고, 이쪽은 남의
		// 번호를 보는 것이다. 기록 없는 열람을 허용할 수 없다.
		return ListingContact{}, fmt.Errorf("연락처 열람 기록: %w", err)
	}
	return c, nil
}

// ContactsTodayFor는 이 매장이 오늘 열어본 번호 수다.
//
// 한 계정이 목록을 훑으며 전부 눌러 번호를 긁는 걸 막는 데 쓴다.
// 상한에 걸려도 목록은 계속 보여야 한다 — 장터를 못 쓰게 만드는 게 아니라
// 수집을 막는 게 목적이다.
func (s *Store) ContactsTodayFor(ctx context.Context, storeID string, now time.Time) (int, error) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM listing_contacts
		 WHERE viewer_store_id=$1 AND created_at >= $2`, storeID, dayStart,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("연락처 열람 집계: %w", err)
	}
	return n, nil
}

// ReportListing은 글을 신고한다.
//
// 두 번째 반환값이 true면 이 신고로 글이 내려갔다. 한 매장은 한 번만
// 신고할 수 있다 — 없으면 한 명이 세 번 눌러 남의 글을 내릴 수 있다.
func (s *Store) ReportListing(ctx context.Context, listingID, reporterStoreID, reason string) (removed bool, err error) {
	err = s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO listing_reports (listing_id, reporter_store_id, reason)
			VALUES ($1,$2,$3)
			ON CONFLICT (listing_id, reporter_store_id) DO NOTHING`,
			listingID, reporterStoreID, strings.TrimSpace(reason))
		if err != nil {
			return fmt.Errorf("신고 기록: %w", err)
		}
		if tag.RowsAffected() == 0 {
			// 이미 신고한 매장이다. 조용히 넘어간다 — "이미 신고하셨습니다"는
			// 알려줄 가치가 있지만 에러는 아니다.
			return nil
		}

		var count int
		if err := tx.QueryRow(ctx, `
			UPDATE listings SET report_count = report_count + 1
			 WHERE id=$1 RETURNING report_count`, listingID).Scan(&count); err != nil {
			return fmt.Errorf("신고 수 갱신: %w", err)
		}
		if count >= domain.ListingReportLimit {
			if _, err := tx.Exec(ctx, `
				UPDATE listings SET status='removed', closed_at=now()
				 WHERE id=$1 AND status='open'`, listingID); err != nil {
				return fmt.Errorf("신고 누적 처리: %w", err)
			}
			removed = true
		}
		return nil
	})
	return removed, err
}
