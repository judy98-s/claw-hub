package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/judy98-s/claw-hub/internal/domain"
)

// 접수한 뒤 그 접수번호로 조회하는, 손님이 실제로 지나는 경로.
func (h *harness) getStatus(machineCode, receipt string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/api/public/claims/%s?machineCode=%s", receipt, machineCode)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

// postAndGetReceipt는 접수하고 접수번호를 돌려준다.
func postAndGetReceipt(t *testing.T, h *harness, f formOpts, key string) string {
	t.Helper()
	rec := h.postClaim(f, key)
	if rec.Code != http.StatusCreated {
		t.Fatalf("접수 실패: %d %s", rec.Code, rec.Body)
	}
	var res claimResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res.ReceiptCode
}

func TestPublicStatus_접수하고_조회한다(t *testing.T) {
	h := newHarness(t)
	receipt := postAndGetReceipt(t, h, defaultForm(), "key-1")

	rec := h.getStatus("ABCD23", receipt)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var res publicStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Stage != stageWaiting {
		t.Errorf("stage = %q, want waiting", res.Stage)
	}
	if res.ReceiptCode != receipt {
		t.Errorf("receiptCode = %q, want %q (영수증에 적힌 모양 그대로)", res.ReceiptCode, receipt)
	}
	if res.AmountKRW != 2000 {
		t.Errorf("amountKrw = %d", res.AmountKRW)
	}
	if res.IssueLabel != "돈만 빠짐" {
		t.Errorf("issueLabel = %q", res.IssueLabel)
	}
	if res.ReceivedAt == "" {
		t.Error("접수 시각이 비었다")
	}
	if res.ReviewedAt != "" || res.SettledAt != "" {
		t.Errorf("아직 처리 전인데 시각이 있다: reviewed=%q settled=%q", res.ReviewedAt, res.SettledAt)
	}
	// 중간 캐시가 남의 상태를 돌려주면 안 된다.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

// 이 테스트가 이 엔드포인트의 존재 이유를 지킨다. 응답에 개인정보가
// 한 글자라도 섞이면, 접수번호를 긁어 개인정보를 모을 수 있게 된다.
func TestPublicStatus_개인정보를_내려주지_않는다(t *testing.T) {
	h := newHarness(t)
	f := defaultForm()
	f.phone = "010-9876-5432"
	f.account = "1002-555-123456"
	f.holder = "박서진"
	f.description = "비밀스러운 설명 문구"
	receipt := postAndGetReceipt(t, h, f, "key-1")

	body := h.getStatus("ABCD23", receipt).Body.String()
	for _, leak := range []string{
		"01098765432", "010-9876-5432",
		"1002555123456", "1002-555-123456",
		"박서진", "비밀스러운 설명 문구",
		"riskScore", "risk_score", "phoneHash",
	} {
		if strings.Contains(body, leak) {
			t.Errorf("응답에 %q 가 들어 있다: %s", leak, body)
		}
	}
}

// on_hold 는 리스크 자동 보류다. 손님에게 "보류됐다"고 알려주면 번호를
// 바꿔가며 우회하는 법을 학습시킨다. pending 과 똑같이 보여야 한다.
func TestPublicStatus_보류를_손님에게_드러내지_않는다(t *testing.T) {
	hidden := []domain.Status{domain.StatusPending, domain.StatusNeedsReview, domain.StatusOnHold}
	for _, st := range hidden {
		if got := stageOf(st); got != stageWaiting {
			t.Errorf("%s -> %q, want waiting", st, got)
		}
	}

	// 응답 본문에 원본 status 문자열이 남아서도 안 된다. 프런트에서
	// 뭉치면 여기서 새어 나간다.
	h := newHarness(t)
	receipt := postAndGetReceipt(t, h, defaultForm(), "key-1")
	h.store.setStatus(receipt, domain.StatusOnHold)

	body := h.getStatus("ABCD23", receipt).Body.String()
	for _, leak := range []string{"on_hold", "needs_review", "보류", "확인 필요"} {
		if strings.Contains(body, leak) {
			t.Errorf("응답에 %q 가 들어 있다: %s", leak, body)
		}
	}
}

func TestPublicStatus_상태별_단계(t *testing.T) {
	cases := []struct {
		status domain.Status
		want   publicStage
	}{
		{domain.StatusApproved, stageApproved},
		{domain.StatusPaid, stageDone},
		{domain.StatusRejected, stageRejected},
	}
	for i, c := range cases {
		h := newHarness(t)
		receipt := postAndGetReceipt(t, h, defaultForm(), fmt.Sprintf("key-%d", i))
		h.store.setStatus(receipt, c.status)

		var res publicStatusResponse
		rec := h.getStatus("ABCD23", receipt)
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		if res.Stage != c.want {
			t.Errorf("%s -> %q, want %q", c.status, res.Stage, c.want)
		}
	}

	// 모르는 상태는 waiting 으로 떨어져야 한다. 상태를 추가했을 때
	// 실수로 "환불이 끝났어요"가 뜨는 것보다 "기다려주세요"가 낫다.
	if got := stageOf(domain.Status("some_future_status")); got != stageWaiting {
		t.Errorf("모르는 상태 -> %q, want waiting", got)
	}
}

// 승인했다가 거절로 바꾼 흐름에서 송금 시각이 남아 있으면 "환불됐다"로 읽힌다.
func TestPublicStatus_거절은_송금_시각을_지운다(t *testing.T) {
	h := newHarness(t)
	receipt := postAndGetReceipt(t, h, defaultForm(), "key-1")
	h.store.setStatus(receipt, domain.StatusPaid)
	h.store.setStatus(receipt, domain.StatusRejected)

	var res publicStatusResponse
	if err := json.Unmarshal(h.getStatus("ABCD23", receipt).Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Stage != stageRejected {
		t.Fatalf("stage = %q", res.Stage)
	}
	if res.SettledAt != "" {
		t.Errorf("거절인데 송금 시각이 남아 있다: %q", res.SettledAt)
	}
}

func TestPublicStatus_기계코드가_필요하다(t *testing.T) {
	h := newHarness(t)
	receipt := postAndGetReceipt(t, h, defaultForm(), "key-1")

	// machineCode 없이
	req := httptest.NewRequest(http.MethodGet, "/api/public/claims/"+receipt, nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	// 다른 기계 코드로는 안 보여야 한다 — 매장 간 열거를 막는 선이다.
	h.store.addMachine("ZZZZ99", "9번 기계")
	if rec := h.getStatus("ZZZZ99", receipt); rec.Code != http.StatusNotFound {
		t.Errorf("다른 기계로 조회 status = %d, want 404", rec.Code)
	}
}

func TestPublicStatus_없는_접수번호(t *testing.T) {
	h := newHarness(t)
	for _, code := range []string{
		"DEADBEEF", // 형식은 맞지만 없는 번호
		"ZZZZZZZZ", // 16진수가 아니다
		"ABC",      // 너무 짧다
		"",         // 비었다
	} {
		if rec := h.getStatus("ABCD23", code); rec.Code != http.StatusNotFound {
			t.Errorf("%q -> status = %d, want 404", code, rec.Code)
		}
	}
}

func TestPublicStatus_조회_한도(t *testing.T) {
	h := newHarness(t)
	receipt := postAndGetReceipt(t, h, defaultForm(), "key-1")

	for i := 0; i < statusChecksPerMinute; i++ {
		if rec := h.getStatus("ABCD23", receipt); rec.Code != http.StatusOK {
			t.Fatalf("%d번째 조회에서 막혔다: %d", i+1, rec.Code)
		}
	}
	if rec := h.getStatus("ABCD23", receipt); rec.Code != http.StatusTooManyRequests {
		t.Errorf("한도 초과 status = %d, want 429", rec.Code)
	}
}
