# claw-hub 1단계 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans 로 태스크 단위 구현.
> 각 단계는 체크박스(`- [ ]`)로 추적한다.

**Goal:** 인형뽑기 기계의 QR 신고 접수부터 사장님의 환불 처리까지를 동작하는 소프트웨어로 만든다.

**Architecture:** Go API + Postgres + Redis(선택) 백엔드와 Next.js 프론트를 REST로 분리한다.
틀리면 돈이 나가는 로직(상태 전이, 리스크 평가)은 전부 `internal/domain`에 순수 함수로 모으고,
외부 연동(송금·알림·저장소·캐시)은 인터페이스 뒤에 둬서 1단계 구현을 나중에 교체할 수 있게 한다.

**Tech Stack:** Go 1.24 (표준 `net/http`), pgx/v5, PostgreSQL 16, Redis 7,
Next.js 15 (App Router), Tailwind v4, Pretendard, Phosphor Icons, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-21-clawhub-design.md`

## Global Constraints

- Go 1.24 이상. 라우팅은 표준 `net/http.ServeMux`의 메서드+경로 패턴을 쓴다. chi·gin·echo 금지.
- `internal/domain`은 다른 `internal/*` 패키지를 import 하지 않는다. 표준 라이브러리만 쓴다.
- 계좌번호·전화번호·예금주명은 **어떤 로그에도 평문으로 남기지 않는다.** `crypto.Mask*`를 통과시킨다.
- 필수 환경변수(`DATA_ENCRYPTION_KEY`, `HASH_PEPPER`, `SESSION_SECRET`, `DATABASE_URL`)가
  비었으면 프로세스는 기동을 거부한다.
- 금액은 전부 정수 원(KRW). 부동소수점 금지.
- 프론트: Tailwind v4, 본문 폰트 Pretendard(`next/font/local` 자체 호스팅), 아이콘은
  `@phosphor-icons/react` 단일 패밀리에 `strokeWidth` 1.5 고정.
- 프론트 금지: 이모지, 손으로 그린 SVG 아이콘, `h-screen`(대신 `min-h-[100dvh]`),
  보라색 그라데이션, 다크 메시 히어로, 3개 균등 피처 카드, 전면 글래스모피즘,
  `Inter`/`slate-900` 기본 조합, 무한 루프 마이크로 애니메이션.
- 다이얼: 손님 폼 VARIANCE 4 / MOTION 3 / DENSITY 4, 대시보드 3 / 2 / 6.
- 모든 상태 전이는 `claim_events`에 append-only로 기록한다.

## Review Focus

스펙이 전제하지만 어느 태스크의 테스트도 자연히 다루지 않는 입력들. 각 항목의 테스트는
괄호 안 태스크가 소유한다.

1. **같은 `Idempotency-Key`로 동시 제출 2건** — 경합으로 claim이 중복 생성되면 환불이 두 번
   나간다. 캐시 선점이 아니라 DB 유니크 제약으로 막아야 한다. (Task 5, Task 10)
2. **Redis 다운 상태의 접수** — 레이트리밋·멱등성이 캐시에 의존하는데, 캐시 장애가 접수 전체를
   막으면 안 된다. 캐시 오류는 로깅 후 통과시키되 DB 유니크 제약이 최후 방어선이다. (Task 6, Task 10)
3. **금액 경계값** — 0원, 음수, 정책 임계치 정확히 10,000원, 비현실적 고액(1,000,000원 초과).
   9,999/10,000은 사진 필수 여부가 갈리는 지점이다. (Task 4, Task 10)
4. **비활성·존재하지 않는 기계 코드** — 스티커가 떼여 다른 곳에 붙거나 폐기된 기계 코드로
   들어오는 접수. 404로 끝내고 claim을 만들지 않는다. (Task 10)
5. **사진 없는 고액 건 / 상한 초과 업로드** — 10,000원 이상인데 사진 0장, 4장 이상, 장당 5MB 초과,
   이미지가 아닌 파일. 전부 접수 거부이고 부분 저장이 남으면 안 된다. (Task 10)

---

### Task 1: 프로젝트 부트스트랩과 설정

**Files:**
- Create: `go.mod`, `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`

**Interfaces:**
- Produces: `config.Config` 구조체와 `config.Load() (Config, error)`.
  `Config.Policy domain.Policy`는 Task 4 이후에 채운다 — 1단계에서는 원시 int 필드로 둔다.

- [ ] **Step 1: 실패하는 테스트** — `DATA_ENCRYPTION_KEY`가 비면 `Load`가 에러를 반환하고,
  에러 메시지에 누락된 키 이름이 포함된다. 키가 base64 32바이트가 아니면 에러.
  `REDIS_URL`이 비면 에러가 아니라 `Config.RedisURL == ""`(인메모리 모드).
  정책 기본값(임계 10000, watch 3, hold 5, 기계알림 3, 보존 90일)이 채워진다.
- [ ] **Step 2: 테스트 실패 확인** — `go test ./internal/config/` → 컴파일 실패
- [ ] **Step 3: 최소 구현** — `os.Getenv` 읽기, 필수 키 검증, base64 디코드 후 길이 확인,
  정수 기본값 파싱. 누락 키는 **한 번에 모아서** 보고한다 (하나씩 고치며 재기동하는 건 시간 낭비).
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: `.env.example` 작성** — 스펙 §10의 키 전부, 키 생성 명령 주석 포함
- [ ] **Step 6: 커밋** `feat(config): 환경변수 로딩과 필수값 검증`

---

### Task 2: 암호화·해시·마스킹

**Files:**
- Create: `internal/crypto/crypto.go`, `internal/crypto/crypto_test.go`

**Interfaces:**
- Produces:
  ```go
  type Cipher struct{ /* aead */ }
  func NewCipher(key []byte) (*Cipher, error)      // key는 정확히 32바이트
  func (c *Cipher) Encrypt(plain string) ([]byte, error)
  func (c *Cipher) Decrypt(ct []byte) (string, error)

  type Hasher struct{ /* pepper */ }
  func NewHasher(pepper []byte) (*Hasher, error)
  func (h *Hasher) Hash(value string) []byte        // HMAC-SHA256, 32바이트

  func MaskPhone(s string) string                   // "010-1234-5678" → "010-****-5678"
  func MaskAccount(s string) string                 // 뒤 4자리만 남김
  func NormalizePhone(s string) (string, error)     // 하이픈/공백 제거, 01012345678 형식 검증
  func NormalizeAccount(s string) string            // 숫자만 남김
  ```

- [ ] **Step 1: 실패하는 테스트** — 암복호화 왕복, 같은 평문을 두 번 암호화하면 ciphertext가
  다르다(nonce 무작위), 변조된 ciphertext는 복호화 실패, 31/33바이트 키는 생성 실패,
  `Hash`는 결정적이고 pepper가 다르면 결과가 다르다, `NormalizePhone`이 `010-1234-5678` ·
  `01012345678` · `+821012345678`을 같은 값으로 정규화하고 `0212345678`(유선)은 거부,
  마스킹 함수가 원본 길이보다 정보를 더 흘리지 않는다.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — `crypto/aes` + `cipher.NewGCM`, nonce는 `crypto/rand`로 매번 생성해
  ciphertext 앞에 붙인다. `crypto/hmac` + `sha256`.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(crypto): AES-GCM 암호화와 HMAC 해시`

> **왜 HMAC인가:** 한국 휴대폰 번호는 `010` + 8자리라 경우의 수가 1억 미만이다.
> 순수 SHA256이면 평범한 노트북으로 몇 분 만에 전수 대입해 전부 복원된다. pepper가 이걸 막는다.

---

### Task 3: 도메인 — Claim과 상태 전이

**Files:**
- Create: `internal/domain/claim.go`, `internal/domain/claim_test.go`, `internal/domain/errors.go`

**Interfaces:**
- Produces:
  ```go
  type Status string
  const (StatusPending, StatusNeedsReview, StatusOnHold, StatusApproved, StatusRejected, StatusPaid)

  type IssueType string
  const (IssueDollStuck, IssueCashEaten, IssueClawBroken, IssueOther)

  type Actor struct { Kind string; ID string }   // Kind: "customer" | "owner" | "system"

  type Event struct {
      ClaimID string
      Actor   Actor
      Action  string          // "create" | "transition" | "view_contact"
      From, To Status
      Note    string
      At      time.Time
  }

  type Claim struct {
      ID, StoreID, MachineID string
      IssueType   IssueType
      AmountKRW   int
      Description string
      Status      Status
      RiskScore   int
      RiskReasons []RiskReason
      CreatedAt, ResolvedAt, PaidAt time.Time
      PayoutMethod string           // "deeplink" | "manual" | ""
  }

  func (c *Claim) CanTransition(to Status) bool
  func (c *Claim) Transition(to Status, by Actor, note string) (Event, error)
  func ParseIssueType(string) (IssueType, error)
  ```

- [ ] **Step 1: 실패하는 테스트** — 허용 전이 전수 표(`pending→approved`, `pending→rejected`,
  `needs_review→approved/rejected`, `on_hold→needs_review/rejected`, `approved→paid`,
  `approved→rejected`)와 금지 전이 전수(`paid→*` 전부 거부, `rejected→*` 전부 거부,
  `pending→paid` 직행 거부, 자기 자신으로의 전이 거부). `Transition` 성공 시 `Event`에
  from/to/actor/시각이 담기고 `Claim.Status`가 갱신된다. 실패 시 Claim은 변경되지 않는다.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — 허용 전이를 `map[Status][]Status`로 선언하고 전이 함수는 그 표만 본다.
  `approved→paid`에서 `PaidAt`을, 종료 전이에서 `ResolvedAt`을 채운다.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(domain): Claim 엔티티와 상태 전이 규칙`

> **`approved→rejected`를 허용하는 이유:** 승인했는데 송금 직전에 사진을 다시 보니 조작이었다거나,
> 계좌가 틀려 송금이 반려되는 경우가 실제로 생긴다. 되돌릴 길이 없으면 사장님이 DB를 직접 고친다.

---

### Task 4: 도메인 — 리스크 평가

**Files:**
- Create: `internal/domain/policy.go`, `internal/domain/risk.go`, `internal/domain/risk_test.go`

**Interfaces:**
- Produces:
  ```go
  type Policy struct {
      ReviewThresholdKRW   int  // 10000
      RepeatWatchCount     int  // 3
      RepeatHoldCount      int  // 5
      AccountSharingPhones int  // 3
      PayoutCeilingKRW     int  // 50000
      RapidDuplicateWindow time.Duration // 10분
      MaxAmountKRW         int  // 1000000, 초과는 접수 거부
  }
  func DefaultPolicy() Policy

  type RiskInput struct {
      AmountKRW            int
      PhoneClaims30d       int
      PhonePaidTotal30d    int
      AccountDistinctPhones int
      ManualStatus         string  // "normal" | "watch" | "blocked"
      LastSameMachineAt    time.Time // 같은 전화+같은 기계 직전 제출 (없으면 zero)
      Now                  time.Time
  }

  type RiskReason struct { Code, Message string }  // Message는 사장님에게 그대로 보이는 한국어
  type RiskResult struct {
      Score      int
      Reasons    []RiskReason
      Status     Status  // pending | needs_review | on_hold
      RejectDuplicate bool
  }
  func Evaluate(in RiskInput, p Policy) RiskResult
  ```

- [ ] **Step 1: 실패하는 테스트** — 규칙별 경계값을 전수로:
  - 금액 9,999 → `pending` / 10,000 → `needs_review` / 10,001 → `needs_review`
  - 금액 0·음수 → 유효성 에러 (Evaluate 이전 단계에서 걸러야 함을 명시하는 테스트)
  - `PhoneClaims30d` 2 → 플래그 없음 / 3 → `needs_review` / 4 → `needs_review` / 5 → `on_hold`
  - `AccountDistinctPhones` 2 → 없음 / 3 → `needs_review`
  - `PhonePaidTotal30d` 50,000 → 없음 / 50,001 → `needs_review`
  - `ManualStatus == "blocked"` → 다른 모든 값이 정상이어도 `on_hold`
  - `LastSameMachineAt`이 9분 전 → `RejectDuplicate true` / 11분 전 → false / zero → false
  - **합성 우선순위:** `on_hold` > `needs_review` > `pending`. 여러 규칙이 동시에 걸리면
    가장 강한 상태가 이기고 `Reasons`에는 **발동한 규칙이 전부** 담긴다.
  - 모든 `Reason.Message`가 비어 있지 않고 숫자가 채워진 한국어 문장이다
    ("이 번호로 30일간 4번째 신고입니다").
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — 규칙 하나당 함수 하나, `Evaluate`는 그것들을 순회하며 결과를 합성.
  새 규칙 추가가 기존 규칙을 건드리지 않는 형태로.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(domain): 리스크 평가와 블랙리스트 규칙`

> **점수가 아니라 문장을 반환하는 이유:** 사장님은 "위험도 72점"으로 판단할 수 없다.
> "이 번호로 30일간 4번째 신고입니다"를 봐야 승인할지 전화를 걸지 결정한다.

---

### Task 5: 스키마 마이그레이션과 Postgres 리포지토리

**Files:**
- Create: `migrations/0001_init.sql`, `internal/store/store.go`, `internal/store/migrate.go`,
  `internal/store/claim.go`, `internal/store/machine.go`, `internal/store/user.go`,
  `internal/store/contact.go`, `internal/store/store_integration_test.go`
- Create: `docker-compose.yml` (postgres + redis만 먼저)

**Interfaces:**
- Consumes: `domain.Claim`, `domain.Status`, `crypto.Cipher`, `crypto.Hasher`
- Produces:
  ```go
  type Store struct{ pool *pgxpool.Pool; cipher *crypto.Cipher; hasher *crypto.Hasher }
  func Open(ctx, dsn string, c *crypto.Cipher, h *crypto.Hasher) (*Store, error)
  func (s *Store) Migrate(ctx) error                       // embed된 SQL을 번호순 적용

  func (s *Store) MachineByCode(ctx, code string) (Machine, error)   // 비활성이면 ErrNotFound
  func (s *Store) CreateClaim(ctx, in CreateClaimInput) (domain.Claim, error)
  func (s *Store) ClaimByID(ctx, storeID, id string) (ClaimDetail, error) // 복호화된 연락처 포함
  func (s *Store) ListClaims(ctx, f ClaimFilter) ([]ClaimSummary, Cursor, error)
  func (s *Store) ApplyTransition(ctx, id string, ev domain.Event, next domain.Status) error
  func (s *Store) RiskFactsFor(ctx, phoneHash, accountHash []byte, machineID string, now time.Time) (domain.RiskInput, error)
  func (s *Store) SetContactStatus(ctx, hash []byte, kind, status, note string) error
  func (s *Store) ListContacts(ctx, storeID string) ([]ContactSummary, error)
  func (s *Store) MachineDailyCounts(ctx, storeID string, from, to time.Time) ([]DailyCount, error)
  func (s *Store) ClaimsInLast24h(ctx, machineID string) (int, []domain.IssueType, error)
  func (s *Store) ExpiredPhotos(ctx, now time.Time, limit int) ([]Photo, error)
  func (s *Store) DeletePhotoRow(ctx, id string) error
  ```

- [ ] **Step 1: 마이그레이션 SQL 작성** — 스펙 §5의 테이블 전부. 핵심 제약:
  - `claims.idempotency_key` 컬럼에 **`UNIQUE (store_id, idempotency_key)`**
    — Review Focus #1의 최후 방어선. 캐시가 아니라 DB가 중복을 막는다.
  - `machines.code UNIQUE`, `users.email UNIQUE`
  - `claims.amount_krw` 에 `CHECK (amount_krw > 0)`
  - `contact_flags` PK는 `(identity_hash, identity_type, store_id)`
  - 스펙 §5의 인덱스 4개
  - `claim_events`에는 UPDATE/DELETE를 쓰지 않는다 (append-only, 코드 규약)
- [ ] **Step 2: 실패하는 통합 테스트** (`//go:build integration`) —
  마이그레이션이 두 번 실행돼도 안전하다(멱등), `CreateClaim` 후 `ClaimByID`로 읽으면
  전화번호·계좌가 **평문으로 복원**된다, 같은 `idempotency_key`로 두 번 `CreateClaim` 하면
  두 번째는 `ErrDuplicate`, `MachineByCode`가 `active=false` 기계에 `ErrNotFound`,
  `RiskFactsFor`가 30일 경계(29일 전 포함 / 31일 전 제외)를 정확히 센다,
  `ApplyTransition`이 claim 상태 갱신과 event 삽입을 **한 트랜잭션**으로 처리한다.
- [ ] **Step 3: 테스트 실패 확인** — `docker compose up -d postgres` 후
  `go test -tags=integration ./internal/store/`
- [ ] **Step 4: 구현** — pgx/v5 pool. 쓰기 시 `cipher.Encrypt` + `hasher.Hash`,
  읽기 시 `Decrypt`. `RiskFactsFor`는 스펙 §5대로 `claims`를 실시간 집계한다.
- [ ] **Step 5: 테스트 통과 확인**
- [ ] **Step 6: 커밋** `feat(store): 스키마 마이그레이션과 Postgres 리포지토리`

---

### Task 6: 캐시 (인메모리 + Redis)

**Files:**
- Create: `internal/cache/cache.go`, `internal/cache/memory.go`, `internal/cache/redis.go`,
  `internal/cache/memory_test.go`

**Interfaces:**
- Produces:
  ```go
  type Cache interface {
      Incr(ctx, key string, ttl time.Duration) (int64, error)
      SetNX(ctx, key, val string, ttl time.Duration) (bool, error)
      Get(ctx, key string) (string, bool, error)
  }
  func NewMemory() Cache
  func NewRedis(url string) (Cache, error)
  func New(url string) (Cache, error)   // url이 빈 문자열이면 NewMemory
  ```

- [ ] **Step 1: 실패하는 테스트** (memory 구현 대상) — `Incr`가 1,2,3으로 증가하고
  TTL 경과 후 1로 리셋, `SetNX`가 최초엔 true 이후엔 false, TTL 만료 후 다시 true,
  동시 `Incr` 1000회가 정확히 1000을 반환한다(`-race`로 검증).
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — memory는 `sync.Mutex` + 만료시각을 담은 맵, 백그라운드 청소 goroutine.
  redis는 `INCR`+`EXPIRE`를 파이프라인으로, `SET NX EX`.
- [ ] **Step 4: 테스트 통과 확인 (`go test -race`)**
- [ ] **Step 5: 커밋** `feat(cache): 인메모리·Redis 캐시 구현`

---

### Task 7: 사진 저장소

**Files:**
- Create: `internal/media/media.go`, `internal/media/local.go`, `internal/media/local_test.go`

**Interfaces:**
- Produces:
  ```go
  type Storage interface {
      Put(ctx, key string, r io.Reader, contentType string, size int64) error
      Open(ctx, key string) (io.ReadCloser, string, error)
      Delete(ctx, key string) error
  }
  func NewLocal(dir string) (Storage, error)
  func DetectImageType(head []byte) (string, error)  // jpeg/png/webp/heic만 허용
  ```

- [ ] **Step 1: 실패하는 테스트** — Put 후 Open으로 같은 바이트가 나온다, Delete 후 Open은
  실패, **경로 탈출 키(`../../etc/passwd`)가 거부된다**, `DetectImageType`이 매직바이트로
  jpeg/png/webp를 식별하고 텍스트 파일·실행파일은 거부한다.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — 키는 서버가 생성하고(`claims/{claimID}/{uuid}.{ext}`) 클라이언트 입력을
  그대로 경로에 쓰지 않는다. 검증은 `filepath.Clean` + 베이스 디렉터리 접두사 확인.
  **확장자를 믿지 않고 매직바이트로 판정한다** — `.jpg`로 올라온 실행 파일을 막는다.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(media): 로컬 사진 저장소와 이미지 타입 검증`

---

### Task 8: 알림 (Slack)

**Files:**
- Create: `internal/notify/notify.go`, `internal/notify/slack.go`, `internal/notify/slack_test.go`

**Interfaces:**
- Produces:
  ```go
  type Notifier interface {
      ClaimCreated(ctx, n ClaimNotice) error
      MachineAlert(ctx, n MachineNotice) error
      DailyDigest(ctx, n DigestNotice) error
  }
  func NewSlack(webhookURL string, baseURL string) Notifier
  func NewNoop() Notifier          // webhook이 비었을 때
  ```

- [ ] **Step 1: 실패하는 테스트** — `httptest` 서버로 Slack webhook을 흉내내고,
  전송된 JSON에 기계 라벨·증상·금액·리스크 사유·대시보드 링크가 포함되며
  **전화번호·계좌번호는 포함되지 않는다**(알림 채널로 개인정보를 흘리지 않는다),
  `on_hold` 건은 메시지에 보류 사유가 앞에 오고, webhook이 500을 반환하면 에러를 반환하되
  **호출자가 무시할 수 있는 에러**임을 문서화한다. Noop은 항상 nil.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — Slack Block Kit. 타임아웃 5초.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(notify): Slack 알림 구현`

> **알림 실패가 접수 실패가 되면 안 된다.** Slack이 죽어도 claim은 DB에 남고 대시보드에 뜬다.
> 알림은 부가 경로이고 대시보드가 단일 진실 공급원이다.

---

### Task 9: 송금 (딥링크)

**Files:**
- Create: `internal/payout/payout.go`, `internal/payout/deeplink.go`, `internal/payout/deeplink_test.go`,
  `internal/payout/banks.go`

**Interfaces:**
- Produces:
  ```go
  type Link struct { Provider, Label, URL string }
  type Request struct { BankCode, AccountNo, Holder string; AmountKRW int }
  type Payout interface {
      Links(Request) ([]Link, error)   // 사장님에게 보여줄 송금 수단들
      Kind() string                    // "deeplink"
  }
  func NewDeeplink(templates map[string]string) Payout

  type Bank struct{ Code, Name string }
  func Banks() []Bank                  // 국내 주요 은행 + 인터넷은행
  func BankByCode(code string) (Bank, bool)
  ```

- [ ] **Step 1: 실패하는 테스트** — 템플릿 `supertoss://send?bank={bank}&accountNo={account}&amount={amount}`에
  값이 치환되고 **각 값이 URL 인코딩된다**(예금주 한글, 특수문자), 계좌번호에 하이픈이 있으면
  제거된 값이 들어간다, 금액이 0 이하면 에러, 알 수 없는 은행코드면 에러,
  템플릿이 하나도 설정되지 않으면 **빈 슬라이스를 반환하고 에러는 아니다**
  (수동 기록 경로로 자연히 떨어진다).
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — 템플릿은 설정 주입. 코드에 스킴을 하드코딩하지 않는다.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(payout): 송금 딥링크 생성`

> **딥링크 URL 스킴은 벤더가 예고 없이 바꾸는 값이다.** 설정으로 빼두면 스킴이 바뀌어도
> 환경변수만 고치면 되고, 어떤 스킴도 안 열리면 UI가 자동으로 수동 기록 경로를 보여준다.

---

### Task 10: 공개 접수 API

**Files:**
- Create: `internal/httpapi/server.go`, `internal/httpapi/public.go`, `internal/httpapi/middleware.go`,
  `internal/httpapi/render.go`, `internal/httpapi/public_test.go`, `cmd/api/main.go`

**Interfaces:**
- Consumes: 앞선 모든 패키지
- Produces: `GET /api/public/machines/{code}`, `POST /api/public/claims`,
  `GET /healthz`. 미들웨어: 레이트리밋, 요청 크기 제한, 요청 ID, 복구, CORS.

- [ ] **Step 1: 실패하는 테스트** — fake store/cache/notify로 `httptest`:
  - 정상 접수 → 201, 응답에 접수번호가 있고 **계좌·전화번호는 응답에 다시 담기지 않는다**
  - **Review Focus #3:** 금액 0 → 400, 음수 → 400, 9,999 + 사진 0장 → 201,
    10,000 + 사진 0장 → **400**, 10,000 + 사진 1장 → 201, 1,000,001 → 400
  - **Review Focus #4:** 없는 코드 → 404, `active=false` 기계 → 404. 두 경우 모두
    claim이 생성되지 않았음을 fake store로 확인
  - **Review Focus #5:** 사진 4장 → 400, 5MB 초과 1장 → 400, 텍스트 파일을 `.jpg`로 → 400.
    거부된 요청에서 **저장소에 아무것도 남지 않는다**
  - **Review Focus #1:** 같은 `Idempotency-Key` 두 번 → 두 번째는 최초와 같은 접수번호로 200,
    claim은 1건만 생성. store가 `ErrDuplicate`를 반환하는 경로(캐시를 통과한 경합)도 같은 결과
  - **Review Focus #2:** cache가 모든 호출에 에러를 반환해도 접수는 201로 성공한다
  - `Idempotency-Key` 헤더 누락 → 400
  - 레이트리밋 초과 → 429
  - 잘못된 전화번호 형식 → 400
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — `multipart/form-data` 파싱, `http.MaxBytesReader`로 16MB 상한,
  `domain.Evaluate` 호출 후 결과 상태로 저장, 저장 성공 후 알림 발송(실패는 로깅만).
  캐시 오류는 로깅 후 통과 — DB 유니크 제약이 받아낸다.
- [ ] **Step 4: 테스트 통과 확인 (`-race`)**
- [ ] **Step 5: `cmd/api/main.go`** — config 로드, 의존성 조립, graceful shutdown
- [ ] **Step 6: 커밋** `feat(api): 공개 신고 접수 엔드포인트`

---

### Task 11: 관리자 인증과 claim 처리 API

**Files:**
- Create: `internal/httpapi/auth.go`, `internal/httpapi/admin_claims.go`,
  `internal/httpapi/session.go`, `internal/httpapi/admin_claims_test.go`

**Interfaces:**
- Produces: `POST /api/admin/login`·`logout`, `GET /api/admin/claims`,
  `GET /api/admin/claims/{id}`, `POST .../approve`·`reject`·`mark-paid`,
  `GET .../payout-link`

- [ ] **Step 1: 실패하는 테스트** — 미인증 요청은 401, 다른 매장의 claim 조회는 404
  (403이 아니라 404 — 존재 여부를 알려주지 않는다), 상세 조회 시 계좌·전화번호가
  복호화되어 반환되고 **그 조회가 `claim_events`에 기록된다**,
  `pending→approved` 200 / `paid→approved` 409, `mark-paid`에 `method`("deeplink"|"manual")가
  필수, 비밀번호 5회 실패 시 잠금, 세션 쿠키가 HttpOnly·SameSite=Lax로 설정된다.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — bcrypt 비밀번호, 서명된 세션 쿠키(`SESSION_SECRET`),
  상태 전이는 반드시 `domain.Claim.Transition`을 거친다(핸들러가 상태를 직접 쓰지 않는다).
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(api): 관리자 인증과 claim 처리`

---

### Task 12: 기계·통계·연락처 API와 QR 생성

**Files:**
- Create: `internal/httpapi/admin_machines.go`, `internal/httpapi/admin_stats.go`,
  `internal/httpapi/admin_contacts.go`, `internal/qrcode/qrcode.go`,
  `internal/httpapi/admin_machines_test.go`

**Interfaces:**
- Produces: `GET/POST/PATCH /api/admin/machines`, `GET /api/admin/machines/{id}/qr.png`,
  `GET /api/admin/stats/daily`, `GET /api/admin/contacts`, `PATCH /api/admin/contacts/{hash}`

- [ ] **Step 1: 실패하는 테스트** — 기계 생성 시 `code`가 자동 생성되고 충돌하면 재시도,
  QR PNG가 유효한 이미지이고 디코드하면 `PUBLIC_BASE_URL/r/{code}`가 나온다,
  통계가 날짜 범위로 필터되고 빈 날짜는 0으로 채워진다(그래프가 끊기지 않게),
  연락처 목록이 신고 횟수 내림차순이고 **전화번호는 마스킹되어 반환된다**,
  `manual_status`를 `blocked`로 바꾸면 이후 `RiskFactsFor`에 반영된다.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — QR은 `github.com/skip2/go-qrcode`. 기계 코드는
  혼동 문자(0/O, 1/I/l)를 뺀 알파벳으로 6자리 — 스티커가 닳아도 사람이 읽고 입력할 수 있게.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(api): 기계 관리·통계·연락처 엔드포인트`

---

### Task 13: 워커 (집계·알림·사진 만료)

**Files:**
- Create: `cmd/worker/main.go`, `internal/worker/worker.go`, `internal/worker/worker_test.go`

**Interfaces:**
- Produces:
  ```go
  func (w *Worker) CheckMachineAlerts(ctx, now time.Time) error  // 접수 직후에도 호출 가능
  func (w *Worker) SendDailyDigest(ctx, day time.Time) error
  func (w *Worker) PurgeExpiredPhotos(ctx, now time.Time) (int, error)
  ```

- [ ] **Step 1: 실패하는 테스트** — 24시간 내 2건인 기계는 알림 없음 / 3건이면 알림,
  **같은 기계에 24시간 내 두 번 알리지 않는다**(캐시 디듀프), 일일 요약이 전일 데이터만 집계,
  만료된 사진은 저장소 파일과 DB 행이 **둘 다** 삭제되고 파일이 이미 없어도 DB 행은 지워진다
  (고아 행이 영원히 남지 않게).
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 구현** — `cmd/worker`는 `time.Ticker` 루프. 일일 요약은 KST 09:00 기준.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 커밋** `feat(worker): 기계 알림·일일 요약·사진 만료 삭제`

---

### Task 14: 프론트 부트스트랩과 디자인 토큰

**Files:**
- Create: `web/package.json`, `web/next.config.ts`, `web/tsconfig.json`,
  `web/app/globals.css`, `web/app/layout.tsx`, `web/lib/api.ts`, `web/lib/format.ts`,
  `web/components/ui/*`

- [ ] **Step 1: Next.js 15 + Tailwind v4 초기화** — v4는 `@tailwindcss/postcss`를 쓴다
  (`tailwindcss` 플러그인을 postcss에 직접 넣으면 안 된다)
- [ ] **Step 2: Pretendard 자체 호스팅** — `next/font/local`, `font-display: swap`.
  Google Fonts `<link>` 금지
- [ ] **Step 3: 디자인 토큰 정의** — `globals.css`의 `@theme`에:
  웜 뉴트럴 그레이 9단계, 액센트 1색, 상태색 3색(승인 초록 / 보류 앰버 / 거절 레드),
  라이트·다크 양쪽. 모든 상태색은 배경 대비 **WCAG AA 4.5:1 이상**
  (사장님이 햇빛 아래 폰으로 본다)
- [ ] **Step 4: 공용 컴포넌트** — Button, Field, Badge, Card, Sheet. Phosphor 아이콘 1.5 고정
- [ ] **Step 5: `lib/api.ts`** — fetch 래퍼, 에러 타입, 금액/날짜 한국어 포매터
- [ ] **Step 6: 커밋** `feat(web): Next.js 부트스트랩과 디자인 토큰`

---

### Task 15: 손님 신고 폼

**Files:**
- Create: `web/app/r/[code]/page.tsx`, `web/app/r/[code]/claim-form.tsx`,
  `web/app/r/[code]/done/page.tsx`, `web/lib/image.ts`, `web/lib/banks.ts`

- [ ] **Step 1: 기계 조회 + 에러 화면** — 없는 코드면 "이 스티커의 기계를 찾을 수 없어요"와
  매장 연락처. 빈 화면이나 스택트레이스 금지
- [ ] **Step 2: 폼 본문** — 한 화면 구성. 증상 4개는 큰 터치 타깃(드롭다운 금지),
  금액, 사진(`capture="environment"`), 전화번호, 은행 그리드 + 계좌(`inputmode="numeric"`),
  예금주. `min-h-[100dvh]`
- [ ] **Step 3: 클라이언트 이미지 리사이즈** (`lib/image.ts`) — canvas로 긴 변 1600px,
  JPEG 0.8. 원본 그대로 올리면 지하 매장 LTE에서 제출이 실패한다
- [ ] **Step 4: 금액 임계 인라인 안내** — 10,000원 이상을 입력하는 순간
  "사진이 필요하고, 확인 후 연락드릴 수 있습니다"가 인라인으로 뜨고 사진 필드가 필수로 바뀐다.
  제출 후 거부보다 입력 중에 아는 게 낫다
- [ ] **Step 5: 제출** — `Idempotency-Key`는 폼 마운트 시 1회 생성해 재시도 간 유지.
  전송 중 버튼 비활성 + 진행 표시. 실패 시 입력값을 보존한 채 재시도 버튼
- [ ] **Step 6: 완료 화면** — 접수번호, 처리 안내. 리스크로 보류돼도 **정상 접수와 동일한 화면**
  (스펙 §4.4 — 차단 사실을 노출하면 우회를 학습시킨다)
- [ ] **Step 7: 커밋** `feat(web): 손님 신고 폼`

---

### Task 16: 관리자 로그인과 접수함

**Files:**
- Create: `web/app/admin/login/page.tsx`, `web/app/admin/layout.tsx`,
  `web/app/admin/page.tsx`, `web/components/claim-row.tsx`, `web/components/risk-badge.tsx`

- [ ] **Step 1: 로그인 화면** — 장식 없이. 실패 시 "이메일 또는 비밀번호가 올바르지 않습니다"
  (어느 쪽이 틀렸는지 알려주지 않는다)
- [ ] **Step 2: 접수함** — 상태 탭(검토필요 / 보류 / 대기 / 승인됨 / 완료),
  기본 진입은 **검토필요**. 행마다 기계 라벨, 증상, 금액, 경과 시간, 리스크 배지
- [ ] **Step 3: 리스크 배지** — 색만 쓰지 않고 아이콘 + 라벨 병기(색각 이상 대응).
  탭하면 발동 규칙 문장 전체를 펼친다
- [ ] **Step 4: 커서 페이징 + 30초 폴링**
- [ ] **Step 5: 커밋** `feat(web): 관리자 로그인과 접수함`

---

### Task 17: claim 상세와 환불 처리

**Files:**
- Create: `web/app/admin/claims/[id]/page.tsx`, `web/components/payout-sheet.tsx`,
  `web/components/photo-viewer.tsx`

- [ ] **Step 1: 상세 화면** — 사진(탭하면 전체화면 확대 — 조작 여부를 보려면 확대가 필수),
  증상, 금액, 기계, 리스크 사유 전문, 이 연락처의 과거 신고 이력
- [ ] **Step 2: 통화 버튼** — 10,000원 이상이면 `tel:` 링크를 **주요 동작으로** 노출
- [ ] **Step 3: 승인/거절** — 거절 시 사유 선택. 되돌릴 수 없는 동작은 확인 시트를 거친다
- [ ] **Step 4: 송금 시트** — 승인 후 열린다. 딥링크 버튼(토스/카뱅) + 계좌번호 원터치 복사.
  앱에서 돌아오면 "보내셨나요?"를 묻고 예 → `mark-paid(deeplink)`.
  딥링크가 없거나 PC면 복사 + 수동 "송금 완료"(`mark-paid(manual)`)만 보여준다
- [ ] **Step 5: 감사 로그 타임라인** — 누가 언제 무엇을 했는지
- [ ] **Step 6: 커밋** `feat(web): claim 상세와 환불 처리`

---

### Task 18: 기계 관리와 연락처 화면

**Files:**
- Create: `web/app/admin/machines/page.tsx`, `web/app/admin/machines/qr-sheet.tsx`,
  `web/app/admin/contacts/page.tsx`

- [ ] **Step 1: 기계 목록** — 라벨, 위치, 최근 7일 접수 추이 스파크라인, 이상 표시
- [ ] **Step 2: QR 시트** — QR 미리보기 + PNG 다운로드 + **A4 스티커 인쇄 레이아웃**.
  QR 아래에 기계 라벨과 "작동 불량 / 오류 시 스캔" 문구를 넣는다.
  QR만 있으면 손님이 무엇인지 모르고 지나친다
- [ ] **Step 3: 기계 추가/수정/비활성화**
- [ ] **Step 4: 연락처 화면** — 마스킹된 번호, 신고 횟수, 누적 지급액, 수동 상태 변경.
  각 행에서 해당 신고 목록으로 이동
- [ ] **Step 5: 커밋** `feat(web): 기계 관리와 연락처 화면`

---

### Task 19: 배포 구성과 문서

**Files:**
- Create: `Dockerfile.api`, `Dockerfile.worker`, `web/Dockerfile`,
  `docker-compose.yml`(전체), `Makefile`, `docs/DEPLOY.md`
- Modify: `README.md`

- [ ] **Step 1: 멀티스테이지 Dockerfile** — Go는 distroless, web은 standalone 출력
- [ ] **Step 2: compose 전체** — api, worker, web, postgres, redis + 헬스체크 + 볼륨
- [ ] **Step 3: Makefile** — `make dev`, `make test`, `make test-integration`, `make migrate`,
  `make keys`(암호화 키 3종 생성)
- [ ] **Step 4: `docs/DEPLOY.md`** — VPS 배포 절차, 키 생성, 첫 사장님 계정 만들기,
  Slack webhook 발급, 백업(개인정보가 든 DB라 암호화 백업)
- [ ] **Step 5: 전체 테스트 통과 확인** — `make test && make test-integration`
- [ ] **Step 6: 커밋** `chore: 배포 구성과 운영 문서`

---

## 실행 방식

이 계획은 **Native**(이 세션에서 직접 구현)로 실행한다. 태스크 간 인터페이스 의존이 촘촘해서
(Task 10은 Task 1~9의 시그니처를 전부 쓴다) 태스크마다 컨텍스트가 비워지는 subagent 방식은
인터페이스 불일치를 만들기 쉽다. 각 단계는 TDD 순서를 지킨다:
실패하는 테스트 → 실패 확인 → 최소 구현 → 통과 확인 → 커밋.
