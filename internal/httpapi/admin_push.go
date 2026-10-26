package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/judy98-s/claw-hub/internal/store"
)

// handlePushKey는 브라우저가 구독할 때 필요한 VAPID 공개키를 준다.
//
// 공개키라서 숨길 것이 없다. 비어 있으면 서버에 키가 없는 것이고,
// 화면은 "알림 켜기"를 숨겨야 한다 — 눌러봤자 실패하는 버튼을 보여주는
// 것보다 없는 게 낫다.
func (s *Server) handlePushKey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"publicKey": s.cfg.VapidPublicKey,
		"enabled":   s.cfg.VapidPublicKey != "" && s.cfg.VapidPrivateKey != "",
	})
}

type pushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// handlePushSubscribe는 브라우저가 만든 구독을 저장한다.
//
// 본문은 PushSubscription.toJSON() 을 그대로 받는다. 모양을 바꾸면
// 프런트가 필드를 옮겨 담아야 하고, 그 과정에서 오타가 난다.
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	u := authUser(r.Context())

	var req pushSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	// endpoint 는 푸시 서비스의 주소다. 아무 문자열이나 저장하면 발송
	// 때마다 실패하고, 실패 로그만 쌓인다.
	if !strings.HasPrefix(endpoint, "https://") || len(endpoint) > 2000 {
		badRequest(w, "구독 정보가 올바르지 않습니다.")
		return
	}
	if req.Keys.P256dh == "" || req.Keys.Auth == "" {
		badRequest(w, "구독 정보가 올바르지 않습니다.")
		return
	}

	if err := s.store.SavePushSubscription(r.Context(), store.PushSubscription{
		StoreID:  u.StoreID,
		UserID:   u.ID,
		Endpoint: endpoint,
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
	}); err != nil {
		internalError(w, r, err, "푸시 구독 저장 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePushUnsubscribe는 구독을 지운다. 없는 것을 지워도 성공이다 —
// 끄기를 두 번 눌렀다고 에러를 보여줄 이유가 없다.
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "요청 형식이 올바르지 않습니다.")
		return
	}
	if err := s.store.DeletePushSubscription(r.Context(), strings.TrimSpace(req.Endpoint)); err != nil {
		internalError(w, r, err, "푸시 구독 삭제 실패")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
