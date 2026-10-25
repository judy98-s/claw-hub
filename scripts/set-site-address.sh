#!/usr/bin/env bash
#
# 공인 IP 를 읽어 .env 의 SITE_ADDRESS 와 PUBLIC_BASE_URL 을 nip.io 주소로
# 맞춘다. **서버에서, claw-hub 디렉터리에서 실행한다.**
#
#   bash scripts/set-site-address.sh                 # IP 를 자동으로 찾는다
#   bash scripts/set-site-address.sh 152.67.89.123   # 직접 준다
#
# 왜 스크립트인가: 예전에는 안내문에 curl 한 줄을 적어 두었는데, 그 curl 이
# 실패하면 변수가 빈 채로 넘어가 SITE_ADDRESS=.nip.io 가 들어갔다. Caddy 가
# 유효하지 않은 주소로 재시작만 반복하고, 원인은 .env 를 열어 보기 전까지
# 보이지 않았다. 여기서는 IP 형식을 검증하고, 틀리면 .env 를 건드리지 않는다.

set -euo pipefail

die() { printf '\033[0;31m%s\033[0m\n' "$*" >&2; exit 1; }

[ -f .env ] || die ".env 가 없다. claw-hub 디렉터리에서 실행하라."

IP="${1:-}"

if [ -z "$IP" ]; then
  # 오라클 인스턴스 메타데이터. 외부 의존이 없어서 이쪽을 먼저 쓴다.
  IP=$(curl -s --max-time 5 -H "Authorization: Bearer Oracle" \
        http://169.254.169.254/opc/v2/vnics/ 2>/dev/null \
       | python3 -c "import sys,json;print(json.load(sys.stdin)[0].get('publicIp',''))" \
       2>/dev/null || true)
fi
if [ -z "$IP" ]; then
  IP=$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null || true)
fi

# 검증을 통과하지 못하면 .env 를 건드리지 않고 끝낸다. 빈 값이 그대로
# 넘어가 SITE_ADDRESS=.nip.io 가 되는 것을 막는 것이 이 검증의 요점이다.
bad_ip() {
  die "공인 IP 를 읽지 못했다 (받은 값: '${IP}'). 콘솔에서 확인한 IP 를 인자로 넘겨라:
    bash scripts/set-site-address.sh 152.67.89.123"
}
printf '%s' "$IP" | grep -Eq '^[0-9]{1,3}(\.[0-9]{1,3}){3}$' || bad_ip

# 옥텟 범위까지 본다. IP 는 인자로도 받으므로 형식만으로는 부족하다.
IFS=. read -r o1 o2 o3 o4 <<<"$IP"
for o in "$o1" "$o2" "$o3" "$o4"; do
  [ "$o" -le 255 ] 2>/dev/null || bad_ip
done
# 0.0.0.0 이나 사설 대역은 공인 IP 가 아니다. Let's Encrypt 가 닿지 못한다.
case "$IP" in
  0.*|10.*|127.*|169.254.*|192.168.*|172.1[6-9].*|172.2[0-9].*|172.3[01].*)
    die "'$IP' 는 공인 IP 가 아니다 (사설 대역). 오라클 콘솔의
    Public IP Address 값을 인자로 넘겨라 — Private IP 가 아니다." ;;
esac

HOST="$(printf '%s' "$IP" | tr '.' '-').nip.io"

sed -i -e '/^SITE_ADDRESS=/d' -e '/^PUBLIC_BASE_URL=/d' .env
{
  echo "SITE_ADDRESS=$HOST"
  echo "PUBLIC_BASE_URL=https://$HOST"
} >> .env

cat <<EOF

 공인 IP   $IP
 주소      https://$HOST

 적용:
   docker compose -f docker-compose.prod.yml up -d
   docker compose -f docker-compose.prod.yml logs caddy | grep -i certificate

 "certificate obtained successfully" 가 보이면 발급 성공이다 (10~30초).

 **주소를 바꾸면 이미 뽑은 QR 스티커는 전부 죽는다.** 공인 IP 를 예약으로
 바꿔 두지 않았다면 인스턴스를 재시작하지 않도록 주의한다.
EOF
