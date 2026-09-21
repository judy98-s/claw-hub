package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/judy98-s/claw-hub/internal/crypto"
	"github.com/judy98-s/claw-hub/internal/payout"
	"github.com/judy98-s/claw-hub/internal/store"
)

// profileResponse는 설정 화면이 받는 내 계정 정보다.
type profileResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Active bool   `json:"active"`
}

func toProfile(u store.User) profileResponse {
	return profileResponse{ID: u.ID, Email: u.Email, Name: u.Name, Phone: u.Phone, Active: u.Active}
}

type profileRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// normalizeOptionalPhone은 비어 있으면 통과, 있으면 형식을 검사한다.
// 연락처는 필수가 아니다 — 혼자 쓰는 사장님은 자기 번호를 적을 이유가 없다.
func normalizeOptionalPhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	return crypto.NormalizePhone(raw)
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req profileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := len([]rune(name)); n < 1 || n > 40 {
		badRequest(w, "이름을 입력해주세요 (40자 이내).")
		return
	}
	phone, err := normalizeOptionalPhone(req.Phone)
	if err != nil {
		badRequest(w, "휴대폰 번호를 정확히 입력해주세요.")
		return
	}

	updated, err := s.store.UpdateUser(r.Context(), u.StoreID, u.ID, name, phone)
	if err != nil {
		internalError(w, r, err, "계정 수정 실패")
		return
	}
	writeJSON(w, http.StatusOK, toProfile(updated))
}

// storeResponse는 매장 정보다.
type storeResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func (s *Server) handleGetStore(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	d, err := s.store.StoreByID(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "매장 조회 실패")
		return
	}
	writeJSON(w, http.StatusOK, storeResponse{ID: d.ID, Name: d.Name, Phone: d.Phone})
}

func (s *Server) handleUpdateStore(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := len([]rune(name)); n < 1 || n > 60 {
		badRequest(w, "매장 이름을 입력해주세요 (60자 이내).")
		return
	}

	// 매장 대표번호는 유선번호일 수 있다. 숫자만 남기고 길이만 본다.
	phone := strings.TrimSpace(req.Phone)
	digits := onlyDigits(phone)
	if phone != "" && (len(digits) < 8 || len(digits) > 12) {
		badRequest(w, "매장 대표번호를 정확히 입력해주세요.")
		return
	}

	d, err := s.store.UpdateStore(r.Context(), u.StoreID, name, digits)
	if err != nil {
		internalError(w, r, err, "매장 수정 실패")
		return
	}
	writeJSON(w, http.StatusOK, storeResponse{ID: d.ID, Name: d.Name, Phone: d.Phone})
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	users, err := s.store.ListUsers(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "계정 목록 조회 실패")
		return
	}
	out := make([]profileResponse, 0, len(users))
	for _, x := range users {
		out = append(out, toProfile(x))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "meId": u.ID})
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
}

// handleCreateUser는 같은 매장에 직원 계정을 추가한다.
//
// 권한 구분은 두지 않았다. 추가된 계정은 사장님과 같은 것을 할 수 있다.
// 한 매장에 두세 명이 쓰는 시스템에서 역할 체계는 관리 비용만 늘린다.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if !strings.Contains(email, "@") || len(email) > 200 {
		badRequest(w, "이메일을 정확히 입력해주세요.")
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := len([]rune(name)); n < 1 || n > 40 {
		badRequest(w, "이름을 입력해주세요 (40자 이내).")
		return
	}
	if len(req.Password) < 8 {
		badRequest(w, "비밀번호는 8자 이상이어야 합니다.")
		return
	}
	phone, err := normalizeOptionalPhone(req.Phone)
	if err != nil {
		badRequest(w, "휴대폰 번호를 정확히 입력해주세요.")
		return
	}

	created, err := s.store.CreateUser(r.Context(), u.StoreID, email, req.Password, name, phone)
	if err != nil {
		// 중복 이메일처럼 사용자가 고칠 수 있는 오류다.
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toProfile(created))
}

type userActiveRequest struct {
	Active bool `json:"active"`
}

func (s *Server) handleSetUserActive(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	target := r.PathValue("id")

	var req userActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	// 자기 자신을 끄면 그 즉시 로그아웃되고 되돌릴 사람이 없을 수 있다.
	if target == u.ID && !req.Active {
		badRequest(w, "자기 계정은 비활성화할 수 없습니다.")
		return
	}

	if err := s.store.SetUserActive(r.Context(), u.StoreID, target, req.Active); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFound(w, "계정을 찾을 수 없습니다.")
			return
		}
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": req.Active})
}

// payoutFor는 매장 설정에 맞는 송금 링크 제공자를 만든다.
//
// 설정이 없으면 환경변수(PAYOUT_DEEPLINK_TEMPLATES)로 떨어진다. 기존
// 배포를 깨지 않으면서, 새로 쓰는 사람은 화면에서만 정하면 되게 한다.
func (s *Server) payoutFor(ctx context.Context, storeID string) payout.Payout {
	st, err := s.store.StoreByID(ctx, storeID)
	if err != nil || st.PayoutProvider == "" {
		return s.payout
	}
	tpl := payout.TemplateFor(st.PayoutProvider, st.PayoutTemplate)
	if tpl == "" {
		// '사용 안 함'을 고른 것이다. 빈 제공자를 주면 UI 가 계좌 복사
		// 경로만 보여준다.
		return payout.NewDeeplink(nil)
	}
	return payout.NewDeeplink(map[string]string{st.PayoutProvider: tpl})
}

// payoutSettingsResponse는 설정 화면이 받는 송금 설정이다.
type payoutSettingsResponse struct {
	Providers []payout.Provider `json:"providers"`
	Banks     []payout.Bank     `json:"banks"`
	Provider  string            `json:"provider"`
	Template  string            `json:"template"`
	BankCode  string            `json:"bankCode"`
	Account   string            `json:"account"`
}

func (s *Server) handleGetPayoutSettings(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	st, err := s.store.StoreByID(r.Context(), u.StoreID)
	if err != nil {
		internalError(w, r, err, "송금 설정 조회 실패")
		return
	}
	provider := st.PayoutProvider
	if provider == "" {
		provider = payout.ProviderNone
	}
	writeJSON(w, http.StatusOK, payoutSettingsResponse{
		Providers: payout.Providers(), Banks: payout.Banks(),
		Provider: provider, Template: st.PayoutTemplate,
		BankCode: st.PayoutBankCode, Account: st.PayoutAccount,
	})
}

type payoutSettingsRequest struct {
	Provider string `json:"provider"`
	Template string `json:"template"`
	BankCode string `json:"bankCode"`
	Account  string `json:"account"`
}

func (s *Server) handleUpdatePayoutSettings(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req payoutSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	if _, ok := payout.ProviderByID(req.Provider); !ok {
		badRequest(w, "송금 앱을 선택해주세요.")
		return
	}
	tpl := strings.TrimSpace(req.Template)
	if req.Provider == payout.ProviderCustom {
		if tpl == "" {
			badRequest(w, "딥링크 주소를 입력해주세요.")
			return
		}
		if !strings.Contains(tpl, "://") {
			badRequest(w, "딥링크 주소 형식이 올바르지 않습니다 (예: myapp://send?...).")
			return
		}
	} else {
		// 내장 앱을 고르면 직접 입력값은 버린다. 남겨두면 나중에 custom 으로
		// 바꿨을 때 예전 값이 되살아나 예상 밖으로 동작한다.
		tpl = ""
	}

	account := onlyDigits(req.Account)
	bankCode := strings.TrimSpace(req.BankCode)
	if account != "" {
		if len(account) < 6 || len(account) > 20 {
			badRequest(w, "출금 계좌번호를 정확히 입력해주세요.")
			return
		}
		if _, ok := payout.BankByCode(bankCode); !ok {
			badRequest(w, "출금 계좌의 은행을 선택해주세요.")
			return
		}
	} else {
		bankCode = ""
	}

	st, err := s.store.UpdatePayoutSettings(r.Context(), u.StoreID, store.PayoutSettings{
		Provider: req.Provider, Template: tpl, BankCode: bankCode, Account: account,
	})
	if err != nil {
		internalError(w, r, err, "송금 설정 저장 실패")
		return
	}
	writeJSON(w, http.StatusOK, payoutSettingsResponse{
		Providers: payout.Providers(), Banks: payout.Banks(),
		Provider: st.PayoutProvider, Template: st.PayoutTemplate,
		BankCode: st.PayoutBankCode, Account: st.PayoutAccount,
	})
}
