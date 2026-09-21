package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// claimTokenTTL은 Slack 알림 링크의 유효기간이다.
//
// 7일: 주말에 들어온 신고를 월요일에 처리해도 열린다. 그 뒤에는 만료되어
// 로그인이 필요하므로, 오래된 Slack 로그를 뒤지는 사람이 계좌를 볼 수 없다.
const claimTokenTTL = 7 * 24 * time.Hour

var errBadClaimToken = errors.New("링크가 올바르지 않습니다")

// claimTokenPurpose는 HMAC 도메인 분리용 접두사다.
//
// 세션 쿠키와 같은 비밀키를 쓰므로, 접두사가 없으면 한쪽 토큰을 다른 쪽에
// 밀어넣는 혼동 공격이 가능해진다.
const claimTokenPurpose = "clawhub:claim-link:v1:"

// claimTokenCodec은 신고 건 하나에만 통하는 접근 토큰을 만든다.
//
// Slack 알림 링크에 붙는다. 로그인 없이 그 건의 상세를 열고 승인·거절·송금
// 기록까지 할 수 있지만, 다른 건이나 연락처 목록·기계 관리에는 닿지 않는다.
// 링크가 새어도 피해가 한 건으로 묶인다.
type claimTokenCodec struct {
	secret []byte
}

func newClaimTokenCodec(secret []byte) *claimTokenCodec {
	return &claimTokenCodec{secret: secret}
}

// encode는 storeID와 만료시각을 담은 토큰을 만든다.
//
// claimID는 토큰 본문에 넣지 않고 서명에만 섞는다. 덕분에 토큰이 짧아지고,
// 다른 건의 URL에 같은 토큰을 붙이면 서명이 맞지 않아 거부된다.
func (c *claimTokenCodec) encode(storeID, claimID string, expiresAt time.Time) string {
	payload := storeID + ":" + strconv.FormatInt(expiresAt.Unix(), 10)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return body + "." + base64.RawURLEncoding.EncodeToString(c.sign(claimID, payload))
}

func (c *claimTokenCodec) sign(claimID, payload string) []byte {
	m := hmac.New(sha256.New, c.secret)
	m.Write([]byte(claimTokenPurpose))
	m.Write([]byte(claimID))
	m.Write([]byte{0}) // claimID와 payload 경계를 못 박는다
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// decode는 토큰이 이 claimID에 유효한지 확인하고 storeID를 돌려준다.
func (c *claimTokenCodec) decode(token, claimID string, now time.Time) (string, error) {
	bodyStr, sigStr, ok := strings.Cut(token, ".")
	if !ok {
		return "", errBadClaimToken
	}

	body, err := base64.RawURLEncoding.DecodeString(bodyStr)
	if err != nil {
		return "", errBadClaimToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil {
		return "", errBadClaimToken
	}

	// 서명 비교는 상수 시간으로. 바이트별 조기 종료는 서명을 한 바이트씩
	// 맞춰 나가는 공격을 가능하게 한다.
	if subtle.ConstantTimeCompare(sig, c.sign(claimID, string(body))) != 1 {
		return "", errBadClaimToken
	}

	storeID, expStr, ok := strings.Cut(string(body), ":")
	if !ok || storeID == "" {
		return "", errBadClaimToken
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", errBadClaimToken
	}
	if now.After(time.Unix(exp, 0)) {
		return "", fmt.Errorf("링크가 만료되었습니다. 대시보드에 로그인해서 확인해주세요")
	}
	return storeID, nil
}

// claimLinkURL은 Slack 알림에 넣을 접근 링크를 만든다.
func (s *Server) claimLinkURL(storeID, claimID string) string {
	token := s.claimToken.encode(storeID, claimID, s.now().Add(claimTokenTTL))
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/admin/claims/" + claimID + "?t=" + token
}
