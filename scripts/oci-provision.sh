#!/usr/bin/env bash
#
# Oracle Cloud 에 claw-hub 용 Always Free 인스턴스를 만든다.
#
# **Oracle Cloud Shell 에서 실행한다.** 콘솔 우측 상단의 >_ 아이콘이다.
# Cloud Shell 은 이미 내 계정으로 인증되어 있어서 API 키가 필요 없다.
#
#   bash oci-provision.sh              # E2.1.Micro (1 OCPU / 1GB) — 기본
#   SHAPE=arm bash oci-provision.sh    # A1.Flex (2 OCPU / 12GB) — 재고가 있으면
#
# 기본이 micro 인 이유: ARM(A1.Flex)은 무료 한도가 훨씬 크지만 재고가 거의
# 없어서 Out of host capacity 가 계속 난다. 홈 리전은 가입 후 바꿀 수 없고
# Always Free 는 홈 리전에서만 되므로 리전을 옮기는 것도 답이 아니다.
# E2.1.Micro 는 AMD 무료 셰이프로 재고가 거의 항상 있다.
#
# 1GB 로도 돌아간다. Redis 를 띄우지 않고(인메모리 폴백), Postgres 를 낮추고,
# 스왑 2GB 를 둔다. 합계 약 500MB 다. 다만 Next.js 빌드는 1.5GB 넘게 먹어서
# 서버에서 빌드하지 않고 GitHub Actions 가 만든 GHCR 이미지를 받아 쓴다
# (docker-compose.prod.yml).
#
# 내 PC 에 이미 쓰던 공개키가 있으면 같이 넣을 수 있다. 그래야 Cloud Shell
# 과 내 PC 양쪽에서 접속된다:
#
#   EXTRA_SSH_PUBKEY='ssh-ed25519 AAAA... clawhub' bash oci-provision.sh
#
# 만드는 것 (전부 Always Free):
#   - VCN 1개                       10.0.0.0/16
#   - 인터넷 게이트웨이 1개
#   - public 서브넷 1개              10.0.0.0/24
#   - 보안 규칙                      inbound 22/80/443, outbound 전체
#   - 인스턴스 1개                   위 셰이프, Ubuntu 24.04, 부트볼륨 50GB
#
# 인스턴스는 cloud-init 으로 부팅할 때 이것까지 해 둔다:
#   - 스왑 2GB
#   - OS 방화벽에 80/443 허용 (오라클은 VCN 과 OS 두 군데에 방화벽이 있다)
#   - Docker 설치 + ubuntu 사용자를 docker 그룹에
#   - make 설치 (Ubuntu 클라우드 이미지에는 없다)
#
# 만들지 않는 것 (콘솔 마법사는 만든다):
#   - NAT 게이트웨이, 서비스 게이트웨이, private 서브넷. 이 구성에 필요가
#     없고, 안 만드는 쪽이 무료 한도에 안전하다.
#   - 예약 공인 IP. 무료 한도가 테넌시마다 달라서 따로 둔다. 스크립트가
#     끝나면 안내한다.
#
# 여러 번 돌려도 된다. 이미 있는 것은 건너뛴다.

set -euo pipefail

NAME=clawhub
VCN_CIDR=10.0.0.0/16
SUBNET_CIDR=10.0.0.0/24
BOOT_GB=50
SWAP_GB=2
SSH_KEY="$HOME/.ssh/${NAME}"

# SHAPE=micro | arm
case "${SHAPE:-micro}" in
  micro)
    # 고정 셰이프다. --shape-config 를 주면 API 가 거부한다.
    OCI_SHAPE="VM.Standard.E2.1.Micro"
    SHAPE_CONFIG=""
    SHAPE_LABEL="E2.1.Micro (1 OCPU / 1GB, AMD)"
    ;;
  arm)
    # 가변 셰이프다. 무료 한도가 정확히 2 OCPU / 12GB 다.
    OCI_SHAPE="VM.Standard.A1.Flex"
    SHAPE_CONFIG='{"ocpus":2,"memoryInGBs":12}'
    SHAPE_LABEL="A1.Flex (2 OCPU / 12GB, ARM)"
    ;;
  *)
    echo "SHAPE 는 micro 또는 arm 이다. 받은 값: ${SHAPE}" >&2
    exit 1
    ;;
esac

say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
ok()  { printf '    \033[0;32m%s\033[0m\n' "$*"; }
warn(){ printf '    \033[0;33m%s\033[0m\n' "$*"; }
die() { printf '\n\033[0;31m실패: %s\033[0m\n' "$*" >&2; exit 1; }

# ── 0. 환경 확인 ─────────────────────────────────────────────────────
command -v oci >/dev/null || die "oci CLI 가 없다. Cloud Shell 에서 실행하고 있는지 확인하라."
command -v jq  >/dev/null || die "jq 가 없다. Cloud Shell 에서 실행하고 있는지 확인하라."

# Cloud Shell 은 OCI_TENANCY 를 설정해 준다. 루트 컴파트먼트를 쓴다.
COMP="${OCI_TENANCY:-}"
if [ -z "$COMP" ]; then
  COMP=$(oci iam compartment list --query 'data[0]."compartment-id"' --raw-output 2>/dev/null || true)
fi
[ -n "$COMP" ] || die "컴파트먼트를 찾지 못했다. Cloud Shell 이 아닌 환경이면 OCI_TENANCY 를 직접 설정하라."

REGION="${OCI_REGION:-$(oci iam region-subscription list --query 'data[0]."region-name"' --raw-output 2>/dev/null || echo '?')}"
say "리전 $REGION / 셰이프 $SHAPE_LABEL"
ok  "컴파트먼트 ${COMP:0:30}..."

# ── 1. SSH 키 ────────────────────────────────────────────────────────
#
# Cloud Shell 은 FIPS 모드로 돌아서 ed25519 를 거부한다
# ("ED25519 keys are not allowed in FIPS mode"). RSA 4096 은 FIPS 에서도
# 허용되고 OCI 가 권장하는 형식이다. 다른 환경에서 돌릴 때는 ed25519 를
# 먼저 시도하고, 거부당하면 RSA 로 내려간다.
say "SSH 키"
mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"
if [ -f "${SSH_KEY}.pub" ]; then
  ok "이미 있다: ${SSH_KEY}.pub"
else
  if ssh-keygen -t ed25519 -f "$SSH_KEY" -N '' -C "$NAME" >/dev/null 2>&1; then
    ok "새로 만들었다 (ed25519): ${SSH_KEY}"
  else
    rm -f "$SSH_KEY" "${SSH_KEY}.pub"
    ssh-keygen -t rsa -b 4096 -f "$SSH_KEY" -N '' -C "$NAME" >/dev/null \
      || die "SSH 키 생성에 실패했다."
    ok "새로 만들었다 (RSA 4096 — FIPS 모드): ${SSH_KEY}"
  fi
  warn "개인키 ${SSH_KEY} 는 이 Cloud Shell 안에만 있다."
  warn "내 PC 에서도 접속하려면 맨 끝 안내를 본다."
fi

# 인스턴스에 넣을 공개키 목록. 여러 줄을 넣으면 그 키들 전부로 접속된다.
# 내 PC 키를 EXTRA_SSH_PUBKEY 로 주면 양쪽에서 들어갈 수 있다.
AUTH_KEYS=$(mktemp)
cat "${SSH_KEY}.pub" > "$AUTH_KEYS"
if [ -n "${EXTRA_SSH_PUBKEY:-}" ]; then
  case "$EXTRA_SSH_PUBKEY" in
    ssh-*|ecdsa-*) printf '%s\n' "$EXTRA_SSH_PUBKEY" >> "$AUTH_KEYS"
                   ok "EXTRA_SSH_PUBKEY 도 함께 넣는다" ;;
    *) die "EXTRA_SSH_PUBKEY 가 공개키 형식이 아니다. ssh- 로 시작해야 한다. 개인키를 넣지 않았는지 확인하라." ;;
  esac
fi

# ── 1-2. cloud-init ──────────────────────────────────────────────────
#
# 부팅할 때 한 번 돌아간다. 여기서 해 두면 SSH 로 들어가 손으로 할 일이
# 줄고, 특히 OS 방화벽을 빼먹는 실수를 막는다 — 오라클은 VCN 과 OS 두
# 군데에 방화벽이 있어서 한 군데만 열고 왜 안 되는지 찾는 일이 흔하다.
say "cloud-init (스왑 ${SWAP_GB}GB + 방화벽 + Docker + make)"
USER_DATA=$(mktemp)
cat > "$USER_DATA" <<CLOUDINIT
#cloud-config
# 1GB 장비에서는 스왑이 없으면 docker compose pull 중에도 OOM 이 난다.
swap:
  filename: /swapfile
  size: $((SWAP_GB * 1024 * 1024 * 1024))
  maxsize: $((SWAP_GB * 1024 * 1024 * 1024))

package_update: true

runcmd:
  # Ubuntu 기본 iptables 가 80/443 을 막는다. VCN 쪽만 열어도 안 뚫린다.
  - [ sh, -c, "iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT" ]
  - [ sh, -c, "iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT" ]
  - [ sh, -c, "command -v netfilter-persistent >/dev/null || DEBIAN_FRONTEND=noninteractive apt-get install -y iptables-persistent" ]
  # 저장하지 않으면 재부팅하면 원복된다.
  - [ sh, -c, "netfilter-persistent save" ]
  - [ sh, -c, "curl -fsSL https://get.docker.com | sh" ]
  - [ sh, -c, "usermod -aG docker ubuntu" ]
  # Ubuntu 클라우드 이미지에는 make 가 없다. Makefile 타깃(make keys,
  # make prod-up)이 전부 안 돌아서 .env 가 빈 채로 남는다.
  - [ sh, -c, "DEBIAN_FRONTEND=noninteractive apt-get install -y make" ]
  - [ sh, -c, "echo clawhub-cloudinit-done > /var/log/clawhub-ready" ]
CLOUDINIT
ok "준비했다"

# ── 2. Ubuntu 24.04 ARM 이미지 ───────────────────────────────────────
say "Ubuntu 24.04 이미지 찾기 ($OCI_SHAPE)"
IMAGE_ID=$(oci compute image list \
  --compartment-id "$COMP" \
  --operating-system "Canonical Ubuntu" \
  --operating-system-version "24.04" \
  --shape "$OCI_SHAPE" \
  --sort-by TIMECREATED --sort-order DESC \
  --query 'data[0].id' --raw-output 2>/dev/null || true)
[ -n "$IMAGE_ID" ] && [ "$IMAGE_ID" != "null" ] || die "$OCI_SHAPE 용 Ubuntu 24.04 이미지를 찾지 못했다."
ok "${IMAGE_ID:0:40}..."

# ── 3. VCN ───────────────────────────────────────────────────────────
say "VCN"
VCN_ID=$(oci network vcn list --compartment-id "$COMP" --display-name "$NAME-vcn" \
  --query 'data[0].id' --raw-output 2>/dev/null || true)
if [ -n "$VCN_ID" ] && [ "$VCN_ID" != "null" ]; then
  ok "이미 있다"
else
  VCN_ID=$(oci network vcn create --compartment-id "$COMP" \
    --cidr-blocks "[\"$VCN_CIDR\"]" \
    --display-name "$NAME-vcn" --dns-label "$NAME" \
    --query 'data.id' --raw-output)
  ok "만들었다"
fi

RT_ID=$(oci network vcn get --vcn-id "$VCN_ID" --query 'data."default-route-table-id"' --raw-output)
SL_ID=$(oci network vcn get --vcn-id "$VCN_ID" --query 'data."default-security-list-id"' --raw-output)

# ── 4. 인터넷 게이트웨이 + 라우트 ─────────────────────────────────────
say "인터넷 게이트웨이"
IGW_ID=$(oci network internet-gateway list --compartment-id "$COMP" --vcn-id "$VCN_ID" \
  --query 'data[0].id' --raw-output 2>/dev/null || true)
if [ -n "$IGW_ID" ] && [ "$IGW_ID" != "null" ]; then
  ok "이미 있다"
else
  IGW_ID=$(oci network internet-gateway create --compartment-id "$COMP" \
    --vcn-id "$VCN_ID" --is-enabled true --display-name "$NAME-igw" \
    --query 'data.id' --raw-output)
  ok "만들었다"
fi

say "라우트 규칙 (0.0.0.0/0 → 인터넷 게이트웨이)"
oci network route-table update --rt-id "$RT_ID" --force \
  --route-rules "[{\"cidrBlock\":\"0.0.0.0/0\",\"networkEntityId\":\"$IGW_ID\"}]" >/dev/null
ok "넣었다"

# ── 5. 보안 규칙 ─────────────────────────────────────────────────────
# update 는 규칙을 **교체**한다. egress 를 같이 안 넘기면 서버가 밖으로
# 못 나가서 apt 도 docker pull 도 안 된다.
say "보안 규칙 (inbound 22/80/443)"
oci network security-list update --security-list-id "$SL_ID" --force \
  --ingress-security-rules '[
    {"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":22,"max":22}}},
    {"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":80,"max":80}}},
    {"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":443,"max":443}}},
    {"protocol":"1","source":"0.0.0.0/0","icmpOptions":{"type":3,"code":4}}
  ]' \
  --egress-security-rules '[{"protocol":"all","destination":"0.0.0.0/0"}]' >/dev/null
ok "22 / 80 / 443 열었다. 5432·6379 는 열지 않는다"

# ── 6. public 서브넷 ─────────────────────────────────────────────────
say "public 서브넷"
SUBNET_ID=$(oci network subnet list --compartment-id "$COMP" --vcn-id "$VCN_ID" \
  --display-name "$NAME-public" --query 'data[0].id' --raw-output 2>/dev/null || true)
if [ -n "$SUBNET_ID" ] && [ "$SUBNET_ID" != "null" ]; then
  ok "이미 있다"
else
  SUBNET_ID=$(oci network subnet create --compartment-id "$COMP" --vcn-id "$VCN_ID" \
    --cidr-block "$SUBNET_CIDR" --display-name "$NAME-public" \
    --dns-label public --prohibit-public-ip-on-vnic false \
    --query 'data.id' --raw-output)
  ok "만들었다 (공인 IP 허용)"
fi

# ── 7. 인스턴스 ──────────────────────────────────────────────────────
say "기존 인스턴스 확인"
INST_ID=$(oci compute instance list --compartment-id "$COMP" --display-name "$NAME" \
  --lifecycle-state RUNNING --query 'data[0].id' --raw-output 2>/dev/null || true)

if [ -n "$INST_ID" ] && [ "$INST_ID" != "null" ]; then
  ok "이미 돌고 있다 — 새로 만들지 않는다"
else
  mapfile -t ADS < <(oci iam availability-domain list --compartment-id "$COMP" | jq -r '.data[].name')
  [ "${#ADS[@]}" -gt 0 ] || die "가용 도메인을 찾지 못했다."

  # 무료 계정의 LaunchInstance 는 두 가지 이유로 자주 거절된다.
  #
  #   Out of host capacity (500) — 그 순간 그 AD 에 ARM 재고가 없다
  #   TooManyRequests   (429)    — 오라클이 요청 속도를 제한한다
  #
  # 둘 다 설정 문제가 아니고, 기다렸다 다시 부르면 통한다. 429 는 특히
  # 무료 계정에 공격적으로 걸리고 과거 시도 횟수까지 누적된다. 그래서
  # 치명적 오류로 보지 않고 백오프를 두고 재시도한다.
  MAX_ATTEMPTS="${MAX_ATTEMPTS:-6}"
  say "인스턴스 만들기 — 가용 도메인 ${#ADS[@]}개 × 최대 ${MAX_ATTEMPTS}회"
  LOG=$(mktemp)
  BACKOFF=90

  for ((attempt = 1; attempt <= MAX_ATTEMPTS; attempt++)); do
    THROTTLED=0
    for AD in "${ADS[@]}"; do
      printf '    [%d/%d] %s ... ' "$attempt" "$MAX_ATTEMPTS" "$AD"
      # E2.1.Micro 는 고정 셰이프라 --shape-config 를 주면 API 가 거부한다.
      LAUNCH_ARGS=(
        --compartment-id "$COMP"
        --availability-domain "$AD"
        --subnet-id "$SUBNET_ID"
        --image-id "$IMAGE_ID"
        --shape "$OCI_SHAPE"
        --boot-volume-size-in-gbs "$BOOT_GB"
        --display-name "$NAME"
        --assign-public-ip true
        --ssh-authorized-keys-file "$AUTH_KEYS"
        --user-data-file "$USER_DATA"
        --wait-for-state RUNNING
      )
      [ -n "$SHAPE_CONFIG" ] && LAUNCH_ARGS+=(--shape-config "$SHAPE_CONFIG")

      if oci compute instance launch "${LAUNCH_ARGS[@]}" > "$LOG" 2>&1; then
        INST_ID=$(jq -r '.data.id' "$LOG")
        printf '\033[0;32m성공\033[0m\n'
        break 2
      fi

      if grep -qi 'TooManyRequests\|"status": *429' "$LOG"; then
        printf '\033[0;33m요청 제한 (429)\033[0m\n'
        THROTTLED=1
      elif grep -qi 'out of host capacity\|OutOfCapacity\|OutOfHostCapacity' "$LOG"; then
        printf '\033[0;33m재고 없음\033[0m\n'
      elif grep -qi 'LimitExceeded\|QuotaExceeded' "$LOG"; then
        printf '\033[0;31m한도 초과\033[0m\n'
        tail -25 "$LOG" >&2
        die "무료 한도를 넘었다. 이미 떠 있는 인스턴스가 한도를 쓰고 있지 않은지 확인하라."
      else
        printf '\033[0;31m오류\033[0m\n'
        tail -25 "$LOG" >&2
        die "예상하지 못한 오류다. 위 내용을 보내라."
      fi
    done

    [ "$attempt" -lt "$MAX_ATTEMPTS" ] || break

    # 429 는 속도 제한이므로 더 길게 쉰다. 재고 부족은 다음 분에 풀릴
    # 수도 있어서 짧게 쉰다.
    WAIT=$BACKOFF
    [ "$THROTTLED" -eq 1 ] && WAIT=$((BACKOFF * 2))
    [ "$WAIT" -gt 600 ] && WAIT=600
    warn "${WAIT}초 쉬고 다시 시도한다. (Ctrl+C 로 중단해도 다시 돌리면 이어진다)"
    sleep "$WAIT"
    BACKOFF=$((BACKOFF * 2))
  done

  if [ -z "${INST_ID:-}" ] || [ "$INST_ID" = "null" ]; then
    warn "아직 못 만들었다. 계정이나 설정 문제가 아니다."
    warn ""
    warn "  429 요청 제한이었다면 — 30분 ~ 1시간 뒤에 다시 돌린다."
    warn "  재고 없음이었다면   — 몇 시간 뒤, 한국 새벽에 잘 풀린다."
    if [ "${SHAPE:-micro}" = "arm" ]; then
      warn ""
      warn "  ARM 재고는 며칠씩 없을 수도 있다. 지금 띄우려면 micro 로 간다:"
      warn "      bash oci-provision.sh"
      warn "  (E2.1.Micro 1 OCPU/1GB. 이 앱은 그 사양으로 돌아간다.)"
    fi
    warn ""
    warn "다시 돌릴 때 이미 만든 VCN·서브넷·보안규칙은 건너뛰고 인스턴스만"
    warn "시도한다. 더 오래 재시도하려면 MAX_ATTEMPTS 를 올린다:"
    warn ""
    warn "  MAX_ATTEMPTS=20 bash oci-provision.sh"
    exit 2
  fi
fi

# ── 8. 공인 IP ───────────────────────────────────────────────────────
say "공인 IP"
PUBLIC_IP=$(oci compute instance list-vnics --instance-id "$INST_ID" \
  --query 'data[0]."public-ip"' --raw-output)
[ -n "$PUBLIC_IP" ] && [ "$PUBLIC_IP" != "null" ] || die "공인 IP 가 붙지 않았다. 서브넷 설정을 확인하라."
NIP="${PUBLIC_IP//./-}.nip.io"
ok "$PUBLIC_IP"

# ── 끝 ──────────────────────────────────────────────────────────────
cat <<EOF

────────────────────────────────────────────────────────────────
 완료
────────────────────────────────────────────────────────────────

 공인 IP      $PUBLIC_IP
 셰이프        $SHAPE_LABEL
 접속 주소     https://$NIP
 인스턴스      ${INST_ID:0:40}...

 cloud-init 이 부팅 중에 스왑·방화벽·Docker 를 설치한다. **2~3분 걸린다.**
 끝났는지 확인하는 방법은 아래 1번에 있다.

 ── 1. 접속하고 cloud-init 이 끝났는지 확인 ─────────────────────────

   ssh -i $SSH_KEY ubuntu@$PUBLIC_IP

   cat /var/log/clawhub-ready     # clawhub-cloudinit-done 이 나오면 완료
   docker --version               # 안 되면 아직이다. 1분 뒤 다시
   make --version                 # Makefile 타깃을 쓰려면 필요하다
   free -h                        # Swap 에 ${SWAP_GB}.0Gi 가 보여야 한다

 아직이면 1~2분 기다린다. 5분 넘게 안 되면 진행 상황을 본다:

   sudo cloud-init status --long
   sudo tail -40 /var/log/cloud-init-output.log

 ── 2. 코드와 설정 ──────────────────────────────────────────────────

   git clone https://github.com/judy98-s/claw-hub.git && cd claw-hub
   git checkout claude/nifty-cori-joc5pd
   cp .env.example .env

 비밀값은 make 가 아니라 openssl 로 직접 넣는다 — make 설치가 아직 안
 끝났을 수 있고, 실패하면 .env 가 조용히 빈 채로 남는다. 중복이 생기지
 않게 기존 줄을 먼저 지운다:

   sed -i -e '/^DATA_ENCRYPTION_KEY=/d' -e '/^HASH_PEPPER=/d' \\
          -e '/^SESSION_SECRET=/d'      -e '/^POSTGRES_PASSWORD=/d' \\
          -e '/^PUBLIC_BASE_URL=/d' .env
   {
     echo "DATA_ENCRYPTION_KEY=\$(openssl rand -base64 32)"
     echo "HASH_PEPPER=\$(openssl rand -base64 32)"
     echo "SESSION_SECRET=\$(openssl rand -base64 32)"
     echo "POSTGRES_PASSWORD=\$(openssl rand -base64 24 | tr -d '/+=')"
     echo "SITE_ADDRESS=$NIP"
     echo "PUBLIC_BASE_URL=https://$NIP"
   } >> .env

 값은 보지 않고 채워졌는지만 확인한다:

   for k in DATA_ENCRYPTION_KEY HASH_PEPPER SESSION_SECRET \\
            POSTGRES_PASSWORD SITE_ADDRESS PUBLIC_BASE_URL; do
     v=\$(grep -m1 "^\$k=" .env | cut -d= -f2-)
     [ -n "\$v" ] && echo "  \$k  OK" || echo "  \$k  비어있음 <<<"
   done

 **DATA_ENCRYPTION_KEY 와 HASH_PEPPER 를 잃으면 계좌·전화번호를 영구히
 복구할 수 없다.** 비밀번호 관리자에 따로 보관한다.

 ── 3. 기동 ─────────────────────────────────────────────────────────

 1GB 서버에서는 **빌드하지 않는다.** Next.js 빌드가 1.5GB 넘게 먹어서
 OOM 으로 죽는다. GitHub Actions 가 만든 이미지를 받아 쓴다:

   make prod-up
   make prod-logs

 (make up 은 로컬 빌드라 이 서버에서 쓰면 안 된다.)

 브라우저에서 https://$NIP/admin/login

 ── 4. 첫 계정 ──────────────────────────────────────────────────────

   docker compose -f docker-compose.prod.yml run --rm --entrypoint /setup api \\
     -store "홍대 인형뽑기" -phone "0212345678" \\
     -email "$(printf '%s' 'owner@example.com')" -password "충분히-긴-비밀번호"

 ── 선택: 공인 IP 를 예약으로 ───────────────────────────────────────

 인스턴스를 껐다 켜도 IP 가 유지된다. nip.io 주소와 QR 스티커가 IP 에
 묶여 있으므로 실제 운영 전에는 해 둔다. Cloud Shell 에서:

   VNIC=\$(oci compute instance list-vnics --instance-id $INST_ID \\
     --query 'data[0].id' --raw-output)
   PRIV_ID=\$(oci network private-ip list --vnic-id \$VNIC \\
     --query 'data[0].id' --raw-output)
   oci network public-ip create --compartment-id $COMP \\
     --lifetime RESERVED --private-ip-id \$PRIV_ID --display-name clawhub-ip

 한도 초과 에러가 나면 임시 IP 로 두고, 인스턴스를 재시작하지 않도록만
 주의한다.

 ── 내 PC 에서도 SSH 하려면 ─────────────────────────────────────────

 이 Cloud Shell 에서 개인키를 출력해 내 PC 의 ~/.ssh/clawhub 로 저장하고
 chmod 600 한다. 개인키다 — 채팅·커밋·이슈에 붙여넣지 않는다:

   cat $SSH_KEY

────────────────────────────────────────────────────────────────
EOF
