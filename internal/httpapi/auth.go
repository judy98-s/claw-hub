package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/judy98-s/claw-hub/internal/domain"
	"github.com/judy98-s/claw-hub/internal/store"
)

// authUser는 요청 컨텍스트에 담긴 로그인 사용자다.
func authUser(ctx context.Context) store.User {
	u, _ := ctx.Value(ctxUser).(store.User)
	return u
}

// actorOf는 감사 로그에 쓸 주체를 만든다.
func actorOf(ctx context.Context) domain.Actor {
	u := authUser(ctx)
	return domain.Actor{Kind: domain.ActorOwner, ID: u.ID}
}

// authed는 세션을 검증하고 사용자를 컨텍스트에 넣는다.
func (s *Server) authed(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			unauthorized(w)
			return
		}
		userID, err := s.session.decode(cookie.Value, s.now())
		if err != nil {
			s.clearSessionCookie(w)
			unauthorized(w)
			return
		}
		user, err := s.store.UserByID(r.Context(), userID)
		if err != nil {
			// 계정이 지워졌는데 쿠키가 남아 있는 경우.
			s.clearSessionCookie(w)
			unauthorized(w)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxUser, user)))
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := s.store.Authenticate(r.Context(), req.Email, req.Password)
	if errors.Is(err, store.ErrLocked) {
		writeError(w, http.StatusTooManyRequests, "locked",
			"로그인 시도가 너무 많습니다. 15분 후 다시 시도해주세요.")
		return
	}
	if err != nil {
		// 이메일이 없는지 비밀번호가 틀린지 구분해서 알려주지 않는다.
		// 구분하면 "이 이메일은 가입되어 있다"가 새어 나간다.
		writeError(w, http.StatusUnauthorized, "invalid_credentials",
			"이메일 또는 비밀번호가 올바르지 않습니다.")
		return
	}

	s.setSessionCookie(w, user.ID)
	writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Name: user.Name})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	writeJSON(w, http.StatusOK, userResponse{ID: u.ID, Email: u.Email, Name: u.Name})
}
