# 배포

단일 VPS + Docker Compose 기준이다. 사양은 2 vCPU / 2GB RAM 이면 충분하다.

## 어디에 올릴까

| | 2026년 9월 기준 | 이 앱에 |
|---|---|---|
| **Oracle Cloud Always Free** | 2 OCPU / 12GB ARM. **2026년 6월 15일**에 4 OCPU/24GB 에서 반토막났다. 기존 사용자는 사양을 줄일 때까지 인스턴스가 정지됐다 | **권장.** 반토막나고도 차고 넘친다 |
| Render 무료 | **15분 유휴 후 잠들고 첫 요청에 ~1분** | **손님 폼에는 못 쓴다.** 기계 앞에서 1분 기다릴 손님은 없다 |
| Fly.io 무료 | **2024년 10월에 없어졌다.** 지금은 짧은 체험만 | 해당 없음 |
| Vultr / Hetzner | 월 5~7천원 | Oracle 인스턴스가 안 만들어질 때의 대안 |

**Oracle 의 함정:** 무료 ARM 인스턴스는 지역별 용량이 자주 바닥나서
`Out of host capacity` 로 생성이 실패한다. 며칠 걸리는 일이 흔하다. 하루 이틀
시도해서 안 되면 유료 VPS 로 가는 편이 시간을 아낀다 — 월 6천원이다.



부하 계산: 기계 500대에 대당 하루 1건 접수여도 **하루 500건 ≈ 초당 0.006 요청**이다.
Postgres 단일 인스턴스 여력의 백만 분의 일이라 트래픽으로 아플 일은 구조적으로 없다.
먼저 차는 것은 CPU 가 아니라 **사진 디스크**다.

---

## 0. Oracle Cloud 인스턴스 만들기

되돌릴 수 없는 선택이 **하나** 있고, 사람을 제일 많이 막는 벽이 **하나** 있다.
그 둘만 알고 들어가면 나머지는 클릭이다.

| | 왜 중요한가 |
|---|---|
| **홈 리전** | 가입할 때 한 번 고르면 **못 바꾼다.** 바꾸려면 계정을 새로 만들어야 한다. 그리고 Always Free 자원은 **홈 리전에서만** 만들 수 있다 |
| **Out of host capacity** | 무료 ARM 은 수요가 많아 생성이 자주 실패한다. 계정 문제가 아니라 그 순간 재고가 없는 것이다 |

### 0-1. 가입

<https://www.oracle.com/cloud/free/> → **Start for free**

1. 이메일 → 인증 메일의 링크
2. 국가 **South Korea**, 이름·주소·휴대폰 인증
3. **카드 등록** — 본인 확인용이다. Always Free 자원에는 청구되지 않는다.
   가입하면 30일짜리 $300 체험 크레딧도 같이 주는데, 30일이 지나면 체험
   자원은 멈추고 **Always Free 항목만 계속 남는다.** 카드로 자동 전환되지
   않는다 (직접 Pay-As-You-Go 로 올리지 않는 한)
4. **홈 리전 선택 — 여기가 되돌릴 수 없는 지점이다**

**한국 리전(춘천 `ap-chuncheon-1` 또는 서울 `ap-seoul-1`)을 고른다.**
손님이 기계 앞에서 여는 화면이라 가까울수록 좋다.

다만 한국 리전에 ARM 재고가 없어서 며칠을 시도해도 안 될 수 있다. 그때
선택지는 둘이다.

- **계정을 새로 만들어 일본(도쿄 `ap-tokyo-1` / 오사카 `ap-osaka-1`)으로 간다.**
  한국에서 30~40ms 라 충분히 빠르다. 이 앱은 하루 수백 건 규모라 레이턴시가
  문제 될 구조가 아니다
- **유료 VPS 로 간다.** 월 6천원이면 같은 사양이 즉시 나온다

시간을 얼마나 쓸지 먼저 정해두면 좋다 — **이틀 시도해서 안 되면 옮긴다** 정도.

### 0-2. SSH 키 만들기

인스턴스를 만들 때 **공개키를 붙여야** 접속할 수 있다. 미리 만들어둔다.

```bash
# 맥 / 리눅스 / 윈도우 (PowerShell, Git Bash)
ssh-keygen -t ed25519 -C "clawhub" -f ~/.ssh/clawhub

cat ~/.ssh/clawhub.pub    # 이 한 줄을 콘솔에 붙여넣는다
```

`~/.ssh/clawhub` (확장자 없는 쪽)이 **개인키**다. 이걸 잃으면 서버에 못
들어간다. 남에게 주지 않는다.

### 0-3. 인스턴스 생성

콘솔 → 햄버거 메뉴 → **Compute → Instances → Create instance**

| 항목 | 값 |
|---|---|
| **Name** | `clawhub` |
| **Image** | **Ubuntu 24.04** (Canonical Ubuntu) |
| **Shape** | `VM.Standard.A1.Flex` → **OCPU 2 / Memory 12GB** |
| **Networking** | 기본 VCN 자동 생성. **Assign a public IPv4 address: 예** |
| **SSH keys** | **Paste public keys** → `clawhub.pub` 내용 붙여넣기 |
| **Boot volume** | 기본 50GB 그대로 (무료 한도는 총 200GB) |

> **Shape 을 꼭 확인한다.** 기본으로 잡히는 `VM.Standard.E2.1.Micro` 는
> AMD 무료 사양인데 1 OCPU / 1GB 라 이 앱에는 빠듯하다. **Ampere** 탭에서
> `VM.Standard.A1.Flex` 를 고르고 2/12 로 맞춘다.
>
> **2026년 6월 15일부터 무료 ARM 한도가 4 OCPU/24GB 에서 2 OCPU/12GB 로
> 줄었다.** 그보다 크게 잡으면 무료 범위를 넘어 과금된다.

**Create** 를 누른다.

### 0-4. `Out of host capacity` 가 뜨면

계정이나 설정 문제가 아니다. 그 순간 그 가용 도메인에 ARM 재고가 없는 것이다.

1. **가용 도메인(AD)을 바꿔서 재시도.** 생성 화면에서 AD-1 / AD-2 / AD-3 을
   차례로 시도한다 (리전에 따라 하나뿐일 수도 있다)
2. **시간을 두고 다시.** 재고는 무작위로 짧게 풀린다. 몇 시간 뒤가 나을 때가 많다
3. **새벽에 시도.** 경쟁이 덜하다
4. 이틀을 넘기면 위 §0-1 의 두 선택지로 간다

> 자동 재시도 스크립트를 돌리는 사람도 많다. 다만 오라클 약관상 과한 API
> 호출은 계정 제한 사유가 될 수 있으니, 돌리더라도 **몇 분 간격**으로 둔다.

### 0-5. 공인 IP 를 예약으로 바꾸기 — 빼먹지 말 것

기본값은 **Ephemeral(임시)** 이라 인스턴스를 껐다 켜면 IP 가 바뀐다.
그러면 `nip.io` 주소가 바뀌고 **뽑아둔 QR 스티커가 전부 죽는다.**

인스턴스 상세 → **Resources → Attached VNICs** → VNIC 클릭 →
**IPv4 Addresses** → 공인 IP 우측 ⋮ → **Edit** →
**Reserved Public IP** 선택 → 이름 주고 **Update**

이 화면에 보이는 **Public IP Address** 가 앞으로 쓸 주소다.
`Private IP Address`(10.x / 192.168.x)는 내부용이라 쓰지 않는다.

### 0-6. 방화벽 — 두 군데를 다 열어야 한다

**한 군데만 열고 왜 안 되는지 찾는 일이 아주 흔하다.** 오라클은 VCN 레벨과
인스턴스 OS 레벨에 각각 방화벽이 있다.

**① VCN Security List (콘솔)**

Networking → **Virtual Cloud Networks** → 해당 VCN → **Security Lists** →
Default Security List → **Add Ingress Rules**

| Source CIDR | IP Protocol | Destination Port |
|---|---|---|
| `0.0.0.0/0` | TCP | `80` |
| `0.0.0.0/0` | TCP | `443` |

**② 인스턴스 안쪽 iptables (SSH 접속 후)**

Ubuntu 기본 iptables 가 80/443 을 막고 있다.

```bash
ssh -i ~/.ssh/clawhub ubuntu@<공인IP>

sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo netfilter-persistent save
```

### 0-7. Docker 설치

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER && exec su -l $USER

docker --version    # 확인
```

### 0-8. 여기까지 됐는지 확인

```bash
# 서버 안에서
curl -I http://localhost        # 아직 아무것도 없으니 실패해도 정상

# 내 PC 에서 — 포트가 열렸는지
nc -zv <공인IP> 80
```

`nc` 가 통하면 방화벽 두 군데가 다 열린 것이다. 안 통하면 §0-6 을 다시 본다.

## 1. 준비

서버에 Docker 와 Docker Compose 가 있으면 된다.

```bash
git clone https://github.com/judy98-s/claw-hub.git
cd claw-hub
```

## 2. 비밀값 생성

```bash
make keys
```

출력된 네 줄을 `.env` 에 붙여넣는다.

```bash
cp .env.example .env
$EDITOR .env
```

`.env` 는 api·worker·setup 이 실행될 때 **자동으로 읽는다.** 따로 `export`
하거나 `source` 할 필요가 없다. 진짜 환경변수가 있으면 그쪽이 이긴다 —
도커 compose 의 `environment` 와 CI 비밀값이 파일을 덮어야 하기 때문이다.

`.env` 에 반드시 채워야 하는 것:

| 키 | 설명 |
|---|---|
| `DATA_ENCRYPTION_KEY` | 계좌·전화번호 암호화 키 (32바이트 base64) |
| `HASH_PEPPER` | 조회용 해시 pepper (32바이트 base64) |
| `SESSION_SECRET` | 세션 쿠키 서명 키 (32바이트 base64) |
| `POSTGRES_PASSWORD` | DB 비밀번호 |
| `PUBLIC_BASE_URL` | QR 에 인코딩될 주소. 예) `https://claw.example.com` |

> **`DATA_ENCRYPTION_KEY` 와 `HASH_PEPPER` 를 잃어버리면 기존 데이터를 복구할 수 없다.**
> 계좌와 전화번호가 그 키로 암호화되어 있고, 해시는 그 pepper 로 만들어졌다.
> 키를 바꾸면 과거 건의 계좌를 읽을 수 없고 반복 신고 카운트도 끊긴다.
> 비밀번호 관리자에 따로 보관하라.

선택 항목:

| 키 | 비우면 |
|---|---|
| `SLACK_WEBHOOK_URL` | 알림이 꺼진다. 대시보드는 그대로 동작한다 |
| `PAYOUT_DEEPLINK_TEMPLATES` | 송금 화면에 계좌 복사와 수동 기록만 표시된다 |
| `REDIS_URL` | compose 가 자동으로 채운다. 비우면 인메모리 |

## 3. 도메인 연결

### 먼저: 지금 꼭 사야 하나

**시험 중이면 안 사도 된다.** 도메인 없이도 HTTPS 까지 붙는 길이 둘 있다.

| | 비용 | URL | 항상 켜짐 | 언제 쓰나 |
|---|---|---|---|---|
| **내 PC + `make tunnel`** | 0원 | **재시작마다 바뀜** | ✗ | 잠깐 손님 화면 확인 |
| **오라클 + `nip.io`** | 0원 | IP 고정하면 고정 | ✓ | **시험 배포** |
| 오라클 + 산 도메인 | 연 14,000원 | 고정 | ✓ | 기계에 스티커 붙일 때 |

#### `nip.io` — 등록 없이 쓰는 진짜 도메인

`nip.io` 는 `1-2-3-4.nip.io` 를 `1.2.3.4` 로 풀어주는 공개 DNS 다. 가입도
설정도 없고, **진짜 도메인이라 Let's Encrypt 인증서가 그대로 발급된다.**
Caddy 설정은 도메인을 산 것과 똑같다.

```bash
# 오라클 인스턴스 공인 IP 가 152.67.89.123 이라면
echo 'SITE_ADDRESS=152-67-89-123.nip.io' >> .env
echo 'PUBLIC_BASE_URL=https://152-67-89-123.nip.io' >> .env
make up
```

알아둘 것:

- **오라클에서 공인 IP 를 예약(Reserved)으로 바꿔둔다.** 기본값은 임시라
  인스턴스를 재시작하면 바뀌고, 그러면 주소도 QR 도 같이 죽는다.
- 인증서가 `too many certificates already issued for: nip.io` 로 막히면
  `sslip.io` 로 바꾼다 (`152-67-89-123.sslip.io`). 같은 일을 하는 다른
  서비스다. Let's Encrypt 가 두 도메인의 한도를 25만 장까지 올려둬서
  걸릴 일이 흔하지는 않다.
- 주소가 길고 못생겼다. **손님이 손으로 칠 주소로는 부적합하다.** 시험
  단계에서는 QR 만 쓰고, 진짜로 굴리기로 하면 그때 도메인을 산다.

나중에 도메인을 사면 `.env` 의 두 줄을 바꾸고 `make up` 하면 끝이다.
**스티커만 다시 뽑으면 된다** — 그래서 시험 중에는 스티커를 몇 장만 뽑는다.

---

### 진짜로 굴리기로 했다면

**`.com` 을 Cloudflare Registrar 에서 산다. 연 $10.44 (약 14,000원).**

첫 해 990원짜리 `.shop` 이 싸 보이지만 그건 함정이다. 이 주소는 **QR 스티커에
박혀 기계에 붙는다.** 바꾸려면 스티커를 전부 다시 뽑아 다시 붙여야 하므로,
고를 때 봐야 하는 값은 첫 해 가격이 아니라 **몇 년을 내야 하는 갱신가**다.

2026년 9월 기준 갱신가:

| TLD | 첫 해 | **갱신 (매년)** | 5년 총액 |
|---|---|---|---|
| **`.com` @ Cloudflare** | $10.44 | **$10.44** | **약 $52** |
| `.xyz` @ Cloudflare | $10~ | $11.20 | 약 $56 |
| `.shop` `.site` `.online` | $0.90~ | **$20~40** | $80~160 |
| `.co.kr` | — | $22~ | $110~ |

Cloudflare 가 제일 싼 이유는 할인 중이라서가 아니라 **원가로 팔기 때문이다.**
레지스트리 도매가($10.26)에 ICANN 수수료($0.18)만 더하고 마진을 붙이지 않는다.
그래서 2년차에 가격이 뛰지 않는다. 같은 `.com` 이 GoDaddy 에서는 $22.99 다.

`.xyz` 갱신이 $1.58 이라는 얘기를 보게 되면 그건 **첫 해 값**이다. $0.99 짜리는
6~9자리 **숫자** 도메인 전용이라 매장 주소로는 못 쓴다.

> **시한이 있다.** Verisign 이 2026년 4월에 `.com` 도매가 7% 인상을 예고했고
> **11월 1일부터 $10.97** 이 된다. 10월 안에 등록하면 1년치를 현재가로 잠근다.
> 급할 건 없지만, 어차피 살 거면 11월 전이 낫다.

### Cloudflare 를 쓸 때 반드시 볼 것

Cloudflare Registrar 는 **네임서버를 Cloudflare 로 옮겨야** 쓸 수 있다. DNS 관리가
무료이고 편해서 어차피 이득이지만, 여기서 한 가지가 배포를 통째로 막는다.

**A 레코드를 회색 구름(DNS only)으로 두어야 한다.** 주황 구름(프록시)으로 켜면
Cloudflare 가 TLS 를 자기가 끝내버려서, Caddy 가 Let's Encrypt 인증서를 받으려는
HTTP-01 검증이 실패한다. 증상이 "인증서가 안 나온다"로만 보여서 원인을 찾는 데
저녁 하나가 날아간다.

프록시를 꼭 쓰고 싶으면 Cloudflare 오리진 인증서를 따로 발급해 Caddy 에 물려야
하는데, 매장 하나 규모에서는 그럴 이유가 없다. **회색 구름으로 두고 Caddy 가
직접 인증서를 받게 한다.**

### 이름 짓기

스티커에 인쇄되고, QR 이 안 읽히면 손님이 **손으로 친다.** 그래서:

- **짧게.** 손님이 폰 자판으로 친다
- **하이픈·숫자 피하기.** 말로 불러줄 때 "하이픈"을 설명해야 한다
- 매장 이름보다 **기능**이 낫다. 매장이 늘어도 그대로 쓴다

### 설정

`SITE_ADDRESS` 에 도메인을 넣으면 Caddy 가 Let's Encrypt 인증서를 자동으로 받는다.

```bash
echo 'SITE_ADDRESS=claw.example.com' >> .env
```

도메인의 A 레코드가 서버 IP 를 가리켜야 한다. 도메인 없이 IP 로 돌리려면
`SITE_ADDRESS=:80` 으로 두면 되지만, **그러면 HTTPS 가 아니다.**
계좌번호가 평문으로 오가므로 실제 운영에서는 쓰지 말 것.

## 4. 기동

```bash
make up
make logs
```

마이그레이션은 api 컨테이너가 뜰 때 자동으로 적용된다.

## 5. 첫 계정 만들기

회원가입 화면은 없다. 사장님 한 명이 쓰는 시스템에서 공개 가입 페이지는
공격 표면만 늘린다.

```bash
docker compose run --rm --entrypoint /setup api \
  -store "홍대 인형뽑기" \
  -phone "0212345678" \
  -email "owner@example.com" \
  -password "충분히-긴-비밀번호"
```

`-phone` 은 손님이 기계를 못 찾았을 때 안내되는 매장 대표번호다.

## 6. 기계 등록과 QR 인쇄

1. `https://<도메인>/admin/login` 에서 로그인
2. **기계** 탭 → **추가** → 이름과 위치 입력
3. 목록에서 QR 아이콘 → **스티커 인쇄**

스티커에는 QR, 기계 이름, 6자리 코드, 그리고 코드를 직접 입력할 주소가 함께
인쇄된다. QR 만 붙이면 손님은 그게 뭔지 모르고 지나치고, QR 이 긁혀 안 읽힐 때
갈 곳이 없다.

방수 라벨지에 인쇄해 기계 투입구 근처, 손님 눈높이에 붙인다.

---

## Slack 알림 설정

1. Slack 워크스페이스에서 앱 생성 → **Incoming Webhooks** 활성화
2. 알림 받을 채널을 고르고 Webhook URL 복사
3. `.env` 의 `SLACK_WEBHOOK_URL` 에 넣고 `make up` 으로 재기동

알림에는 기계·증상·금액·리스크 사유·처리 링크가 들어가고,
**전화번호와 계좌번호는 들어가지 않는다.** Slack 은 DB 보다 접근 통제가
느슨하고 메시지는 검색·저장·전달된다. 사장님은 링크를 눌러 열어보면 되고,
그 열람은 감사 로그에 남는다.

### 알림 링크는 로그인 없이 열린다

링크에는 **그 신고 건 전용 서명 토큰**이 붙는다. 누르면 로그인 없이 바로
상세가 열리고 승인·거절·송금 기록까지 된다. 매번 비밀번호를 치지 않아도 된다.

범위는 **그 건 하나뿐**이다.

- 다른 신고 건, 연락처 목록, 기계 관리에는 닿지 않는다 (전부 401)
- 토큰은 서명에 claim id 가 섞여 있어 다른 건의 주소에 붙여도 거부된다
- **7일** 뒤 만료된다. 그 뒤에는 로그인해야 한다
- 첫 진입 후 주소창에서 토큰이 지워진다. 사장님이 주소를 복사해 남에게
  보내도 그 사람은 못 연다

> **알림 채널은 반드시 비공개로 만들고 사장님만 두세요.**
> 그 채널에 있는 사람은 누구나 링크를 눌러 손님 계좌를 볼 수 있습니다.
> 이것이 이 방식의 유일한 약점이고, 채널 권한으로만 막을 수 있습니다.

전체 접수함을 보려면 기존대로 `/admin/login` 에서 로그인한다. 링크 접근과
로그인 접근은 감사 로그에서 구분된다(`link` vs `owner`).

## 송금 딥링크 설정

**보통은 환경변수를 건드릴 필요가 없다.** 사장님이 대시보드
**설정 → 송금** 에서 앱을 고르면 되고, 그 값이 환경변수보다 우선한다.
앱을 바꾸려고 서버 설정을 고치고 재기동하는 건 말이 안 되기 때문이다.

고를 수 있는 것:

| 선택 | 동작 |
|---|---|
| 사용 안 함 | 계좌번호 복사 버튼만 표시 |
| 토스 | 은행·계좌·금액이 자동으로 채워짐 |
| 카카오뱅크 | **앱만 열림.** 계좌와 금액은 직접 입력해야 한다 — 자동 입력되는 공개 방식이 확인되지 않았다 |
| 직접 입력 | 딥링크 주소를 직접 넣음 |

같은 화면에서 **출금 계좌**도 적어둘 수 있다. 송금 화면에 "이 계좌에서
나갑니다"로 표시만 된다 — 송금 앱은 출금 계좌를 URL 로 받지 않고 로그인한
사람의 주계좌를 쓰기 때문이다. 직원이 여러 명일 때 개인 계좌에서 나가는
일을 막아준다.

아래 환경변수는 설정을 한 번도 건드리지 않은 매장의 기본값이다.

```
PAYOUT_DEEPLINK_TEMPLATES={"toss":"supertoss://send?bank={bankShort}&accountNo={account}&amount={amount}"}
```

치환 토큰:

| 토큰 | 값 | 예 |
|---|---|---|
| `{bankShort}` | 송금 앱이 받는 짧은 은행명 | `국민`, `카카오` |
| `{bankName}` | 정식 명칭 | `국민은행` |
| `{bank}` | 금융결제원 기관코드 | `004` |
| `{account}` | 계좌번호 (숫자만) | `1234567890` |
| `{amount}` | 금액 | `2000` |
| `{holder}` | 예금주 | `김민수` |

> **`bank` 에는 반드시 `{bankShort}` 를 쓴다.** 토스는 기관코드(`004`)도
> 정식명(`국민은행`)도 인식하지 못한다. `국민` 처럼 짧은 이름이어야 한다.
> 축약형은 `internal/payout/banks.go` 의 `Short` 필드에 있고, 틀린 값이
> 있으면 설정이 아니라 그 파일을 고쳐야 한다.

### 반드시 실기기에서 확인할 것

**이 스킴은 토스가 공식 문서로 제공하는 규격이 아니다.** 커뮤니티가 관찰해
정리한 값이고, 예고 없이 바뀔 수 있다. 그래서 코드가 아니라 설정에 뒀다.

배포 직후 **토스 앱이 설치된 실제 폰**으로 아래를 확인하라.

1. 사장님 대시보드에서 소액 건을 하나 승인한다
2. **환불 보내기** → **토스로 보내기** 를 누른다
3. 토스 앱이 열리고 **은행·계좌·금액이 모두 채워져** 있는지 본다
4. 채워지지 않았다면 어느 값이 비었는지 보고, 위 표대로 템플릿을 고친다

은행이 안 채워지면 `{bankShort}` 값 문제이므로 `banks.go` 를 고치고 재배포한다.
계좌나 금액만 안 채워지면 파라미터 이름이 바뀐 것이므로 템플릿만 고치면 된다.

확인 전까지는 **딥링크 없이 운영해도 된다.** `PAYOUT_DEEPLINK_TEMPLATES={}`
로 두면 사장님 화면에 계좌 복사와 수동 "송금 완료"만 뜨고, 환불은 그대로 나간다.
딥링크는 그 위에 얹는 편의 기능이지 필수 경로가 아니다.

카카오뱅크 송금 딥링크는 확인된 스킴이 없어 기본 설정에 넣지 않았다.

> 서버가 직접 계좌이체를 하지는 않는다. 카카오페이·토스페이먼츠는 결제(수납)
> API 이지 송금 API 가 아니고, 실제 송금에는 지급대행·펌뱅킹·오픈뱅킹 출금이체
> 계약이 필요하다(사업자 심사, 수 주 소요). 계약이 되면 `internal/payout` 에
> 구현체를 하나 추가하면 되고, 도메인 코드는 건드리지 않는다.

---

## 백업

**개인정보가 든 DB다.** 암호화해서 보관한다.

```bash
docker compose exec -T postgres pg_dump -U clawhub clawhub \
  | gzip \
  | openssl enc -aes-256-cbc -pbkdf2 -pass file:/root/.backup-pass \
  > backup-$(date +%F).sql.gz.enc
```

사진은 `media` 볼륨에 있다. 90일이 지나면 워커가 자동으로 지우므로
백업 보존 기간도 그에 맞춘다 — 지운 사진이 백업에 영원히 남으면
보존 정책이 무의미하다.

복구에는 `DATA_ENCRYPTION_KEY` 와 `HASH_PEPPER` 가 **함께** 필요하다.
DB 만 복구하면 계좌를 읽을 수 없다.

## 디스크 관리

사진이 유일하게 자라는 데이터다. 건당 최대 3장 × 5MB 이지만 브라우저에서
긴 변 1600px 로 줄여 올리므로 실제로는 장당 300KB 안팎이다.

하루 50건 × 1장 = 약 15MB/일 ≈ **월 450MB**. 90일 보존이면 정상 상태에서
1.5GB 안팎을 유지한다.

```bash
docker compose exec api df -h /data   # 사용량 확인
```

## 운영 중 확인

```bash
make logs                              # 전체 로그
docker compose logs -f api             # API 만
docker compose ps                      # 컨테이너 상태
curl -s https://<도메인>/healthz        # 헬스체크
```

## 정책값 조정

임계값은 전부 환경변수다. 현장 데이터가 쌓이면 재배포 없이 `.env` 를 고치고
`make up` 으로 재기동하면 된다.

| 키 | 기본 | 뜻 |
|---|---|---|
| `POLICY_REVIEW_THRESHOLD_KRW` | 10000 | 이 금액 이상은 사진 필수 + 수동 검토 |
| `POLICY_REPEAT_WATCH_COUNT` | 3 | 30일 내 이 횟수부터 검토 대상 |
| `POLICY_REPEAT_HOLD_COUNT` | 5 | 30일 내 이 횟수부터 자동 보류 |
| `POLICY_PAYOUT_CEILING_KRW` | 50000 | 30일 누적 지급이 이를 넘으면 검토 |
| `POLICY_MACHINE_ALERT_COUNT` | 3 | 24시간 내 이 건수부터 점검 알림 |
| `PHOTO_RETENTION_DAYS` | 90 | 사진 보존 기간 |

**이 숫자들은 현장 데이터 없이 정한 추정값이다.** 한 달 운영해 보고
"3건은 너무 빡빡하다" 같은 게 드러나면 그때 조정하는 것을 전제로 만들었다.

---

## 내 컴퓨터에서 먼저 돌려보기

VPS 를 사기 전에 전체 흐름을 확인하고 싶다면.

```bash
make dev                      # postgres + redis
cp .env.example .env
make keys                     # 출력된 값을 .env 에 붙여넣기
make api                      # 다른 터미널
make web                      # 또 다른 터미널
make setup-account STORE="테스트 매장" EMAIL=me@example.com PASSWORD=secret123
```

여기까지 하면 `http://localhost:3000` 에서 **내 컴퓨터로는** 다 된다.

**손님 폰과 Slack 링크는 이대로는 안 된다.** 손님 폰이 `localhost` 를 찍으면
자기 폰을 가리키고, Slack 알림 링크도 폰에서 안 열린다. 터널로 공개 주소를
붙인다.

```bash
make tunnel                   # cloudflared 필요, 회원가입 불필요
# → https://xxxx.trycloudflare.com 가 나온다
```

나온 주소를 `.env` 의 `PUBLIC_BASE_URL` 에 넣고 `make api` 를 다시 띄운다.
이제 QR 도 Slack 링크도 폰에서 열린다.

**이건 테스트용이다.** 컴퓨터를 끄면 주소가 죽고, 다시 켜면 주소가 바뀌어서
**이미 붙여놓은 QR 스티커가 전부 무효가 된다.** 실제 손님을 받기 시작하면
VPS 로 옮기고 고정 도메인을 써야 한다.

---

## 안드로이드 앱

사장님 대시보드를 Play 스토어 앱으로 만들려면 [docs/ANDROID.md](ANDROID.md) 를 본다.
지금 웹을 그대로 띄우는 껍데기(TWA)라 코드베이스가 늘지 않는다.

`.env` 에 두 줄이 더 필요하다. 자세한 건 위 문서에 있다.

```
ANDROID_PACKAGE_NAME=com.example.clawhub
ANDROID_SHA256_FINGERPRINTS=내지문,구글지문
```

## 알아둘 함정

**Next.js 의 rewrites 는 빌드 시점에 굳는다.** `next.config.ts` 의 `rewrites()`
는 `routes-manifest.json` 으로 직렬화되고, `next start` 는 런타임
`API_ORIGIN` 을 다시 읽지 않는다. 그래서 프로덕션에서는 rewrites 에 의존하지
않고 **Caddy 가 `/api/*` 를 Go 서버로 보낸다.** `next.config.ts` 의 rewrites 는
로컬 `next dev` 전용이다.

**인스턴스를 여러 대 띄우려면 Redis 가 필요하다.** `REDIS_URL` 이 비면 인메모리
캐시라 레이트리밋이 인스턴스마다 따로 세어진다. compose 구성은 이미 Redis 를
띄우므로 해당 없다.

**Valkey 나 ElastiCache 로 바꾸려면 `REDIS_URL` 만 고치면 된다.** Valkey 는
Redis 프로토콜 호환 포크이고, Go 코드는 접속 주소만 본다.
