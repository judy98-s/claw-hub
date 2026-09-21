#!/usr/bin/env bash
#
# Slack 웹훅이 살아 있는지 확인한다.
#
# 사용법:  ./scripts/check-slack.sh
# .env 의 SLACK_WEBHOOK_URL 을 읽어 테스트 메시지 한 통을 보낸다.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
	echo ".env 가 없습니다. cp .env.example .env 부터 하세요."
	exit 1
fi

# .env 에서 값만 뽑는다 (source 하면 따옴표 없는 JSON 값에서 깨진다)
url="$(grep -E '^SLACK_WEBHOOK_URL=' .env | head -1 | cut -d= -f2-)"

if [ -z "$url" ]; then
	echo "SLACK_WEBHOOK_URL 이 비어 있습니다."
	exit 1
fi

echo "테스트 메시지를 보냅니다..."
code="$(curl -s -o /tmp/clawhub-slack-resp -w '%{http_code}' -X POST \
	-H 'Content-Type: application/json' \
	-d '{"text":"claw-hub 연결 테스트 — 이 메시지가 보이면 알림 설정이 끝난 겁니다."}' \
	"$url")"

if [ "$code" = "200" ]; then
	echo "성공. Slack 채널을 확인하세요."
else
	echo "실패 (HTTP $code): $(cat /tmp/clawhub-slack-resp)"
	echo
	echo "  invalid_token / no_service  → 웹훅 URL 이 폐기됐거나 오타입니다. 다시 발급하세요."
	echo "  channel_not_found           → 웹훅이 걸린 채널이 삭제됐습니다."
	exit 1
fi
