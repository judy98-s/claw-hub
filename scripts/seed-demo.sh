#!/usr/bin/env bash
#
# 데모 데이터를 넣는다. 기계 몇 대와 민원 여러 건, 처리된 건까지.
#
# **서버에서, claw-hub 디렉터리에서 실행한다.**
#
#   bash scripts/seed-demo.sh owner@example.com '비밀번호'
#
# 왜 필요한가: 빈 대시보드는 아무것도 말해주지 않는다. 목록이 어떻게
# 보이는지, 기계 이상 알림이 언제 뜨는지, 승인한 건이 어디로 가는지는
# 데이터가 있어야 보인다.
#
# 넣는 것
#   기계 3대 (없으면 만든다)
#   민원 9건 — 현금·카드, 금액 다양, 고액 1건(확인 필요로 떨어진다)
#   1번 기계에 4건을 몰아 기계 이상 알림을 띄운다
#   2건은 승인까지 해 둔다 (송금 대기 탭을 채운다)
#
# 이미 데이터가 있으면 그 위에 더한다. 지우지 않는다.

set -u

EMAIL="${1:-}"
PASSWORD="${2:-}"
BASE="${BASE_URL:-http://localhost:8080}"

die() { printf '\033[0;31m%s\033[0m\n' "$*" >&2; exit 1; }
say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

[ -n "$EMAIL" ] && [ -n "$PASSWORD" ] || die "사용법: bash scripts/seed-demo.sh <이메일> <비밀번호>

비밀번호가 셸 히스토리에 남는 게 싫으면 먼저 읽어서 넘긴다:
  read -rs PW && bash scripts/seed-demo.sh you@example.com \"\$PW\""

# 도커로 돌고 있으면 api 컨테이너를 통해 부른다. 호스트에는 8080 이
# 열려 있지 않다 — compose 가 포트를 publish 하지 않기 때문이다.
CURL="curl"
if ! curl -s -o /dev/null --max-time 3 "$BASE/healthz" 2>/dev/null; then
  if docker compose -f docker-compose.prod.yml ps api >/dev/null 2>&1; then
    say "호스트에서 api 에 닿지 않는다 — Caddy 를 통해 부른다"
    SITE=$(grep -m1 '^SITE_ADDRESS=' .env 2>/dev/null | cut -d= -f2-)
    [ -n "$SITE" ] || die ".env 의 SITE_ADDRESS 를 읽지 못했다."
    BASE="https://$SITE"
  fi
fi
curl -s -o /dev/null --max-time 10 "$BASE/healthz" || die "$BASE 에 닿지 않는다."
say "서버 $BASE"

JAR=$(mktemp); code=$($CURL -s -c "$JAR" -o /dev/null -w '%{http_code}' -X POST "$BASE/api/admin/login" \
  -H 'Content-Type: application/json' \
  --data-binary "$(printf '{"email":%s,"password":%s}' \
    "$(printf '%s' "$EMAIL" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
    "$(printf '%s' "$PASSWORD" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')")")
[ "$code" = "200" ] || die "로그인 실패 (HTTP $code). 이메일과 비밀번호를 확인하라."
say "로그인 완료"

machines() {
  $CURL -s -b "$JAR" "$BASE/api/admin/machines" | python3 -c "
import sys,json
d=json.load(sys.stdin); items = d if isinstance(d,list) else d.get('machines',d.get('items',[]))
print(' '.join(m['code'] for m in items))"
}

CODES=$(machines)
if [ "$(printf '%s' "$CODES" | wc -w)" -lt 3 ]; then
  say "기계 만들기"
  i=$(printf '%s' "$CODES" | wc -w)
  while [ "$i" -lt 3 ]; do
    i=$((i+1))
    $CURL -s -b "$JAR" -o /dev/null -X POST "$BASE/api/admin/machines" \
      -H 'Content-Type: application/json' \
      -d "{\"label\":\"${i}번 기계\",\"location\":\"데모\"}"
  done
  CODES=$(machines)
fi
read -r M1 M2 M3 <<< "$CODES"
say "기계: $M1 $M2 $M3"

# 고액 건에는 사진이 필요하다 (기본 1만원 이상). 데모에 "확인 필요" 건이
# 하나는 있어야 리스크 상자와 사진 보기가 어떻게 보이는지 알 수 있어서,
# 1×1 PNG 를 만들어 붙인다.
PHOTO=$(mktemp /tmp/clawhub-demo-XXXX.png)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\xcf\xc0\x00\x00\x03\x01\x01\x00\x18\xdd\x8d\xb0\x00\x00\x00\x00IEND\xaeB`\x82' > "$PHOTO"
trap 'rm -f "$JAR" "$PHOTO"' EXIT

n=0
seed() {
  n=$((n+1))
  local code=$1 issue=$2 amt=$3 phone=$4 pay=$5; shift 5
  local args=(); for kv in "$@"; do args+=(-F "$kv"); done
  local r
  r=$($CURL -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/public/claims" \
    -H "Idempotency-Key: demo-$(date +%s)-$n-$RANDOM" \
    -F "machineCode=$code" -F "issueType=$issue" -F "amountKrw=$amt" \
    -F "phone=$phone" -F "paymentMethod=$pay" "${args[@]}")
  case "$r" in
    201) printf '  %-28s \033[0;32m접수\033[0m\n' "$issue $amt원" ;;
    # 409 는 오류가 아니다. 같은 번호로 방금 들어온 건을 막는 중복 방지가
    # 동작한 것이고, 스크립트를 두 번 돌리면 당연히 나온다.
    409) printf '  %-28s \033[0;33m건너뜀 (이미 있음)\033[0m\n' "$issue $amt원" ;;
    429) printf '  %-28s \033[0;33m레이트리밋 — 한 시간 뒤 다시\033[0m\n' "$issue $amt원" ;;
    *)   printf '  %-28s \033[0;31mHTTP %s\033[0m\n' "$issue $amt원" "$r" ;;
  esac
}

say "민원 넣기"
# 1번 기계에 네 건 — 기계 이상 알림(기본 임계치 3건)이 뜬다
seed "$M1" cash_eaten  1000 01099990001 cash bankCode=004 accountNo=12345678901 holder=김민지
seed "$M1" doll_stuck  2000 01099990002 card cardLast4=4321
seed "$M1" claw_broken 1000 01099990003 cash bankCode=092 accountNo=3333041234567 holder=이서준
seed "$M1" cash_eaten  3000 01099990004 card cardLast4=8812
# 나머지 기계
seed "$M2" cash_eaten  2000 01099990005 cash bankCode=090 accountNo=100012345678 holder=박하준
seed "$M2" other       1500 01099990006 card cardLast4=1111
seed "$M3" doll_stuck  5000 01099990007 cash bankCode=088 accountNo=110123456789 holder=최유나
# 고액 — 확인 필요로 떨어진다. 1만원 이상은 사진이 있어야 접수된다.
seed "$M3" cash_eaten 30000 01099990008 cash bankCode=004 accountNo=98765432109 holder=정우진 \
  "photos=@$PHOTO;type=image/png"
seed "$M3" claw_broken 1000 01099990009 card cardLast4=7777

say "두 건은 승인까지"
IDS=$($CURL -s -b "$JAR" "$BASE/api/admin/claims?status=pending" | python3 -c "
import sys,json
d=json.load(sys.stdin); items = d if isinstance(d,list) else d.get('claims',d.get('items',[]))
print(' '.join(c['id'] for c in items[:2]))" 2>/dev/null)
for id in $IDS; do
  r=$($CURL -s -b "$JAR" -o /dev/null -w '%{http_code}' -X POST "$BASE/api/admin/claims/$id/approve" \
    -H 'Content-Type: application/json' -d '{}')
  printf '  승인 %s\n' "$r"
done

cat <<EOF

────────────────────────────────────────────────────────────────
 완료. 대시보드에서 확인한다.

   /admin          기계 이상 알림이 떠 있어야 한다
   /admin/claims   처리 대기 · 송금 대기 탭에 나뉘어 있다
   /admin/machines 1번 기계에 "점검 필요" 배지

 레이트리밋(IP 당 시간당 10건)에 걸렸으면 한 시간 뒤 다시 돌린다.
 지우고 다시 하려면 Postgres 에서 claims 를 비운다:

   docker compose -f docker-compose.prod.yml exec -T postgres \\
     psql -U clawhub -d clawhub -c "DELETE FROM claims;"
────────────────────────────────────────────────────────────────
EOF
