package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorBody는 모든 에러 응답의 형태다.
//
// Message는 손님과 사장님이 화면에서 그대로 읽는 한국어다. 영어 에러나
// 스택트레이스를 노출하면 길에서 화난 손님이 더 화가 난다.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// 헤더는 이미 나갔다. 할 수 있는 건 기록뿐이다.
		slog.Error("응답 직렬화 실패", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: code, Message: message})
}

// 자주 쓰는 에러 응답들.
func badRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "invalid_request", message)
}

func notFound(w http.ResponseWriter, message string) {
	writeError(w, http.StatusNotFound, "not_found", message)
}

func unauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "로그인이 필요합니다")
}

// internalError는 원인을 로그에만 남기고 밖으로는 일반 문구만 낸다.
// DB 에러 메시지에는 테이블명과 쿼리 구조가 들어 있다.
func internalError(w http.ResponseWriter, r *http.Request, err error, what string) {
	slog.Error(what, "err", err, "path", r.URL.Path, "request_id", requestIDOf(r.Context()))
	writeError(w, http.StatusInternalServerError, "internal_error",
		"처리 중 문제가 생겼습니다. 잠시 후 다시 시도해주세요.")
}
