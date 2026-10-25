package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

// signupsPerIPPerDay는 한 IP에서 하루에 만들 수 있는 매장 수다.
//
// 접수(ipClaimsPerHour)보다 훨씬 조인 이유: 손님은 하루에 여러 번 신고할
// 수 있지만, 사장님이 매장을 여러 개 만드는 일은 거의 없다. 가게가 둘인
// 사장님도 하루에 둘을 만들면 끝난다.
const signupsPerIPPerDay = 3

type signupRequest struct {
	StoreName string `json:"storeName"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Name      string `json:"name"`
}

// handleSignup은 매장과 사장님 계정을 함께 만들고 바로 로그인시킨다.
//
// 가입 직후 로그인 화면으로 보내지 않는다. 방금 정한 비밀번호를 한 번 더
// 치게 하는 단계는 이탈만 만든다.
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.SignupOpen {
		// 404가 아니라 403이다. 경로가 없는 것이 아니라 닫혀 있다는 뜻을
		// 화면이 구분해서 보여줘야 한다.
		writeError(w, http.StatusForbidden, "signup_closed",
			"이 서버는 회원가입을 받지 않습니다. 매장 관리자에게 계정을 요청해주세요.")
		return
	}

	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}

	req.StoreName = strings.TrimSpace(req.StoreName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "사장님"
	}

	if req.StoreName == "" {
		badRequest(w, "매장 이름을 입력해주세요.")
		return
	}
	if len([]rune(req.StoreName)) > 60 {
		badRequest(w, "매장 이름이 너무 깁니다.")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		badRequest(w, "이메일 형식을 확인해주세요.")
		return
	}
	// 저장 계층도 8자를 검사하지만, 여기서 막으면 bcrypt 를 돌리지 않는다.
	if len(req.Password) < 8 {
		badRequest(w, "비밀번호는 8자 이상이어야 합니다.")
		return
	}
	if len(req.Password) > 72 {
		// bcrypt 는 72바이트를 넘으면 잘라 버린다. 조용히 자르면 사용자가
		// 믿는 비밀번호와 실제로 저장된 것이 달라진다.
		badRequest(w, "비밀번호는 72자 이하여야 합니다.")
		return
	}

	storeID, user, err := s.store.CreateStoreWithOwner(
		r.Context(), req.StoreName, req.Phone, req.Email, req.Password, req.Name)
	if err != nil {
		// 로그인과 달리 이메일 중복을 숨기지 않는다. 숨기면 "가입했는데
		// 로그인이 안 된다"는 상태에 갇힌다. 가입 화면에서 이 정보가
		// 새는 것은 어차피 막을 수 없다 — 시도해 보면 알 수 있다.
		if strings.Contains(err.Error(), "이미 등록된 이메일") {
			writeError(w, http.StatusConflict, "email_taken",
				"이미 가입된 이메일입니다. 로그인해주세요.")
			return
		}
		slog.Error("회원가입 실패", "err", err, "request_id", requestIDOf(r.Context()))
		writeError(w, http.StatusInternalServerError, "signup_failed",
			"가입에 실패했습니다. 잠시 후 다시 시도해주세요.")
		return
	}

	slog.Info("매장 생성", "store_id", storeID, "user_id", user.ID,
		"request_id", requestIDOf(r.Context()))

	s.setSessionCookie(w, user.ID)
	writeJSON(w, http.StatusCreated, userResponse{
		ID: user.ID, Email: user.Email, Name: user.Name, Phone: user.Phone,
	})
}

// rateLimitSignup은 IP 단위로 가입을 제한한다.
//
// 접수(rateLimitPublic)와 달리 캐시 오류에서 **막는다.** 접수는 통과시킨다 —
// Redis가 죽었다고 손님이 고장난 기계 앞에서 환불을 못 받으면 안 되고,
// 중복의 실제 방어선은 DB 유니크 제약이기 때문이다. 가입은 반대다.
// 이메일만 바꾸면 매장을 무한히 만들 수 있어서 DB 제약이 받아 주지 못하고,
// 못 만든 사람은 잠시 후 다시 하면 된다.
func (s *Server) rateLimitSignup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		n, err := s.cache.Incr(r.Context(), "rl:signup:"+ip, 24*time.Hour)
		if err != nil {
			slog.Error("가입 레이트리밋 확인 실패 — 막음", "err", err,
				"request_id", requestIDOf(r.Context()))
			writeError(w, http.StatusServiceUnavailable, "unavailable",
				"일시적인 문제로 가입을 받지 못했습니다. 잠시 후 다시 시도해주세요.")
			return
		}
		if n > signupsPerIPPerDay {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"가입 시도가 너무 많습니다. 24시간 후 다시 시도해주세요.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleSignupStatus는 가입이 열려 있는지 알려준다.
//
// 화면이 닫힌 서버에서 가입 링크를 보여주지 않게 하려면 미리 알아야 한다.
// 눌러 보고 403을 받는 것보다 낫다.
func (s *Server) handleSignupStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"open": s.cfg.SignupOpen})
}
