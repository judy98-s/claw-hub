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
		user, ok := s.userFromSession(r)
		if !ok {
			// 쿠키가 있는데 못 쓰면 지운다. 만료됐거나 계정이 삭제된 경우다.
			if _, err := r.Cookie(sessionCookie); err == nil {
				s.clearSessionCookie(w)
			}
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxAccess, access{
			storeID: user.StoreID,
			actor:   domain.Actor{Kind: domain.ActorOwner, ID: user.ID},
		})
		next(w, r.WithContext(ctx))
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
	Phone string `json:"phone"`
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
	writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Name: user.Name, Phone: user.Phone})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())
	writeJSON(w, http.StatusOK, userResponse{ID: u.ID, Email: u.Email, Name: u.Name, Phone: u.Phone})
}

// access는 이 요청이 어떤 자격으로 들어왔는지다.
type access struct {
	storeID string
	actor   domain.Actor
	// viaLink가 true면 Slack 링크의 토큰으로 들어온 것이다.
	// 이 건 하나에만 권한이 있다.
	viaLink bool
}

// accessOf는 컨텍스트에 담긴 자격을 꺼낸다.
func accessOf(ctx context.Context) access {
	a, _ := ctx.Value(ctxAccess).(access)
	return a
}

// claimToken은 요청에서 접근 토큰을 찾는다.
//
// 헤더를 먼저 본다. 쿼리 문자열은 프록시 접근 로그와 Referer 헤더에 남기
// 쉬워서, 프론트가 첫 로드 이후에는 헤더로 보낸다. 쿼리는 Slack 링크를
// 처음 눌렀을 때의 진입 경로다.
func claimTokenFrom(r *http.Request) string {
	if v := r.Header.Get("X-Claim-Token"); v != "" {
		return v
	}
	return r.URL.Query().Get("t")
}

// claimScoped는 로그인 세션 또는 그 건 전용 링크 토큰을 받는다.
//
// 둘 중 하나만 있으면 된다. 세션이 있으면 매장 전체 권한이고, 토큰만
// 있으면 경로의 claim 하나에만 권한이 있다.
func (s *Server) claimScoped(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := s.userFromSession(r); ok {
			ctx := context.WithValue(r.Context(), ctxUser, user)
			ctx = context.WithValue(ctx, ctxAccess, access{
				storeID: user.StoreID,
				actor:   domain.Actor{Kind: domain.ActorOwner, ID: user.ID},
			})
			next(w, r.WithContext(ctx))
			return
		}

		token := claimTokenFrom(r)
		if token == "" {
			unauthorized(w)
			return
		}
		storeID, err := s.claimToken.decode(token, r.PathValue("id"), s.now())
		if err != nil {
			// 만료와 위조를 구분해서 알려준다. 만료면 사장님이 로그인하면
			// 되고, 위조면 링크를 잘못 복사한 것이다.
			writeError(w, http.StatusUnauthorized, "invalid_link", err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), ctxAccess, access{
			storeID: storeID,
			actor:   domain.Actor{Kind: domain.ActorLink},
			viaLink: true,
		})
		next(w, r.WithContext(ctx))
	})
}

// userFromSession은 쿠키에서 로그인 사용자를 찾는다. 없으면 ok=false.
func (s *Server) userFromSession(r *http.Request) (store.User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return store.User{}, false
	}
	userID, err := s.session.decode(cookie.Value, s.now())
	if err != nil {
		return store.User{}, false
	}
	user, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		return store.User{}, false
	}
	return user, true
}
