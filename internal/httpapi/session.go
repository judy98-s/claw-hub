package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie   = "clawhub_session"
	sessionLifetime = 14 * 24 * time.Hour
)

var errBadSession = errors.New("세션이 올바르지 않습니다")

// sessionCodec은 세션 쿠키를 서명하고 검증한다.
//
// 서버에 세션 테이블을 두지 않는다. 사장님 한두 명이 쓰는 시스템에서
// 세션 저장소를 운영할 이유가 없고, 서명된 쿠키면 충분하다.
// 대신 즉시 무효화가 안 되므로 수명을 2주로 제한했다.
type sessionCodec struct {
	secret []byte
}

func newSessionCodec(secret []byte) *sessionCodec {
	return &sessionCodec{secret: secret}
}

// encode는 "userID.만료시각.서명" 형태의 토큰을 만든다.
func (c *sessionCodec) encode(userID string, expiresAt time.Time) string {
	payload := userID + "." + strconv.FormatInt(expiresAt.Unix(), 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(c.sign(payload))
}

func (c *sessionCodec) sign(payload string) []byte {
	m := hmac.New(sha256.New, c.secret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// decode는 토큰을 검증하고 userID를 돌려준다.
func (c *sessionCodec) decode(token string, now time.Time) (string, error) {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return "", errBadSession
	}
	payload, sigStr := token[:i], token[i+1:]

	sig, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil {
		return "", errBadSession
	}
	// 서명 비교는 상수 시간으로. 바이트별 조기 종료는 서명을 한 바이트씩
	// 맞춰 나가는 공격을 가능하게 한다.
	if subtle.ConstantTimeCompare(sig, c.sign(payload)) != 1 {
		return "", errBadSession
	}

	j := strings.LastIndex(payload, ".")
	if j < 0 {
		return "", errBadSession
	}
	userID, expStr := payload[:j], payload[j+1:]

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", errBadSession
	}
	if now.After(time.Unix(exp, 0)) {
		return "", fmt.Errorf("세션이 만료되었습니다")
	}
	if userID == "" {
		return "", errBadSession
	}
	return userID, nil
}

// setSessionCookie는 로그인 쿠키를 심는다.
func (s *Server) setSessionCookie(w http.ResponseWriter, userID string) {
	exp := s.now().Add(sessionLifetime)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.session.encode(userID, exp),
		Path:     "/",
		Expires:  exp,
		HttpOnly: true, // JS가 읽을 수 없다 — XSS로 세션을 훔칠 수 없다
		Secure:   s.secureCookies(),
		SameSite: http.SameSiteLaxMode, // CSRF 기본 방어
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secureCookies(), SameSite: http.SameSiteLaxMode,
	})
}

// secureCookies는 배포 주소가 https일 때만 Secure를 켠다.
// 로컬 개발(http://localhost)에서 Secure를 켜면 쿠키가 아예 저장되지 않는다.
func (s *Server) secureCookies() bool {
	return strings.HasPrefix(s.cfg.PublicBaseURL, "https://")
}
