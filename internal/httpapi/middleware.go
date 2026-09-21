package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxUser
)

func requestIDOf(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestID).(string)
	return id
}

// withRequestID는 요청마다 추적 ID를 붙인다.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			// 추적 ID를 못 만든다고 요청을 거절할 이유는 없다.
			b = []byte(time.Now().Format("150405.000"))
		}
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
	})
}

// withRecover는 핸들러 패닉이 프로세스를 죽이지 않게 한다.
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("핸들러 패닉", "panic", v, "path", r.URL.Path,
					"request_id", requestIDOf(r.Context()))
				writeError(w, http.StatusInternalServerError, "internal_error",
					"처리 중 문제가 생겼습니다. 잠시 후 다시 시도해주세요.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withMaxBytes는 요청 본문 크기를 제한한다.
// 이게 없으면 누구나 디스크를 채울 수 있다.
func withMaxBytes(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// clientIP는 요청자의 IP를 최선으로 추정한다.
//
// 리버스 프록시 뒤에서 동작하므로 X-Forwarded-For를 본다. 이 헤더는
// 위조 가능하지만, 레이트리밋은 정직한 사용자의 실수와 가벼운 남용을 막는
// 장치이지 결연한 공격자를 막는 장치가 아니다. 진짜 방어는 전화번호 단위
// 카운트와 DB 유니크 제약이다.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// randomToken은 저장 키에 쓰는 무작위 토큰을 만든다.
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 엔트로피를 못 얻으면 키가 충돌할 수 있다. 시각을 섞어 최소한
		// 같은 요청 안에서는 겹치지 않게 한다.
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
