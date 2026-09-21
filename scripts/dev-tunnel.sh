#!/usr/bin/env bash
#
# 내 컴퓨터에서 돌리는 claw-hub 에 공개 HTTPS 주소를 붙인다.
#
# 손님 폰이 QR 을 찍으면 localhost 는 자기 폰을 가리킨다. Slack 알림 링크도
# 폰에서는 안 열린다. 터널이 그 둘을 해결한다.
#
# 테스트용이다. 컴퓨터를 끄면 주소가 죽고, 다시 켜면 주소가 바뀌어서
# 이미 붙여놓은 QR 스티커가 전부 무효가 된다. 실제 운영은 VPS 로 옮겨야 한다.
set -euo pipefail

PORT="${PORT:-3000}"

if ! command -v cloudflared >/dev/null 2>&1; then
	cat <<'MSG'
cloudflared 가 없습니다. 설치하세요:

  macOS   brew install cloudflared
  Linux   https://github.com/cloudflare/cloudflared/releases 에서 받기
  Windows winget install --id Cloudflare.cloudflared

회원가입이나 로그인은 필요 없습니다.
MSG
	exit 1
fi

echo "터널을 엽니다 (localhost:${PORT})..."
echo

cloudflared tunnel --url "http://localhost:${PORT}" 2>&1 | while IFS= read -r line; do
	echo "$line"
	if [[ "$line" =~ (https://[a-z0-9-]+\.trycloudflare\.com) ]]; then
		url="${BASH_REMATCH[1]}"
		cat <<MSG

────────────────────────────────────────────────────────────
공개 주소: ${url}

이제 .env 를 고치고 API 를 다시 띄우세요:

  PUBLIC_BASE_URL=${url}

QR 과 Slack 링크가 이 주소로 만들어집니다. 이미 만들어둔 QR 은 예전
주소를 가리키니, 기계 화면에서 QR 을 다시 받거나 손님에게
${url}/r 에서 코드를 입력하게 하세요.
────────────────────────────────────────────────────────────

MSG
	fi
done
