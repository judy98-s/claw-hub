# claw-hub 설계 스펙

**작성일:** 2026-09-21
**상태:** 검토 대기
**방법론:** superpowers (brainstorming → spec → plan → TDD 구현)

---

## 1. 무엇을 만드는가

인형뽑기(크레인 게임) 기계의 **고장·환불 문의를 접수하고 처리하는 시스템**.

기계마다 QR 스티커가 붙어 있고, 손님이 QR을 찍으면 모바일 웹 신고 폼이 열린다.
손님은 증상·사진·금액·전화번호·계좌를 입력하고 제출한다. 사장님은 Slack 알림을 받고
웹 대시보드에서 승인/거절하고, 승인 시 송금 딥링크 버튼으로 환불한다.

**성공 기준**

- 손님이 QR을 찍고 제출까지 **60초 이내**, 앱 설치·로그인·카톡 친구추가 없이.
- 사장님이 알림을 받고 환불 완료까지 **30초 이내**, 계좌번호를 손으로 옮겨적지 않고.
- 같은 사람이 반복해서 거짓 환불을 요구하면 **자동으로 보류**되어 사장님 화면에 근거와 함께 뜬다.
- "3번 기계 오늘 3건"처럼 **기계별 이상 징후가 사장님에게 먼저 도달**한다.

**명시적 비목표 (YAGNI)**

- 결제 수납, 매출 정산, 회계 연동 — 이 시스템은 돈을 **돌려주는** 쪽만 다룬다.
- 다점포 프랜차이즈 권한 체계, 직원 역할 세분화 — 1단계는 매장 1개 + 사장님 계정.
- 네이티브 모바일 앱 — 대시보드는 PWA로 홈화면 추가만 지원한다.
- 카카오 알림톡/챗봇 — 2단계. 단, 알림 채널은 인터페이스로 추상화해 추가 비용을 최소화한다.

---

## 2. 핵심 제약: "원클릭 자동 송금"은 1단계에서 불가능하다

이 스펙에서 가장 중요한 현실 제약이다. 설계 전체가 여기에 맞춰져 있다.

- **카카오페이 API**는 온라인 *결제* API다. 사업자가 임의 개인 계좌로 *송금*하는 공개 API는 없다.
- **토스페이먼츠**는 PG(결제대행)다. 송금이 아니다.
- 서버가 실제로 돈을 보내려면 **지급대행 / 펌뱅킹 / 오픈뱅킹 출금이체** 중 하나여야 하고,
  셋 다 사업자 계약 + 심사가 필요하다(수 주 소요, 통상 법인 요구).

**1단계 결정: 딥링크 반자동 + 수동 기록 병행.**

사장님이 대시보드에서 "환불 보내기"를 누르면 토스/카카오뱅크 앱이 **은행·계좌·금액이 채워진
송금 화면**으로 열린다. 사장님은 인증 한 번만 한다. 앱에서 돌아오면 대시보드가
"보냈어요 / 안 보냈어요"를 묻고, 보냈으면 `paid`로 기록한다.

딥링크가 안 열리는 환경(PC 브라우저 등)을 위해 **계좌번호 원터치 복사 + 수동 '송금 완료' 체크**를
항상 함께 제공한다. 이게 "수동 기록" 경로다.

**확장 지점:** `payout.Payout` 인터페이스 하나를 두고 1단계는 `DeeplinkPayout`으로 구현한다.
나중에 지급대행 계약이 되면 `PGPayout`을 **파일 하나 추가**해서 갈아끼운다. 도메인 코드는 건드리지 않는다.

> 딥링크 URL 스킴은 벤더가 예고 없이 바꾸는 값이다. 코드에 하드코딩하지 않고
> 설정(`payout.deeplink.templates`)으로 빼서, 스킴이 바뀌어도 재배포 없이 고칠 수 있게 한다.
> 어떤 스킴도 열리지 않을 때의 폴백이 곧 수동 기록 경로다.

---

## 3. 시스템 구성

```
  손님 폰                         Next.js (web)                 Go API                    Postgres
 ┌────────┐   QR 스캔   ┌──────────────────────┐   REST   ┌─────────────┐          ┌──────────┐
 │ 카메라  │ ─────────→ │ /r/{code} 신고 폼      │ ───────→ │ 접수 핸들러   │ ───────→ │ claims   │
 └────────┘             │ 사진 업로드            │          │ 리스크 평가   │          │ machines │
                        └──────────────────────┘          │ (순수 함수)   │          │ photos   │
                                                          └──────┬──────┘          └──────────┘
                                                                 │
                          ┌──────────────────────┐               ├──→ Redis (레이트리밋/중복방지)
 사장님 ─────────────────→ │ /admin 대시보드        │ ←─────────────┤
                          │ 승인·거절·송금 딥링크   │               └──→ Slack Webhook (알림)
                          └──────────────────────┘
                                                          ┌─────────────┐
                                                          │ worker      │ 일일 집계 / 사진 만료 삭제
                                                          └─────────────┘
```

**인프라 (1단계):** 단일 VPS + Docker Compose로 `api`, `worker`, `postgres`, `redis`, `web` 5개 컨테이너.

**부하에 대한 판단.** 기계 500대 × 하루 1건 = **하루 500건 ≈ 초당 0.006 요청**이다.
Postgres 단일 인스턴스 여력의 백만 분의 일 수준이라 트래픽 병목은 구조적으로 생기지 않는다.
실제로 먼저 아플 곳은 DB가 아니라 **사진 파일 I/O**이고, 그건 오브젝트 스토리지로 푸는 문제다.
그래서 `media.Storage` 인터페이스를 두고 1단계는 로컬 디스크, 나중에 S3/R2로 교체한다.

**Redis vs Valkey vs ElastiCache는 지금 결정하지 않는다.** Valkey는 Redis 프로토콜 호환 포크라
Go 코드는 `REDIS_URL` 한 줄만 본다. 전환은 환경변수 변경이다. 게다가 Redis는 1단계에서
레이트리밋·멱등성·알림 디듀프에만 쓰이므로, **Redis가 없어도 단일 인스턴스 인메모리로 동작**하도록
`cache.Cache` 인터페이스에 `memory` 구현을 함께 둔다. 개발 환경에서 Redis를 띄우지 않아도 되고,
Redis 장애가 접수 실패로 번지지 않는다.

---

## 4. 도메인 모델

### 4.1 상태 기계 (claim.status)

```
                    ┌──────────────┐
   제출 ──리스크평가─→│   pending    │ 소액·정상 → 사장님 원터치 승인 대기
                    └──────┬───────┘
                           │
   제출 ──리스크평가─→┌──────▼───────┐
                    │ needs_review │ 1만원 이상 또는 리스크 플래그 → 사장님 검토 필요
                    └──────┬───────┘
                           │
   제출 ──리스크평가─→┌──────▼───────┐
                    │   on_hold    │ 자동 보류 (반복 신고/차단 대상)
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ approved │ │ rejected │ │  (보류)   │
        └────┬─────┘ └──────────┘ └──────────┘
             │ 송금 실행 (딥링크 또는 수동)
        ┌────▼─────┐
        │   paid   │  종료 상태
        └──────────┘
```

전이 규칙은 `domain.Claim.Transition(to, actor)`가 강제한다. 허용되지 않은 전이는 에러다.
모든 전이는 `claim_events`에 append-only로 기록된다 — 돈이 오가므로 감사 로그는 선택이 아니다.

### 4.2 증상 유형 (issue_type)

| 값 | 표시 |
|---|---|
| `doll_stuck` | 인형 걸림 |
| `cash_eaten` | 현금 먹음 |
| `claw_broken` | 집게 불량 |
| `other` | 기타 |

### 4.3 금액 구간 정책

| 구간 | 사진 | 초기 상태 | 사장님 액션 |
|---|---|---|---|
| ~ 9,999원 | 선택 | `pending` | 원터치 승인 |
| 10,000원 이상 | **필수 (1장 이상)** | `needs_review` | 사진 확인 후 승인, **통화 버튼 제공** |

1만원 이상에서 SMS OTP 인증은 넣지 않는다. 건당 비용이 붙고, 실제 방어력은 "사장님이 직접
전화를 건다"가 더 높다. 대시보드에 `tel:` 링크를 두어 한 번 탭으로 통화한다.

임계 금액은 설정값(`policy.review_threshold_krw`, 기본 10000)으로 뺀다.

### 4.4 리스크 / 블랙리스트 규칙

`domain/risk.go`에 **순수 함수**로 구현한다. DB도 HTTP도 모르는 함수라 테스트가 전수로 가능하다.

```go
func Evaluate(in RiskInput, p Policy) RiskResult  // → Score, Reasons, SuggestedStatus
```

| 규칙 | 조건 (기본값) | 결과 |
|---|---|---|
| `manual_block` | 연락처가 수동 차단 목록 | `on_hold` |
| `repeat_watch` | 같은 전화번호 30일 내 3건 이상 | `needs_review` |
| `repeat_hold` | 같은 전화번호 30일 내 5건 이상 | `on_hold` |
| `account_sharing` | 같은 계좌에 서로 다른 전화번호 3개 이상 | `needs_review` |
| `payout_ceiling` | 같은 전화번호 30일 누적 지급 50,000원 초과 | `needs_review` |
| `high_amount` | 금액 ≥ 10,000원 | `needs_review` |
| `rapid_duplicate` | 같은 전화번호 + 같은 기계 10분 내 재제출 | 접수 거부 (중복) |

**결과 합성:** 가장 강한 상태가 이긴다 (`on_hold` > `needs_review` > `pending`).
모든 발동 규칙은 `risk_reasons` jsonb에 남아 사장님 화면에 **근거 문장**으로 그대로 노출된다
("이 번호로 30일간 4번째 신고입니다"). 점수만 보여주면 사장님이 판단할 수 없다.

**설계 판단 — 차단이 아니라 보류로.** `blocked` 연락처여도 접수 자체는 받고 `on_hold`로 둔다.
손님 화면에는 정상 접수와 똑같이 보인다. 이유는 두 가지다. (1) 상습 신고자도 진짜로 기계가
고장났을 수 있다. (2) 차단 사실을 알려주면 번호를 바꿔가며 우회하는 법을 학습시킨다.

### 4.5 기계 리포트

- **실시간:** 한 기계에 24시간 내 `machine_alert_threshold`(기본 3)건 이상 접수 → Slack 알림
  `"3번 기계 · 오늘 3건 접수 (현금 먹음 2, 집게 불량 1) — 점검이 필요해 보입니다"`.
  같은 기계에 대해 24시간 내 1회만 알린다 (Redis 디듀프).
- **일일 요약:** 매일 09:00 KST, 전일 접수/지급 총계 + 접수 상위 기계 3곳을 Slack으로.

---

## 5. 데이터 모델 (Postgres)

```
stores          id, name, created_at
users           id, store_id, email(uniq), password_hash, role, created_at
machines        id, store_id, code(uniq, 공개 QR 코드), label, location, active, created_at
claims          id(uuid), store_id, machine_id, issue_type, amount_krw, description,
                phone_enc, phone_hash, bank_code, account_enc, account_hash, holder_enc,
                status, risk_score, risk_reasons(jsonb),
                created_at, resolved_at, resolved_by, paid_at, payout_method,
                ip_hash, user_agent
claim_photos    id, claim_id, object_key, content_type, size_bytes, created_at, expires_at
claim_events    id, claim_id, actor, action, from_status, to_status, note, created_at
contact_flags   identity_hash, identity_type('phone'|'account'), store_id,
                manual_status('normal'|'watch'|'blocked'), note, updated_at, updated_by
machine_daily   machine_id, day, claim_count, paid_count, total_paid_krw
```

**인덱스:** `claims(store_id, status, created_at desc)` — 대시보드 기본 조회.
`claims(phone_hash, created_at desc)` — 리스크 평가.
`claims(account_hash, created_at desc)` — 계좌 공유 탐지.
`claims(machine_id, created_at desc)` — 기계 리포트.

**집계 전략.** 30일 카운트는 **매번 `claims`에서 실시간 집계**한다. 별도 카운터 테이블을 두면
정합성이 깨질 여지가 생기고, 이 데이터 규모(수만 행)에서 인덱스 스캔은 밀리초 단위다.
`machine_daily`만 예외로 머티리얼라이즈하는데, 이건 일일 요약 리포트 전용 읽기 최적화다.

---

## 6. 개인정보 처리

계좌번호·전화번호·예금주명은 개인정보다. 나중에 붙이는 게 아니라 **처음부터** 넣는다.

- **암호화 저장:** `AES-256-GCM`. 키는 환경변수(`DATA_ENCRYPTION_KEY`), 나중에 KMS로 이전.
  `internal/crypto`가 이 경계를 소유하고, 도메인 코드는 평문만 본다.
- **해시:** `HMAC-SHA256(pepper, value)`. 순수 SHA256을 쓰면 안 된다 —
  한국 휴대폰 번호는 경우의 수가 1억 미만이라 무차별 대입으로 전부 복원된다. pepper가 이걸 막는다.
- **사진 보존:** 업로드 시 `expires_at = now + 90일`. worker가 매일 만료분을 삭제한다.
- **로그:** 계좌·전화번호는 어떤 로그에도 평문으로 남기지 않는다. 로깅 시 마스킹 헬퍼를 통과시킨다.
- **손님 폼:** 로그인이 없다. 대신 IP·기계 단위 레이트리밋으로 남용을 막는다.
- **사장님 대시보드:** 세션 쿠키(HttpOnly, Secure, SameSite=Lax) 인증.

---

## 7. API

**공개 (인증 없음, 레이트리밋 적용)**

| 메서드 | 경로 | 설명 |
|---|---|---|
| `GET` | `/api/public/machines/{code}` | QR 코드 → 기계 정보(라벨/매장명). 없으면 404 |
| `POST` | `/api/public/claims` | 신고 접수. 멱등성 키 헤더 필수 |
| `POST` | `/api/public/claims/{id}/photos` | 사진 업로드 (multipart, 최대 5MB × 3장) |

**관리자 (세션 인증)**

| 메서드 | 경로 | 설명 |
|---|---|---|
| `POST` | `/api/admin/login` / `logout` | 인증 |
| `GET` | `/api/admin/claims` | 목록. `status`, `machine_id`, 기간, 커서 페이징 |
| `GET` | `/api/admin/claims/{id}` | 상세. 계좌·전화번호 **복호화해서 반환** (감사 로그 남김) |
| `POST` | `/api/admin/claims/{id}/approve` / `reject` / `mark-paid` | 상태 전이 |
| `GET` | `/api/admin/claims/{id}/payout-link` | 딥링크 URL 목록 (토스/카뱅) 생성 |
| `GET` | `/api/admin/machines` · `POST` · `PATCH` | 기계 CRUD |
| `GET` | `/api/admin/machines/{id}/qr.png` | QR 이미지 (인쇄용) |
| `GET` | `/api/admin/stats/daily` | 기계별/일자별 집계 |
| `GET` | `/api/admin/contacts` · `PATCH` `/api/admin/contacts/{hash}` | 연락처 카운트 조회 / 수동 상태 변경 |

**멱등성.** 접수 API는 `Idempotency-Key` 헤더를 요구한다. 모바일 네트워크에서 제출 버튼이
두 번 눌리거나 재시도가 발생해도 중복 환불이 생기면 안 된다. 키는 Redis에 24시간 보관하고,
같은 키 재요청은 최초 결과를 그대로 돌려준다.

---

## 8. 프론트엔드

**Design Read** (taste-skill §0.B):
> 손님 신고 폼 for *길에서 이미 짜증난 일반 이용객*, 사장님 대시보드 for *비기술 자영업자*,
> with a **trust-first · 모바일 우선 · 저마찰** language,
> leaning toward Tailwind v4 + Pretendard + Phosphor icons + 절제된 모션.

**다이얼** (taste-skill §1):

| 화면 | DESIGN_VARIANCE | MOTION_INTENSITY | VISUAL_DENSITY |
|---|---|---|---|
| 손님 신고 폼 | 4 | 3 | 4 |
| 사장님 대시보드 | 3 | 2 | 6 |

둘 다 기본값(8/6/4)보다 낮다. 근거: taste-skill의 *trust-first / 규제 산업* 프리셋에 해당한다.
**돈과 개인정보가 오가고, 사용자는 이미 화가 나 있다.** 화려한 모션은 여기서 신뢰를 깎는다.
대시보드는 taste-skill이 명시적으로 범위 밖(`not dashboards`)이라 같은 레포의
`minimalist-skill` 규칙을 적용한다.

**안티-디폴트 (taste-skill §0.D) — 금지 목록:**
AI 보라색 그라데이션, 다크 메시 위 중앙정렬 히어로, 3개 균등 피처 카드,
전면 글래스모피즘, 무한 루프 마이크로 애니메이션, `Inter + slate-900`.

**대신:** 웜 뉴트럴 그레이 베이스 + 단일 액센트 1색 + 상태색(승인/보류/거절) 3색.
한국어 본문은 **Pretendard** 자체 호스팅 (`next/font/local`). Inter는 한글 글리프가 없어
시스템 폰트로 폴백되고, 그 순간 디자인이 무너진다.

**아이콘:** `@phosphor-icons/react`, `strokeWidth` 1.5로 전역 고정. 이모지 금지. SVG 직접 그리기 금지.

**라우트**

```
/r/[code]              손님 신고 폼 (공개)
/r/[code]/done         제출 완료 — 접수번호, 예상 처리 안내
/admin/login
/admin                 접수함 — 상태 탭 + 리스크 배지
/admin/claims/[id]     상세 — 사진, 리스크 근거, 승인/거절, 송금 딥링크, 통화 버튼
/admin/machines        기계 목록 + QR 다운로드 + 이상 징후
/admin/contacts        연락처별 신고 횟수 / 수동 차단
```

**손님 폼 UX 원칙**

- 한 화면, 스크롤 1회 이내. 단계 마법사 금지 — 이탈만 늘린다.
- 증상 선택은 큰 터치 타깃 4개. 드롭다운 금지.
- 사진은 `capture="environment"`로 카메라 바로 열기.
- 계좌 입력 시 은행은 아이콘 그리드, 계좌번호는 `inputmode="numeric"`.
- 1만원 이상 입력하는 순간 **사진 필수 + "확인 후 연락드릴 수 있습니다"** 안내가 인라인으로 뜬다.
  제출 후에 거부당하는 것보다 입력 중에 아는 게 낫다.
- `min-h-[100dvh]` 사용 (`h-screen` 금지 — iOS Safari 주소창에서 레이아웃이 튄다).

---

## 9. 코드 구조

```
claw-hub/
├── cmd/
│   ├── api/main.go             HTTP 서버 진입점
│   └── worker/main.go          일일 집계 · 사진 만료 삭제
├── internal/
│   ├── config/                 환경변수 로딩 + 검증
│   ├── domain/                 엔티티와 규칙. DB/HTTP 의존성 0
│   │   ├── claim.go            Claim, 상태 전이
│   │   ├── machine.go
│   │   ├── risk.go             리스크 평가 (순수 함수)
│   │   └── policy.go           임계값 설정 구조체
│   ├── store/                  Postgres 리포지토리 (pgx)
│   ├── cache/                  Cache 인터페이스 + redis/memory 구현
│   ├── crypto/                 AES-GCM 암호화 + HMAC 해시 + 마스킹
│   ├── media/                  Storage 인터페이스 + local/s3 구현
│   ├── notify/                 Notifier 인터페이스 + slack/noop 구현
│   ├── payout/                 Payout 인터페이스 + deeplink 구현
│   ├── httpapi/                핸들러 · 미들웨어 · 라우팅
│   └── qrcode/                 QR 생성
├── migrations/                 번호순 SQL, embed로 바이너리에 포함
├── web/                        Next.js 15 + Tailwind v4
└── docker-compose.yml
```

**핵심 원칙 (superpowers 설계 지침).** `internal/domain`은 다른 어떤 내부 패키지도 import 하지
않는다. 리스크 규칙과 상태 전이는 DB도 HTTP도 모르는 순수 함수라, **Postgres 없이 전수 테스트**가
가능하다. 이 시스템에서 틀리면 돈이 나가는 로직이 전부 거기에 모여 있다.

라우터는 Go 1.22+ 표준 `net/http.ServeMux`를 쓴다. 메서드·경로 패턴을 지원하므로
chi/gin이 필요 없다 (gstack §2 "Search Before Building" — 이미 표준에 있으면 의존성을 늘리지 않는다).

**테스트 전략**

| 계층 | 방식 |
|---|---|
| `domain` | 순수 유닛 테스트. 테이블 드리븐. 리스크 규칙은 경계값 전수 |
| `crypto` | 왕복 테스트 + 알려진 벡터 |
| `store` | 통합 테스트. docker-compose Postgres. `//go:build integration` 태그로 분리 |
| `httpapi` | `httptest` + 인메모리 fake 리포지토리 |
| `payout`, `notify` | 인터페이스 fake로 호출 검증 |
| `web` | Playwright로 손님 폼 제출 ~ 대시보드 승인 해피패스 1개 |

TDD 순서를 지킨다: 실패하는 테스트 → 실패 확인 → 최소 구현 → 통과 확인 → 커밋.

---

## 10. 설정 (환경변수)

```
DATABASE_URL                 postgres://...
REDIS_URL                    redis://...           (빈 값이면 인메모리 캐시 사용)
DATA_ENCRYPTION_KEY          32바이트 base64       (필수)
HASH_PEPPER                  32바이트 base64       (필수)
SESSION_SECRET               32바이트 base64       (필수)
SLACK_WEBHOOK_URL            (빈 값이면 알림 비활성)
MEDIA_DIR                    ./data/photos
PUBLIC_BASE_URL              QR에 인코딩될 주소
POLICY_REVIEW_THRESHOLD_KRW  10000
POLICY_REPEAT_WATCH_COUNT    3
POLICY_REPEAT_HOLD_COUNT     5
POLICY_MACHINE_ALERT_COUNT   3
PHOTO_RETENTION_DAYS         90
```

필수 항목이 비어 있으면 **서버가 기동을 거부**한다. 암호화 키 없이 개인정보를 받는 상태로
떠 있는 것보다 안 뜨는 게 낫다.

---

## 11. 단계 구분

**1단계 (이번 구현 범위)**
접수 폼 · 사진 업로드 · 리스크 평가 · Slack 알림 · 대시보드 승인/거절 ·
딥링크 + 수동 송금 기록 · 연락처 카운트 · 기계별 집계와 알림 · QR 생성.

**2단계 (이후)**
카카오 알림톡 채널 · 지급대행 API 연동 · 다점포 권한 · S3 전환 · 손님용 처리 현황 조회.

---

## 12. 열린 위험

| 위험 | 대응 |
|---|---|
| 토스/카뱅 딥링크 스킴이 바뀌거나 미지원 | 설정으로 외부화 + 수동 기록 폴백 상시 제공 |
| 손님이 계좌번호를 틀리게 입력 | 예금주명 병기 요구, 송금 실패 시 `approved`로 되돌리는 경로 제공 |
| 사진 저장소가 디스크를 채움 | 90일 자동 삭제 + 건당 5MB×3장 상한 + 디스크 사용량 알림 |
| 레이트리밋 우회 (IP 변경) | 1단계는 전화번호 기준 카운트로 대응. 남용이 실제로 발생하면 캡차 추가 |
| Slack 장애로 알림 유실 | 알림은 부가 경로다. 대시보드가 항상 단일 진실 공급원 |
