# 배포

단일 VPS + Docker Compose 기준이다. 사양은 2 vCPU / 2GB RAM 이면 충분하다.

부하 계산: 기계 500대에 대당 하루 1건 접수여도 **하루 500건 ≈ 초당 0.006 요청**이다.
Postgres 단일 인스턴스 여력의 백만 분의 일이라 트래픽으로 아플 일은 구조적으로 없다.
먼저 차는 것은 CPU 가 아니라 **사진 디스크**다.

---

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
