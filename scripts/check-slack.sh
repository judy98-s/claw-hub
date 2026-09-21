#!/usr/bin/env bash
#
# Slack 웹훅이 살아 있는지 확인한다.
#
# 사용법:  make check-slack
# .env 의 SLACK_WEBHOOK_URL 을 읽어 테스트 메시지 한 통을 보낸다.
#
# 이 스크립트가 필요한 이유: 웹훅이 죽어도 증상이 조용하다. 접수는 정상이고
# 대시보드에도 뜨는데 알림만 안 온다 — 알림 실패가 접수 실패로 번지지 않게
# 설계했기 때문이다. 그래서 설정 직후 한 번 확인할 방법이 있어야 한다.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
	echo "✗ .env 가 없습니다."
	echo "  cp .env.example .env  부터 하세요."
	exit 1
fi

# .env 에서 값만 뽑는다. source 하면 따옴표 없는 JSON 값에서 깨진다.
url="$(grep -E '^SLACK_WEBHOOK_URL=' .env | head -1 | cut -d= -f2- || true)"
url="${url%\"}"
url="${url#\"}"

if [ -z "$url" ]; then
	echo "✗ .env 의 SLACK_WEBHOOK_URL 이 비어 있습니다."
	echo
	echo "  Slack 워크스페이스 → 앱 만들기 → Incoming Webhooks 켜기 →"
	echo "  알림 받을 채널 고르기 → 나온 URL 을 .env 에 붙여넣으세요."
	echo
	echo "  알림 채널은 비공개로 만들고 사장님만 두세요. 그 채널에 있는"
	echo "  사람은 누구나 알림 링크로 손님 계좌를 볼 수 있습니다."
	exit 1
fi

echo "테스트 메시지를 보냅니다..."

resp="$(mktemp)"
trap 'rm -f "$resp"' EXIT

# curl 실패(연결 거부·DNS·타임아웃)를 직접 잡는다.
# set -e 에 맡기면 스크립트가 아무 말 없이 죽어서, 정작 원인을 알려줘야 할
# 스크립트가 침묵한다.
if ! code="$(curl -s -o "$resp" -w '%{http_code}' --max-time 15 -X POST \
	-H 'Content-Type: application/json' \
	-d '{"text":"claw-hub 연결 테스트 — 이 메시지가 보이면 알림 설정이 끝난 겁니다."}' \
	"$url")"; then
	echo "✗ Slack 에 연결하지 못했습니다."
	echo
	echo "  인터넷 연결을 확인하고, URL 이 https://hooks.slack.com/ 으로"
	echo "  시작하는지 보세요. 회사 방화벽이 막고 있을 수도 있습니다."
	exit 1
fi

body="$(cat "$resp")"

case "$code" in
200)
	echo "✓ 성공. Slack 채널을 확인하세요."
	;;
404)
	echo "✗ 웹훅을 찾을 수 없습니다 (HTTP 404): $body"
	echo "  URL 에 오타가 있거나 웹훅이 삭제됐습니다. Slack 앱 설정에서 다시 발급하세요."
	exit 1
	;;
403)
	echo "✗ 거부되었습니다 (HTTP 403): $body"
	echo "  웹훅이 폐기됐거나 앱이 워크스페이스에서 제거됐습니다."
	exit 1
	;;
*)
	echo "✗ 실패 (HTTP $code): $body"
	echo
	echo "  invalid_payload   → 이 스크립트의 버그입니다. 알려주세요."
	echo "  channel_not_found → 웹훅이 걸린 채널이 삭제됐습니다."
	echo "  no_service        → 웹훅이 폐기됐습니다. 다시 발급하세요."
	exit 1
	;;
esac
