#!/usr/bin/env bash
#
# Oracle Cloud 에 claw-hub 용 Always Free 인스턴스를 만든다.
#
# **Oracle Cloud Shell 에서 실행한다.** 콘솔 우측 상단의 >_ 아이콘이다.
# Cloud Shell 은 이미 내 계정으로 인증되어 있어서 API 키가 필요 없다.
#
#   bash oci-provision.sh
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
#   - 인스턴스 1개                   VM.Standard.A1.Flex, 2 OCPU / 12GB,
#                                   Ubuntu 24.04 (ARM), 부트볼륨 50GB
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
OCPUS=2
MEMORY_GB=12
BOOT_GB=50
SSH_KEY="$HOME/.ssh/${NAME}"

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
say "리전 $REGION / 컴파트먼트 ${COMP:0:30}..."

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

# ── 2. Ubuntu 24.04 ARM 이미지 ───────────────────────────────────────
say "Ubuntu 24.04 (ARM) 이미지 찾기"
IMAGE_ID=$(oci compute image list \
  --compartment-id "$COMP" \
  --operating-system "Canonical Ubuntu" \
  --operating-system-version "24.04" \
  --shape "VM.Standard.A1.Flex" \
  --sort-by TIMECREATED --sort-order DESC \
  --query 'data[0].id' --raw-output 2>/dev/null || true)
[ -n "$IMAGE_ID" ] && [ "$IMAGE_ID" != "null" ] || die "A1.Flex 용 Ubuntu 24.04 이미지를 찾지 못했다."
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

  say "인스턴스 만들기 — 가용 도메인 ${#ADS[@]}개를 차례로 시도"
  LOG=$(mktemp)
  for AD in "${ADS[@]}"; do
    printf '    %s ... ' "$AD"
    if oci compute instance launch \
        --compartment-id "$COMP" \
        --availability-domain "$AD" \
        --subnet-id "$SUBNET_ID" \
        --image-id "$IMAGE_ID" \
        --shape "VM.Standard.A1.Flex" \
        --shape-config "{\"ocpus\":$OCPUS,\"memoryInGBs\":$MEMORY_GB}" \
        --boot-volume-size-in-gbs "$BOOT_GB" \
        --display-name "$NAME" \
        --assign-public-ip true \
        --ssh-authorized-keys-file "$AUTH_KEYS" \
        --wait-for-state RUNNING \
        > "$LOG" 2>&1; then
      INST_ID=$(jq -r '.data.id' "$LOG")
      printf '\033[0;32m성공\033[0m\n'
      break
    fi
    if grep -qi 'out of host capacity\|OutOfCapacity' "$LOG"; then
      printf '\033[0;33m재고 없음\033[0m\n'
    else
      printf '\033[0;31m오류\033[0m\n'
      tail -20 "$LOG" >&2
      die "인스턴스 생성이 실패했다. 위 오류를 보내라."
    fi
  done

  if [ -z "${INST_ID:-}" ] || [ "$INST_ID" = "null" ]; then
    warn "모든 가용 도메인에 ARM 재고가 없다. 계정이나 설정 문제가 아니다."
    warn "몇 시간 뒤 다시 돌려라 (한국 새벽에 잘 풀린다). 이 스크립트는"
    warn "이미 만든 VCN·서브넷을 건너뛰고 인스턴스만 다시 시도한다."
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
 접속 주소     https://$NIP
 인스턴스      ${INST_ID:0:40}...

 다음 — 이 Cloud Shell 에서 바로 접속한다:

   ssh -i $SSH_KEY ubuntu@$PUBLIC_IP

 서버에 들어가서 (docs/DEPLOY.md §0-7 ② 부터):

   sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
   sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
   sudo netfilter-persistent save

   curl -fsSL https://get.docker.com | sudo sh
   sudo usermod -aG docker \$USER && exec su -l \$USER

   git clone https://github.com/judy98-s/claw-hub.git && cd claw-hub
   git checkout claude/nifty-cori-joc5pd
   cp .env.example .env
   make keys >> .env
   # .env 에서 값이 비어 있는 DATA_ENCRYPTION_KEY / HASH_PEPPER /
   # SESSION_SECRET / PUBLIC_BASE_URL 줄을 지운다
   echo 'SITE_ADDRESS=$NIP'              >> .env
   echo 'PUBLIC_BASE_URL=https://$NIP'   >> .env
   make up

 선택 — 공인 IP 를 예약으로 바꾼다 (인스턴스를 껐다 켜도 IP 가 유지된다.
 nip.io 주소와 QR 스티커가 IP 에 묶여 있으므로 실제 운영 전에는 해 둔다):

   PRIV=\$(oci compute instance list-vnics --instance-id $INST_ID \\
     --query 'data[0]."private-ip"' --raw-output)
   VNIC=\$(oci compute instance list-vnics --instance-id $INST_ID \\
     --query 'data[0].id' --raw-output)
   PRIV_ID=\$(oci network private-ip list --vnic-id \$VNIC \\
     --query 'data[0].id' --raw-output)
   oci network public-ip create --compartment-id $COMP \\
     --lifetime RESERVED --private-ip-id \$PRIV_ID --display-name clawhub-ip

 예약 IP 한도 초과 에러가 나면 임시 IP 로 두고, 인스턴스를 재시작하지
 않도록만 주의한다.

 내 PC 에서도 SSH 하려면 — 이 Cloud Shell 에서 개인키를 출력해
 내 PC 의 ~/.ssh/clawhub 로 저장하고 chmod 600 한다:

   cat $SSH_KEY

 (개인키다. 채팅·커밋·이슈에 붙여넣지 않는다.)

────────────────────────────────────────────────────────────────
EOF
