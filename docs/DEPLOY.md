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

알림에는 기계·증상·금액·리스크 사유·대시보드 링크가 들어가고,
**전화번호와 계좌번호는 들어가지 않는다.** Slack 은 DB 보다 접근 통제가
느슨하고 메시지는 검색·저장·전달된다. 사장님은 링크를 눌러 대시보드에서
보면 되고, 그 열람은 감사 로그에 남는다.

## 송금 딥링크 설정

```
PAYOUT_DEEPLINK_TEMPLATES={"toss":"supertoss://send?bank={bank}&accountNo={account}&amount={amount}"}
```

치환 토큰: `{bank}` `{bankName}` `{account}` `{amount}` `{holder}`

**딥링크 스킴은 벤더가 예고 없이 바꾼다.** 그래서 코드가 아니라 설정에 둔다.
버튼이 아무것도 하지 않으면 스킴이 바뀐 것이니 여기만 고치고 재기동하면 된다.
어떤 경우든 사장님 화면에는 계좌 복사와 수동 "송금 완료"가 항상 함께 있으므로
딥링크가 죽어도 환불은 나간다.

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
