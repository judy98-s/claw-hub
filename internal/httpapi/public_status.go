package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

// 접수 상태 조회 한도. 접수(3/시간)보다 느슨하다 — 손님은 진행 상황을
// 여러 번 들여다본다. 긁기를 막는 수준만 유지한다.
const statusChecksPerMinute = 20

// publicStage는 손님에게 보여줄 진행 단계다.
//
// 서버 status 6개를 4개로 줄여 내려준다. 특히 on_hold(리스크 자동 보류)를
// waiting 에 섞는다. 보류됐다는 사실을 알려주면 번호를 바꿔가며 우회하는
// 법을 학습시킨다 — done 화면에 이미 적어둔 원칙이다.
//
// **서버가 뭉쳐서 내려야 한다.** 프런트에서 뭉치면 원본 status 가 네트워크
// 응답에 남아 개발자 도구로 그대로 보인다.
type publicStage string

const (
	stageWaiting  publicStage = "waiting"  // pending · needs_review · on_hold
	stageApproved publicStage = "approved" // 승인, 송금 대기
	stageDone     publicStage = "done"     // 송금 완료 · 카드 취소 완료
	stageRejected publicStage = "rejected" // 거절
)

func stageOf(s domain.Status) publicStage {
	switch s {
	case domain.StatusApproved:
		return stageApproved
	case domain.StatusPaid:
		return stageDone
	case domain.StatusRejected:
		return stageRejected
	default:
		// pending · needs_review · on_hold, 그리고 모르는 값.
		// 기본값이 waiting 이어야 안전하다 — 상태를 추가했을 때
		// 실수로 "환불 끝났어요"가 뜨는 것보다 "기다려주세요"가 낫다.
		return stageWaiting
	}
}

type publicStatusResponse struct {
	ReceiptCode string      `json:"receiptCode"`
	Stage       publicStage `json:"stage"`
	// IssueLabel 은 라벨만 준다. 코드를 주면 프런트가 또 표를 들고 있어야 한다.
	IssueLabel    string `json:"issueLabel"`
	AmountKRW     int    `json:"amountKrw"`
	PaymentMethod string `json:"paymentMethod"`
	StoreName     string `json:"storeName"`
	StorePhone    string `json:"storePhone"`
	StoreType     string `json:"storeType"`
	// 타임라인 시각. 비어 있으면 아직 그 단계가 아니다.
	// 분 단위로 자른다 — 초까지 주면 사장님이 몇 초 만에 처리했는지가
	// 손님에게 보이고, 그걸로 "제대로 안 봤다"는 항의가 생긴다.
	ReceivedAt string `json:"receivedAt"`
	ReviewedAt string `json:"reviewedAt,omitempty"`
	SettledAt  string `json:"settledAt,omitempty"`
}

// handlePublicClaimStatus는 접수번호로 처리 상태를 돌려준다.
//
//	GET /api/public/claims/{receiptCode}?machineCode=ABCD23
//
// machineCode 는 필수다. 이유는 store.PublicStatusByReceipt 에 적었다.
func (s *Server) handlePublicClaimStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	machineCode := domain.NormalizeMachineCode(r.URL.Query().Get("machineCode"))
	if err := domain.ValidateMachineCode(machineCode); err != nil {
		// 접수번호가 맞는지 틀렸는지 구분해서 알려주지 않는다.
		notFound(w, "접수 내역을 찾을 수 없습니다. 접수번호를 다시 확인해주세요.")
		return
	}

	// 기계 단위로 센다. IP 단위로 세면 같은 가게에서 여러 손님이 쓸 때
	// 서로의 한도를 깎는다 — 지하 상가는 공용 와이파이를 쓴다.
	key := "rl:status:" + machineCode + ":" + clientIP(r)
	if n, err := s.cache.Incr(ctx, key, time.Minute); err != nil {
		slog.Warn("상태 조회 레이트리밋 확인 실패 — 통과시킴",
			"err", err, "request_id", requestIDOf(ctx))
	} else if n > statusChecksPerMinute {
		writeError(w, http.StatusTooManyRequests, "rate_limited",
			"조회가 너무 많습니다. 잠시 후 다시 시도해주세요.")
		return
	}

	code := r.PathValue("receiptCode")
	p, err := s.store.PublicStatusByReceipt(ctx, machineCode, code)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w, "접수 내역을 찾을 수 없습니다. 접수번호를 다시 확인해주세요.")
		return
	}
	if err != nil {
		internalError(w, r, err, "접수 상태 조회 실패")
		return
	}

	res := publicStatusResponse{
		ReceiptCode:   normalizeReceiptCode(code),
		Stage:         stageOf(p.Status),
		IssueLabel:    p.IssueType.Label(),
		AmountKRW:     p.AmountKRW,
		PaymentMethod: string(p.PaymentMethod),
		StoreName:     p.StoreName,
		StorePhone:    p.StorePhone,
		StoreType:     string(p.StoreType),
		ReceivedAt:    minuteStamp(p.CreatedAt),
		ReviewedAt:    minuteStamp(p.ResolvedAt),
		SettledAt:     minuteStamp(p.PaidAt),
	}

	// 거절은 종료다. 사장님이 승인했다가 거절로 바꾼 흐름에서 송금 시각이
	// 남아 있으면 "환불됐다"로 읽힌다.
	if res.Stage == stageRejected {
		res.SettledAt = ""
	}

	// 캐시 금지. 공용 와이파이의 중간 캐시가 남의 상태를 돌려줄 수 있다.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, res)
}

// minuteStamp는 분 단위 RFC3339 문자열이다. zero time 은 빈 문자열이다.
func minuteStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Truncate(time.Minute).Format(time.RFC3339)
}
