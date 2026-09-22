# 재고 장부 구현 계획 (커뮤니티 1단계)

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans 로 태스크 단위 구현.
> 각 단계는 체크박스(`- [ ]`)로 추적한다.

**Goal:** 사장님이 인형 사입을 기록하고, 자기 원가와 재고를 본다.
**매장이 한 곳이어도 쓸모가 있어야 한다** — 그게 이 단계의 전부이자 유일한 합격 기준이다.

**Why first:** 커뮤니티 세 기능(단가 비교·장터·순위)은 전부 사입 데이터를 연료로 쓴다.
비교부터 만들면 빈 표가 뜨고, 아무도 두 번째로 열지 않는다. 사장님이 **자기 원가를
보려고** 입력한 것의 부산물이 비교 데이터가 된다.

**Spec:** `docs/superpowers/specs/2026-09-22-community-spec.md` §3, §9 Q1·Q2

**Tech Stack:** 기존과 동일. Go 1.24 표준 `net/http`, pgx/v5, PostgreSQL 16,
Next.js 15 App Router, Tailwind v4, Pretendard, Phosphor Icons.

---

## Global Constraints

기존 `2026-09-21-clawhub-phase1.md`의 Global Constraints를 전부 상속한다. 이 단계에만
해당하는 것을 덧붙인다.

- **금액은 전부 정수 원(KRW).** 개당 실단가는 나눗셈이 들어가므로 **반올림 규칙을
  한 함수에 가둔다.** 화면과 서버가 각자 나누면 1원씩 어긋나고, 그 1원을 사장님이 먼저 본다.
- **재고 수량 테이블을 따로 두지 않는다.** 현재 재고는 `purchases` 합계에서
  `inventory_adjustments` 합계를 더해 그때그때 계산한다. 기존 `RiskFactsFor`와 같은
  원칙이다 — 이 데이터 규모에서 인덱스 스캔은 밀리초이고, 카운터를 두면 정합성이
  깨질 여지만 생긴다.
- **두 테이블 다 append-only.** 사입 기록을 고치는 대신 조정을 쌓는다. 장부는 고쳐
  쓰는 게 아니라 이어 쓰는 것이다.
- **`purchases.doll_id`가 NULL인 행은 매장 간 집계에 들어가지 않는다.** 1단계에는 전부
  NULL이다 (카탈로그 없음, 스펙 §9 Q2). 이 규칙을 코드에 박아둬야 "일단 대충 세자"가
  나중에 스며들지 않는다. 한 매장 안에서는 정규화한 이름(`name_key`)으로 묶는다.
- **자동완성은 같은 매장의 과거 이름만 돌려준다.** 전체에서 찾아주면 편하지만, 그건
  "남의 매장이 요즘 뭘 들이는지"를 흘리는 것이다.
- **영수증 사진은 자동 삭제하지 않는다.** 신고 사진은 개인정보라 보존 기간이 지나면
  지우지만, 영수증은 사장님 장부의 증빙이다. 성격이 반대다.
- **마진·수익률은 이 단계에서 만들지 않는다.** 기계 데이터가 없어 매출을 모른다.
  추정치를 숫자로 보여주면 그건 거짓말이다 (스펙 §10).

## Review Focus

스펙이 전제하지만 어느 태스크의 테스트도 자연히 다루지 않는 입력들.
각 항목의 테스트는 괄호 안 태스크가 소유한다.

1. **개당 실단가의 반올림 경계** — 배송비 3,000원을 7개에 나누면 428.57원이다.
   버림·올림·반올림 중 무엇인지, 그리고 **개당 실단가 × 수량이 실제 결제액과
   얼마나 어긋나는지**를 정해둬야 한다. 배송비 0, 수량 1도 같은 함수를 지난다. (Task 20)
2. **같은 인형의 다른 표기** — `쿠로미 중형` / `쿠로미  중형`(두 칸) / `쿠로미중형` /
   `쿠로미 중형 ` 는 한 매장 장부 안에서 한 줄로 묶여야 한다. 자유 입력을 받는 이상
   정규화가 유일한 방어선이다. 반대로 `쿠로미 대형`은 묶이면 안 된다. (Task 20)
3. **재고가 음수로 내려가는 조정** — 실사에서 "지금 0개"는 정상이고 "-3개"는 오타다.
   0은 받고 음수는 막는다. (Task 20, Task 22)
4. **남의 매장 격리** — 목록·상세·조정·**자동완성** 전부. 자동완성이 특히 위험하다.
   한 글자만 쳐도 남의 매장 품목이 쏟아질 수 있는 자리다. (Task 21, Task 22)
5. **더블탭 이중 등록** — 제출 버튼을 두 번 누르면 60개가 120개가 된다. 신고 접수와
   같은 `Idempotency-Key` + DB 유니크 제약으로 막는다. 캐시가 아니라 제약이 방어선이다.
   (Task 21, Task 22)
6. **미래 날짜 / 아주 오래된 날짜** — 사입일을 `datetime-local`로 받으면 2206년이
   들어온다. 미래는 거부하고, 과도하게 과거(5년 전)도 거부한다. (Task 20)

---

### Task 20: 도메인 — 사입 계산과 이름 정규화

**Files:**
- Create: `internal/domain/purchase.go`, `internal/domain/purchase_test.go`

**Interfaces:**
- Produces:
  ```go
  // UnitCostKRW는 배송비를 얹은 개당 실단가다. 원 단위로 반올림한다.
  func UnitCostKRW(unitPriceKRW, qty, shippingKRW int) int

  // TotalCostKRW는 실제 결제액이다. unitCost*qty 가 아니다 — 반올림 때문에 다르다.
  func TotalCostKRW(unitPriceKRW, qty, shippingKRW int) int

  // NameKey는 같은 인형을 한 줄로 묶기 위한 비교용 키다.
  func NameKey(name string) string

  type PurchaseInput struct {
      Name        string
      Vendor      string
      UnitPriceKRW int
      Qty         int
      ShippingKRW int
      PurchasedAt time.Time
  }
  func ValidatePurchase(in PurchaseInput, now time.Time) error

  // ValidateAdjustment는 실사 보정값을 검증한다. 0은 정상, 음수는 오타다.
  func ValidateAdjustment(countedQty int) error
  ```
- 다른 `internal/*`를 import 하지 않는다.

- [ ] **Step 1: 실패하는 테스트**
  - `UnitCostKRW(2300, 60, 3000) == 2350` (138,000+3,000=141,000 / 60)
  - `UnitCostKRW(2000, 7, 3000) == 2429` (17,000/7 = 2428.57 → 반올림)
  - `UnitCostKRW(2000, 1, 0) == 2000`, `UnitCostKRW(2000, 3, 0) == 2000`
  - `TotalCostKRW(2000, 7, 3000) == 17000` — `UnitCostKRW*qty`(17,003)와 다르다는 걸
    테스트가 직접 말한다. 이 차이를 모르면 "합계가 3원 더 나온다"는 버그가 난다.
  - `NameKey("쿠로미 중형") == NameKey("쿠로미  중형") == NameKey(" 쿠로미 중형 ")`
  - `NameKey("쿠로미중형") == NameKey("쿠로미 중형")` — 띄어쓰기는 사람마다 다르다
  - `NameKey("쿠로미 대형") != NameKey("쿠로미 중형")`
  - `NameKey("KUROMI") == NameKey("kuromi")`
  - `ValidatePurchase`: 수량 0·음수, 단가 음수, 배송비 음수, 이름 공백, 이름 80자 초과,
    미래 날짜, 5년 초과 과거 → 각각 한국어 에러
  - `ValidateAdjustment(0) == nil`, `ValidateAdjustment(-1) != nil`
- [ ] **Step 2: 테스트 실패 확인** — `go test ./internal/domain/ -run Purchase` → 컴파일 실패
- [ ] **Step 3: 최소 구현** — 반올림은 `(총액 + qty/2) / qty`. `NameKey`는 유니코드 공백
  전부 제거 + 소문자화. 왜 공백을 "접는" 게 아니라 "지우는"지 주석으로 남긴다.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: `arch_test.go`가 여전히 통과하는지 확인** (domain 격리)
- [ ] **Step 6: 커밋** `feat(domain): 사입 단가 계산과 인형 이름 정규화`

---

### Task 21: 스키마와 저장소

**Files:**
- Create: `internal/store/migrations/0005_inventory.sql`, `internal/store/inventory.go`
- Modify: `internal/store/store_integration_test.go`

**Interfaces:**
- Produces:
  ```go
  type CreatePurchaseInput struct {
      StoreID, UserID string
      Name, Vendor    string
      UnitPriceKRW, Qty, ShippingKRW int
      PurchasedAt     time.Time
      ReceiptKey      string
      IdempotencyKey  string
  }
  func (s *Store) CreatePurchase(ctx, CreatePurchaseInput) (Purchase, bool, error) // bool = 기존 건
  func (s *Store) ListPurchases(ctx, storeID string, nameKey string, limit int) ([]Purchase, error)
  func (s *Store) InventoryFor(ctx, storeID string) ([]InventoryRow, error)
  func (s *Store) InventoryItem(ctx, storeID, nameKey string) (InventoryRow, error)
  func (s *Store) AdjustInventory(ctx, storeID, userID, nameKey string, countedQty int, note string) error
  func (s *Store) DollNameSuggestions(ctx, storeID, q string, limit int) ([]string, error)
  func (s *Store) InventorySummary(ctx, storeID string, from, to time.Time) (InventorySummary, error)
  ```

**스키마 (0005_inventory.sql):**
```sql
CREATE TABLE purchases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  created_by uuid REFERENCES users(id),
  doll_id uuid,                      -- 1단계에는 늘 NULL. 카탈로그가 생기면 채운다.
  name text NOT NULL,                -- 사장님이 적은 그대로. 표본이자 화면 표기.
  name_key text NOT NULL,            -- domain.NameKey(name). 묶는 기준.
  vendor text NOT NULL DEFAULT '',
  unit_price_krw int NOT NULL CHECK (unit_price_krw >= 0),
  qty int NOT NULL CHECK (qty > 0),
  shipping_krw int NOT NULL DEFAULT 0 CHECK (shipping_krw >= 0),
  purchased_at timestamptz NOT NULL,
  receipt_key text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (store_id, idempotency_key)  -- 더블탭 방어선. 캐시가 아니라 여기다.
);
CREATE INDEX purchases_store_key_idx ON purchases (store_id, name_key, purchased_at DESC);

CREATE TABLE inventory_adjustments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  created_by uuid REFERENCES users(id),
  name_key text NOT NULL,
  delta int NOT NULL,        -- 실사값 - 계산값. 음수가 보통이다 (팔려나갔으니까).
  counted_qty int NOT NULL CHECK (counted_qty >= 0),
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inventory_adj_store_key_idx ON inventory_adjustments (store_id, name_key);
```

- [ ] **Step 1: 실패하는 테스트** (통합, `-tags=integration`)
  - 사입 1건 저장 후 되읽으면 이름·단가·수량이 그대로다
  - 같은 멱등키 재시도는 기존 건을 `existing=true`로 돌려주고 새 행을 만들지 않는다
  - 같은 `name_key`의 사입 2건이 재고에서 **한 줄**로 합쳐진다 (수량 합, 가중평균 원가)
  - 가중평균은 수량 가중이다 — 2,000원 10개와 3,000원 90개의 평균은 2,500이 아니라 2,900
  - 조정을 넣으면 재고 수량이 내려가고, 조정 이력이 남는다
  - `InventoryFor`는 **수량 0인 품목도 돌려준다** (다 팔렸다는 것도 정보다)
  - `DollNameSuggestions`가 남의 매장 이름을 섞지 않는다
  - `ListPurchases`가 남의 매장 건을 돌려주지 않는다
- [ ] **Step 2: 테스트 실패 확인** — 마이그레이션 없음
- [ ] **Step 3: 최소 구현** — 마이그레이션 + 쿼리. 재고는
  `purchases`를 `name_key`로 GROUP BY 하고 `inventory_adjustments` 합을 LEFT JOIN 해서 계산.
  가중평균 원가는 `SUM(unit_price*qty + shipping) / SUM(qty)`.
- [ ] **Step 4: 테스트 통과 확인** (Postgres 필요)
- [ ] **Step 5: 마이그레이션 두 번 실행해도 안전한지 확인** (기존 `TestMigrate_두번`)
- [ ] **Step 6: 커밋** `feat(store): 사입 기록과 재고 집계`

---

### Task 22: HTTP API

**Files:**
- Create: `internal/httpapi/admin_inventory.go`, `internal/httpapi/admin_inventory_test.go`
- Modify: `internal/httpapi/server.go` (라우트 + Store 인터페이스), `internal/httpapi/fake_test.go`

**Interfaces:**
```
GET    /api/admin/inventory                  재고 목록 + 요약
GET    /api/admin/inventory/{nameKey}        한 품목 상세 (사입 이력 포함)
POST   /api/admin/inventory/{nameKey}/count  실사 보정 {countedQty, note}
GET    /api/admin/purchases                  사입 이력
POST   /api/admin/purchases                  사입 등록 (multipart: 영수증 사진 선택)
GET    /api/admin/doll-names?q=              자동완성
```

- [ ] **Step 1: 실패하는 테스트**
  - 사입 등록 → 재고 목록에 반영
  - `Idempotency-Key` 없으면 400, 같은 키 재시도는 200 + 기존 건 (신고 접수와 같은 규약)
  - 수량 0·미래 날짜·빈 이름 → 400, 한국어 메시지
  - 실사 보정 음수 → 400
  - 미인증 전부 401
  - **응답에 `lastUnitCostKrw`가 있어서 "지난번엔 얼마였는지"를 화면이 알 수 있다**
  - 자동완성이 같은 매장 것만 돌려준다
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 최소 구현** — 영수증 업로드는 기존 `collectPhotos`/`writePhotos` 재사용.
  단, **보존 기간을 주지 않는다** (`ExpiresAt` 제로값) — 영수증은 지우지 않는다.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: 사진 만료 워커가 영수증을 지우지 않는지 확인** — `ExpiredPhotos`는
  `claim_photos`만 보므로 영향 없다. 테스트로 못 박는다.
- [ ] **Step 6: 커밋** `feat(api): 사입 등록과 재고 조회`

---

### Task 23: 프론트 — 재고 화면

**Files:**
- Create: `web/app/admin/(shell)/inventory/page.tsx`,
  `web/app/admin/(shell)/inventory/purchase-sheet.tsx`,
  `web/components/ui/combobox.tsx`
- Modify: `web/app/admin/(shell)/nav-sidebar.tsx`

**화면:**

```
재고                                    [+ 사입 기록]
─────────────────────────────────────────────────
이번 달 인형값   412,000원
재고 자산        1,284,000원   (12종 · 546개)

품목              보유    평균 원가    마지막 사입
쿠로미 중형 30cm   42개    2,350원     9/14 · 캐치돌
짱구 대형 45cm     18개    5,200원     9/02 · 토이토이
포켓몬 키링         0개    1,100원     8/21 · 도매꾹
```

- 카드가 아니라 **표**다 (스펙 §7.2). 한 화면에 12줄이 들어가야 한다.
- 숫자는 `tabular-nums`. 세로로 자릿수가 맞아야 눈으로 비교된다.
- 수량 0은 흐리게 — 지우지 않는다. 다 팔렸다는 것도 정보다.
- 폰에서는 표를 가로 스크롤 시키지 않는다. 줄당 2행으로 접는다.

**사입 기록 시트 (30초 안에 끝나야 한다):**
- 인형 이름: **콤보박스**. 치는 즉시 내 과거 품목이 뜨고, 고르면 끝.
  없으면 그대로 새 이름이 된다. 이게 §9 Q2 결정의 실행부다 —
  같은 사장님이 같은 인형을 두 번 다르게 적는 것만 막아도 표본 절반이 정리된다.
- 어디서: 최근 쓴 거래처 버튼 + 직접 입력
- 단가 / 수량 / 배송비(선택) / 날짜(기본 오늘) / 영수증(선택)
- **입력하는 동안 개당 실단가를 실시간으로 보여준다.** 배송비를 왜 적는지가 그때 보인다.
- 고른 인형에 과거 기록이 있으면 `지난번 2,480원 (8/21 · 캐치돌)`을 바로 아래 띄운다.

- [ ] **Step 1: 메뉴에 "재고" 추가** — 접수함과 기계 사이
- [ ] **Step 2: 목록 화면** — 빈 상태는 `[첫 사입 기록하기]` 버튼 하나 + 샘플 한 줄을
  흐리게. "아직 기록이 없습니다" 한 줄로 끝내지 않는다 (스펙 §7.2)
- [ ] **Step 3: 콤보박스** — 기존 `Field`/`Input`과 라벨 연결이 되어야 한다
  (`__labelable`). 키보드 위/아래/Enter/Esc 동작
- [ ] **Step 4: 사입 시트** — 실시간 개당 실단가, 지난번 단가
- [ ] **Step 5: 실사 보정** — 품목 상세에서 "세어보니 n개"
- [ ] **Step 6: `npx tsc --noEmit` + `npm run build` 통과 확인**
- [ ] **Step 7: 커밋** `feat(web): 재고 장부 화면`

---

### Task 24: 실제 기동 검증과 문서

- [ ] **Step 1: 서버·프론트 기동 후 브라우저로 전 과정 확인**
  사입 2건 등록 → 재고 목록 → 같은 이름 다시 등록해서 한 줄로 합쳐지는지 →
  실사 보정 → 데스크톱·모바일 스크린샷
- [ ] **Step 2: 콘솔 에러 0건 확인** — 빌드가 통과해도 런타임에서 터지는 걸 여러 번 봤다
- [ ] **Step 3: `make check` 전체 통과** (fmt + vet + 단위 + 통합)
- [ ] **Step 4: 스펙 §3.1 갱신** — 구현된 내용과 어긋나는 부분 정정
- [ ] **Step 5: 커밋** `docs: 재고 장부 구현 반영`

---

## 이 단계에서 하지 않는 것

- **매장 간 단가 비교** — 카탈로그가 없다. 2단계.
- **마진·수익률** — 매출을 모른다. 추정치는 거짓말이다.
- **재고 자동 차감** — 아무도 매일 안 한다. 가끔 세어보는 실사 보정만 둔다.
- **사입 기록 수정/삭제** — append-only. 틀렸으면 조정을 쌓는다.
- **거래처 관리 화면** — 자유 입력 + 최근 사용 버튼이면 충분하다.
