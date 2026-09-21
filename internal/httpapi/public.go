package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/media"
	"github.com/judy98-s/claw-hub/internal/notify"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

const (
	maxPhotos     = 3
	maxPhotoBytes = 5 << 20

	// 레이트리밋. 정직한 사용자의 실수와 가벼운 남용을 막는 선이다.
	// 결연한 공격자는 전화번호 카운트와 DB 제약으로 막는다.
	ipClaimsPerHour      = 10
	machineClaimsPerHour = 20
)

// machineResponse는 QR을 찍었을 때 손님 화면이 받는 정보다.
type machineResponse struct {
	Code               string `json:"code"`
	Label              string `json:"label"`
	StoreName          string `json:"storeName"`
	StorePhone         string `json:"storePhone"`
	ReviewThresholdKRW int    `json:"reviewThresholdKrw"`
	MaxAmountKRW       int    `json:"maxAmountKrw"`
	MaxPhotos          int    `json:"maxPhotos"`
}

// handleMachineByCode는 QR 코드로 기계를 조회한다.
func (s *Server) handleMachineByCode(w http.ResponseWriter, r *http.Request) {
	code := domain.NormalizeMachineCode(r.PathValue("code"))
	if err := domain.ValidateMachineCode(code); err != nil {
		notFound(w, "이 스티커의 기계를 찾을 수 없습니다.")
		return
	}

	m, info, err := s.store.MachineByCode(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		// 폐기됐거나 비활성인 기계. 손님에게는 같은 메시지다.
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":      "not_found",
			"message":    "이 스티커의 기계를 찾을 수 없습니다. 매장에 직접 문의해주세요.",
			"storePhone": info.Phone,
		})
		return
	}
	if err != nil {
		internalError(w, r, err, "기계 조회 실패")
		return
	}

	writeJSON(w, http.StatusOK, machineResponse{
		Code: m.Code, Label: m.Label,
		StoreName: info.Name, StorePhone: info.Phone,
		ReviewThresholdKRW: s.policy.ReviewThresholdKRW,
		MaxAmountKRW:       s.policy.MaxAmountKRW,
		MaxPhotos:          maxPhotos,
	})
}

// handleBanks는 은행 목록을 반환한다. 손님 폼의 은행 선택에 쓴다.
func (s *Server) handleBanks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"banks": payout.Banks()})
}

// rateLimitPublic은 IP 단위로 접수를 제한한다.
//
// 캐시 오류는 통과시킨다. Redis가 죽었다고 손님이 신고를 못 하면, 고장난
// 기계 앞에서 환불도 못 받고 돌아가게 된다. 중복·남용의 실제 방어는
// DB 유니크 제약과 전화번호 카운트다.
func (s *Server) rateLimitPublic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		n, err := s.cache.Incr(r.Context(), "rl:ip:"+ip, time.Hour)
		if err != nil {
			slog.Warn("레이트리밋 확인 실패 — 통과시킴", "err", err, "request_id", requestIDOf(r.Context()))
			next.ServeHTTP(w, r)
			return
		}
		if n > ipClaimsPerHour {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"요청이 너무 많습니다. 잠시 후 다시 시도하거나 매장에 직접 문의해주세요.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// claimResponse는 접수 완료 화면이 받는 정보다.
// 손님이 입력한 계좌와 전화번호를 되돌려주지 않는다 — 돌려줄 이유가 없고,
// 응답이 캐시되거나 로그에 남으면 그게 유출 경로가 된다.
type claimResponse struct {
	ID          string `json:"id"`
	ReceiptCode string `json:"receiptCode"`
	Message     string `json:"message"`
}

// pendingPhoto는 검증은 끝났지만 아직 저장 전인 사진이다.
type pendingPhoto struct {
	key         string
	contentType string
	size        int64
	data        multipart.File
}

func (s *Server) handleCreateClaim(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" || len(idemKey) > 200 {
		badRequest(w, "요청 식별자가 없습니다. 페이지를 새로고침한 뒤 다시 시도해주세요.")
		return
	}

	if err := r.ParseMultipartForm(4 << 20); err != nil {
		badRequest(w, "첨부한 사진이 너무 크거나 형식이 올바르지 않습니다.")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck

	in, ok := s.parseClaimForm(w, r)
	if !ok {
		return
	}

	// 기계 확인이 먼저다. 없는 기계면 사진을 한 장도 쓰지 않고 끝낸다.
	m, _, err := s.store.MachineByCode(ctx, in.machineCode)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "이 스티커의 기계를 찾을 수 없습니다. 매장에 직접 문의해주세요.")
		return
	}
	if err != nil {
		internalError(w, r, err, "기계 조회 실패")
		return
	}

	photos, msg := collectPhotos(r)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	// 고액 건은 사진이 필수다. 접수와 사진이 한 요청이므로 여기서 원자적으로
	// 걸린다 — 사진 없는 고액 건이 DB에 남을 틈이 없다.
	if domain.RequiresPhoto(in.amountKRW, s.policy) && len(photos) == 0 {
		badRequest(w, fmt.Sprintf("%s원 이상은 증상이 보이는 사진이 필요합니다.",
			comma(s.policy.ReviewThresholdKRW)))
		return
	}

	// 기계 단위 레이트리밋. 한 기계에 폭주가 오면 그 기계만 막는다.
	if n, err := s.cache.Incr(ctx, "rl:machine:"+m.Code, time.Hour); err == nil && n > machineClaimsPerHour {
		writeError(w, http.StatusTooManyRequests, "rate_limited",
			"이 기계에 접수가 몰려 있습니다. 매장에 직접 문의해주세요.")
		return
	}

	// 멱등키 확인이 리스크 평가보다 먼저다. 재시도를 "빠른 중복 제출"
	// 규칙이 먼저 잡으면, 네트워크가 끊겨 다시 보낸 손님이 접수번호 대신
	// 에러를 받는다. 그러면 다른 키로 또 내거나 그냥 포기한다.
	if existing, found, err := s.store.ClaimByIdempotencyKey(ctx, m.StoreID, idemKey); err != nil {
		internalError(w, r, err, "기존 접수 조회 실패")
		return
	} else if found {
		writeJSON(w, http.StatusOK, claimResponse{
			ID: existing.ID, ReceiptCode: receiptCode(existing.ID),
			Message: "이미 접수된 요청입니다.",
		})
		return
	}

	facts, err := s.store.RiskFactsFor(ctx, store.RiskQuery{
		StoreID: m.StoreID, MachineID: m.ID,
		Phone: in.phone, Account: in.account, Now: s.now(),
	})
	if err != nil {
		internalError(w, r, err, "리스크 평가 자료 조회 실패")
		return
	}
	facts.AmountKRW = in.amountKRW

	risk := domain.Evaluate(facts, s.policy)
	if risk.RejectDuplicate {
		writeError(w, http.StatusConflict, "duplicate",
			"방금 같은 기계로 접수하셨습니다. 처리 중이니 잠시만 기다려주세요.")
		return
	}

	// 여기서부터 디스크에 쓴다. 실패하면 전부 되돌린다.
	written, err := s.writePhotos(ctx, photos)
	if err != nil {
		s.cleanupPhotos(ctx, written)
		internalError(w, r, err, "사진 저장 실패")
		return
	}

	res, err := s.store.CreateClaim(ctx, store.CreateClaimInput{
		StoreID: m.StoreID, MachineID: m.ID,
		IssueType: in.issueType, AmountKRW: in.amountKRW, Description: in.description,
		Phone: in.phone, BankCode: in.bankCode, Account: in.account, Holder: in.holder,
		Status: risk.Status, RiskScore: risk.Score, RiskReasons: risk.Reasons,
		IdempotencyKey: idemKey, IP: clientIP(r), UserAgent: r.UserAgent(),
		Photos: photoInputs(written, s.now().AddDate(0, 0, s.cfg.Policy.PhotoRetentionDays)),
	})
	if err != nil {
		s.cleanupPhotos(ctx, written)
		internalError(w, r, err, "신고 저장 실패")
		return
	}

	if res.Existing {
		// 같은 멱등키의 재시도다. 방금 쓴 사진은 고아이므로 지운다.
		s.cleanupPhotos(ctx, written)
		writeJSON(w, http.StatusOK, claimResponse{
			ID: res.ID, ReceiptCode: receiptCode(res.ID),
			Message: "이미 접수된 요청입니다.",
		})
		return
	}

	s.notifyClaimCreated(ctx, res, m.Label, in, risk, len(written))

	writeJSON(w, http.StatusCreated, claimResponse{
		ID: res.ID, ReceiptCode: receiptCode(res.ID),
		Message: "접수되었습니다. 확인 후 입력하신 계좌로 환불해드립니다.",
	})
}

// notifyClaimCreated는 사장님에게 알린다.
// 실패해도 접수는 이미 성공이다. 알림은 부가 경로이고 대시보드가 진실이다.
func (s *Server) notifyClaimCreated(ctx context.Context, res store.CreateClaimResult,
	machineLabel string, in claimForm, risk domain.RiskResult, photoCount int,
) {
	err := s.notify.ClaimCreated(ctx, notify.ClaimNotice{
		ClaimID: res.ID, MachineLabel: machineLabel,
		IssueLabel: in.issueType.Label(), AmountKRW: in.amountKRW,
		Status: risk.Status, RiskReasons: risk.Reasons,
		PhotoCount: photoCount, CreatedAt: res.CreatedAt,
	})
	if err != nil {
		slog.Warn("접수 알림 실패", "err", err, "claim_id", res.ID)
	}
}

// claimForm은 검증을 통과한 폼 입력이다.
type claimForm struct {
	machineCode string
	issueType   domain.IssueType
	amountKRW   int
	description string
	phone       string
	bankCode    string
	account     string
	holder      string
}

// parseClaimForm은 폼을 읽고 검증한다. 실패 시 응답을 직접 쓰고 false를 반환한다.
func (s *Server) parseClaimForm(w http.ResponseWriter, r *http.Request) (claimForm, bool) {
	var f claimForm

	f.machineCode = domain.NormalizeMachineCode(r.FormValue("machineCode"))
	if err := domain.ValidateMachineCode(f.machineCode); err != nil {
		badRequest(w, "기계 코드가 올바르지 않습니다.")
		return f, false
	}

	issueType, err := domain.ParseIssueType(r.FormValue("issueType"))
	if err != nil {
		badRequest(w, "증상을 선택해주세요.")
		return f, false
	}
	f.issueType = issueType

	amount, err := strconv.Atoi(strings.TrimSpace(r.FormValue("amountKrw")))
	if err != nil {
		badRequest(w, "금액을 숫자로 입력해주세요.")
		return f, false
	}
	if err := domain.ValidateAmount(amount, s.policy); err != nil {
		badRequest(w, err.Error())
		return f, false
	}
	f.amountKRW = amount

	f.description = strings.TrimSpace(r.FormValue("description"))
	if len([]rune(f.description)) > 1000 {
		badRequest(w, "설명이 너무 깁니다. 1000자 이내로 적어주세요.")
		return f, false
	}

	phone, err := crypto.NormalizePhone(r.FormValue("phone"))
	if err != nil {
		badRequest(w, "연락받으실 휴대폰 번호를 정확히 입력해주세요.")
		return f, false
	}
	f.phone = phone

	f.bankCode = strings.TrimSpace(r.FormValue("bankCode"))
	if _, ok := payout.BankByCode(f.bankCode); !ok {
		badRequest(w, "은행을 선택해주세요.")
		return f, false
	}

	f.account = crypto.NormalizeAccount(r.FormValue("accountNo"))
	if len(f.account) < 6 || len(f.account) > 20 {
		badRequest(w, "계좌번호를 정확히 입력해주세요.")
		return f, false
	}

	f.holder = strings.TrimSpace(r.FormValue("holder"))
	if n := len([]rune(f.holder)); n < 1 || n > 40 {
		badRequest(w, "예금주 이름을 입력해주세요.")
		return f, false
	}

	return f, true
}

// collectPhotos는 첨부 사진을 검증한다. 문제가 있으면 손님용 메시지를 반환한다.
//
// 확장자와 클라이언트가 보낸 Content-Type은 보지 않는다. 파일 내용의
// 매직바이트로만 판정한다 — 둘 다 손님이 마음대로 정할 수 있는 값이다.
func collectPhotos(r *http.Request) ([]pendingPhoto, string) {
	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		return nil, ""
	}
	if len(files) > maxPhotos {
		return nil, fmt.Sprintf("사진은 최대 %d장까지 첨부할 수 있습니다.", maxPhotos)
	}

	out := make([]pendingPhoto, 0, len(files))
	for _, fh := range files {
		if fh.Size > maxPhotoBytes {
			return nil, fmt.Sprintf("사진 한 장은 %dMB 이하여야 합니다.", maxPhotoBytes>>20)
		}
		if fh.Size == 0 {
			return nil, "빈 파일이 첨부되었습니다."
		}

		f, err := fh.Open()
		if err != nil {
			return nil, "첨부한 사진을 읽을 수 없습니다."
		}

		head := make([]byte, media.SniffLen)
		n, _ := io.ReadFull(f, head)
		ct, err := media.DetectImageType(head[:n])
		if err != nil {
			f.Close() //nolint:errcheck
			return nil, "사진 파일만 첨부할 수 있습니다 (jpg, png, webp, heic)."
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close() //nolint:errcheck
			return nil, "첨부한 사진을 읽을 수 없습니다."
		}

		out = append(out, pendingPhoto{contentType: ct, size: fh.Size, data: f})
	}
	return out, ""
}

// writePhotos는 검증된 사진을 저장소에 쓴다.
func (s *Server) writePhotos(ctx context.Context, photos []pendingPhoto) ([]pendingPhoto, error) {
	written := make([]pendingPhoto, 0, len(photos))
	for _, p := range photos {
		defer p.data.Close() //nolint:errcheck

		// 키는 서버가 만든다. 손님이 보낸 파일명은 경로에 닿지 않는다.
		p.key = fmt.Sprintf("claims/%s/%s%s",
			s.now().Format("2006/01"), randomToken(16), media.ExtFor(p.contentType))

		if err := s.media.Put(ctx, p.key, p.data, p.contentType, p.size); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}

// cleanupPhotos는 접수가 실패했을 때 방금 쓴 사진을 지운다.
// 이게 없으면 거부된 요청이 디스크에 쓰레기를 남긴다.
func (s *Server) cleanupPhotos(ctx context.Context, photos []pendingPhoto) {
	for _, p := range photos {
		if p.key == "" {
			continue
		}
		if err := s.media.Delete(ctx, p.key); err != nil {
			slog.Warn("사진 정리 실패", "err", err, "key", p.key)
		}
	}
}

func photoInputs(photos []pendingPhoto, expiresAt time.Time) []store.PhotoInput {
	out := make([]store.PhotoInput, 0, len(photos))
	for _, p := range photos {
		out = append(out, store.PhotoInput{
			ObjectKey: p.key, ContentType: p.contentType,
			SizeBytes: p.size, ExpiresAt: expiresAt,
		})
	}
	return out
}

// receiptCode는 손님이 매장에 말할 수 있는 짧은 접수번호다.
// UUID 전체를 읽어주는 건 불가능하다.
func receiptCode(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) < 8 {
		return strings.ToUpper(clean)
	}
	return strings.ToUpper(clean[:8])
}

func comma(n int) string {
	s := strconv.Itoa(n)
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
