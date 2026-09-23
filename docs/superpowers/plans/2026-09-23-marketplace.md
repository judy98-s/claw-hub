# 재고 교환 장터 구현 계획 (커뮤니티 2단계)

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans 로 태스크 단위 구현.

**Goal:** 사장님이 안 나가는 재고를 내놓고, 다른 매장 사장님이 보고 전화한다.

**Why now:** 재고 장부가 끝났다. 품명·수량·원가가 이미 시스템에 있으므로 등록은
**재고에서 한 번 탭**이면 된다. 단가 비교(매장 3곳)나 순위(30곳)와 달리 장터는
**매장 5곳이면 이미 돌아간다** — 그래서 다음 차례다.

**Spec:** `docs/superpowers/specs/2026-09-22-community-spec.md` §4

**선행 작업:** 매장 지역과 사업자등록번호가 없으면 장터가 성립하지 않는다.
Task 25에서 함께 넣는다.

---

## Global Constraints

기존 두 계획의 Global Constraints를 전부 상속한다. 이 단계에만 해당하는 것:

- **우리는 거래 당사자가 아니다.** 결제·에스크로·채팅을 만들지 않는다. 직거래다.
  화면 어디에도 "안전 거래" 같은 말을 쓰지 않는다 — 지킬 수 없는 약속이다.
- **전화번호는 HTML에 미리 들어가지 않는다.** 목록·상세 응답 어디에도 없고,
  버튼을 눌러 별도 요청을 해야 서버가 준다. 이게 봇 수집과 사람 사이의
  실질적인 경계선이다.
- **번호 열람은 전부 기록한다.** 누가, 어느 글의, 언제. 환불 건의 계좌 열람과
  같은 원칙이다.
- **게시 조건: 사업자등록번호 + 지역.** 둘 중 하나라도 없으면 글을 쓸 수 없다.
- **게시 시점의 지역을 글에 박아둔다.** 매장이 이사해도 옛 글이 따라 움직이면 안 된다.
- **30일이 지난 글은 목록에서 사라진다.** 죽은 글이 쌓인 장터는 안 여는 장터다.

## 결정: 사업자등록번호를 어떻게 확인하나

국세청 사업자등록 상태조회 API는 공공데이터포털에 **무료로 있다**
(1일 100만건). 다만 **활용신청과 승인이 필요하다**(3시간~3일). 매장 계정으로
신청해야 하는 것이라 지금 붙일 수 없다.

**1단계: 체크섬 검증만 한다.** 사업자등록번호 10자리에는 검증 규칙이 있어서,
오타와 아무렇게나 적은 숫자를 외부 호출 없이 걸러낸다. 실제 존재하는 번호인지는
확인하지 못한다 — 그 사실을 화면에 숨기지 않는다.

**확장 지점:** `bizverify.Verifier` 인터페이스를 두고 1단계는 `Checksum` 으로
구현한다. 키를 받으면 `NTS` 를 **파일 하나 추가**해서 갈아끼운다. 도메인 코드는
건드리지 않는다. (`payout.Payout` 과 같은 구조다.)

> 진위확인(대표자명·개업일자까지 필요) 말고 **상태조회**(번호만)를 쓴다.
> 우리가 알고 싶은 건 "실존하며 폐업하지 않았나"이고, 그건 번호만으로 나온다.

## 결정: 지역을 어떻게 나누나

**필터는 시·도(17개), 표시는 시·군·구(자유 입력)** 로 간다.

시·군·구까지 코드로 나누면 250개를 손으로 넣어야 하고, 매장이 스무 곳일 때
"강남구" 필터는 늘 0건이다. 필터가 쓸모를 가지려면 그 칸에 여러 매장이 있어야 한다.
시·군·구는 목록에 글자로 보여주기만 한다 — 거리를 가늠하는 데는 그걸로 충분하다.

코드는 행정표준코드(11, 26…) 대신 `seoul`, `gyeonggi` 같은 슬러그를 쓴다.
행정표준코드는 실제로 바뀐다 — 강원과 전북이 특별자치도가 되면서 번호가 옮겨갔다.
우리 DB가 그 변경을 따라다닐 이유가 없다.

## Review Focus

1. **자기 글의 번호 열람** — 자기 매장 글에 전화 버튼이 뜨면 안 된다.
   뜨더라도 기록이 남으면 감사 로그가 쓰레기가 된다. (Task 27)
2. **번호 수집** — 한 계정이 목록을 훑으며 전부 눌러 번호를 긁는 경우.
   매장당 하루 상한을 둔다. 상한에 걸려도 글은 계속 보여야 한다. (Task 27)
3. **만료 경계** — 29일 23시간 된 글과 30일 1분 된 글. 만료된 글의 상세·연락도
   막히는가. (Task 26, Task 27)
4. **교환 전용 글의 가격** — 가격이 0인 것과 "가격을 안 적은 것"은 다르다.
   판매인데 0원이면 거부, 교환이면 가격 칸 자체가 없다. (Task 25)
5. **재고보다 많이 내놓기** — 42개 있는데 100개를 올리는 경우. 막을 것인가.
   (막지 않는다. 곧 들어올 물량을 미리 올리는 건 정상이다. 대신 화면에 보유
   수량을 같이 띄워 사장님이 스스로 알게 한다.) (Task 25, Task 28)
6. **매장 인증 없이 게시** — 사업자등록번호나 지역이 비었을 때. 409 로 막고
   설정 화면을 가리킨다. (Task 27)
7. **같은 매장이 같은 글을 여러 번 신고** — 한 매장 한 번으로 제한하지 않으면
   한 명이 3회를 눌러 남의 글을 내릴 수 있다. (Task 26)

---

### Task 25: 도메인 — 사업자등록번호·지역·거래글 규칙

**Files:**
- Create: `internal/domain/listing.go`, `internal/domain/listing_test.go`,
  `internal/domain/region.go`, `internal/domain/region_test.go`
- Create: `internal/bizverify/bizverify.go`, `internal/bizverify/bizverify_test.go`

**Interfaces:**
```go
// 사업자등록번호 10자리의 검증 규칙. 외부 호출 없이 오타를 거른다.
func ValidateBizNo(raw string) error
func NormalizeBizNo(raw string) string   // 하이픈 제거
func FormatBizNo(raw string) string      // 000-00-00000

type ListingKind string // sell | swap | both
func ParseListingKind(s string) (ListingKind, error)
func (k ListingKind) Label() string
func (k ListingKind) NeedsPrice() bool   // 교환 전용이면 가격이 없다

type ListingInput struct {
    Name string; Kind ListingKind
    Qty int; UnitPriceKRW int; Note string
}
func ValidateListing(in ListingInput) error

// 지역
type Region struct{ Code, Name string }
func Regions() []Region
func RegionByCode(code string) (Region, bool)
func ValidateRegionDetail(s string) error
```

- [ ] **Step 1: 실패하는 테스트**
  - `ValidateBizNo`: 실제 유효한 번호 두 개(220-81-62517, 120-81-47521)가 통과.
    마지막 자리를 하나 바꾸면 전부 거부. 9자리·11자리·문자 포함 거부.
    하이픈이 있든 없든 같은 결과.
  - `ParseListingKind`: sell/swap/both 통과, 그 외 한국어 에러.
  - `NeedsPrice`: swap 만 false.
  - `ValidateListing`: 판매인데 0원 거부, 교환인데 가격 있으면 무시(0으로),
    수량 0·음수 거부, 한마디 300자 초과 거부, 이름 빈 값 거부.
  - `Regions()` 17개, 코드 중복 없음, `RegionByCode("seoul")` 성공.
  - `ValidateRegionDetail`: 빈 값 거부, 20자 초과 거부.
- [ ] **Step 2: 테스트 실패 확인**
- [ ] **Step 3: 최소 구현** — 체크섬 가중치 `[1,3,7,1,3,7,1,3,5]`,
  9번째 자리의 기여분에 `*5/10` 을 더한 뒤 `(10 - sum%10) % 10`.
- [ ] **Step 4: 테스트 통과 확인**
- [ ] **Step 5: `bizverify.Verifier` 인터페이스 + `Checksum` 구현**
- [ ] **Step 6: `arch_test.go` 통과 확인** (domain 격리)
- [ ] **Step 7: 커밋** `feat(domain): 사업자등록번호 검증과 거래글 규칙`

---

### Task 26: 스키마와 저장소

**Files:**
- Create: `internal/store/migrations/0006_marketplace.sql`, `internal/store/listing.go`
- Modify: `internal/store/user.go` (매장 설정에 지역·사업자번호)

**스키마:**
```sql
ALTER TABLE stores ADD COLUMN region_code text NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN region_detail text NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN biz_no text NOT NULL DEFAULT '';

CREATE TABLE listings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  created_by uuid REFERENCES users(id),
  name text NOT NULL,
  name_key text NOT NULL,
  kind text NOT NULL,
  qty int NOT NULL CHECK (qty > 0),
  unit_price_krw int NOT NULL DEFAULT 0 CHECK (unit_price_krw >= 0),
  note text NOT NULL DEFAULT '',
  -- 게시 시점의 지역을 박아둔다. 매장이 이사해도 옛 글은 그 자리에 남는다.
  region_code text NOT NULL,
  region_detail text NOT NULL,
  status text NOT NULL DEFAULT 'open',
  report_count int NOT NULL DEFAULT 0,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz
);
CREATE INDEX listings_open_idx ON listings (status, region_code, created_at DESC);

-- 누가 누구 번호를 열어봤나. 환불 건의 계좌 열람과 같은 원칙이다.
CREATE TABLE listing_contacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  viewer_store_id uuid NOT NULL REFERENCES stores(id),
  viewer_user_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX listing_contacts_viewer_idx ON listing_contacts (viewer_store_id, created_at DESC);

CREATE TABLE listing_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  reporter_store_id uuid NOT NULL REFERENCES stores(id),
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  -- 한 매장 한 번. 없으면 한 명이 3회를 눌러 남의 글을 내릴 수 있다.
  UNIQUE (listing_id, reporter_store_id)
);
```

- [ ] **Step 1: 실패하는 테스트** (통합)
  - 등록 후 목록에 뜬다. 지역·거래방식 필터가 듣는다.
  - **목록·상세 어디에도 전화번호가 없다** (구조체에 칸 자체가 없어야 한다)
  - 만료된 글은 목록에 없고 상세도 `ErrNotFound`
  - 내린 글(closed)도 마찬가지
  - 같은 매장이 두 번 신고하면 카운트가 1이다
  - 세 매장이 신고하면 글이 자동으로 내려간다
  - 번호 열람이 기록된다. 같은 사람이 두 번 봐도 두 줄이 남는다
  - 매장당 일일 열람 수를 셀 수 있다
- [ ] **Step 2~4: 구현과 통과 확인**
- [ ] **Step 5: 커밋** `feat(store): 장터 글과 연락 기록`

---

### Task 27: HTTP API

**Files:**
- Create: `internal/httpapi/admin_market.go`, `internal/httpapi/admin_market_test.go`
- Modify: `internal/httpapi/server.go`, `admin_settings.go`, `fake_test.go`

**Interfaces:**
```
GET    /api/admin/market                   목록 (지역·거래방식 필터)
POST   /api/admin/market                   글 올리기
GET    /api/admin/market/{id}              상세
POST   /api/admin/market/{id}/contact      번호 받기 (기록 남음)
POST   /api/admin/market/{id}/close        내 글 내리기
POST   /api/admin/market/{id}/report       신고
GET    /api/admin/market/mine              내 글
GET    /api/admin/regions                  시·도 목록
PATCH  /api/admin/store                    (확장) 지역·사업자등록번호
```

- [ ] **Step 1: 실패하는 테스트**
  - 사업자번호·지역 없이 글쓰기 → **409**, 메시지가 설정 화면을 가리킨다
  - 목록 응답에 `phone` 키가 **아예 없다**
  - `contact` 를 눌러야 번호가 온다. 그때 기록이 남는다
  - **자기 글에는 `contact` 가 403** — 자기 번호를 자기가 열람할 이유가 없고,
    그 기록이 남으면 감사 로그가 쓰레기가 된다
  - 하루 상한을 넘기면 `429`. **그래도 목록은 계속 보인다**
  - 남의 글을 내리려 하면 403
  - 만료·삭제된 글의 상세·연락은 404
  - 미인증 전 경로 401
- [ ] **Step 2~4: 구현과 통과 확인**
- [ ] **Step 5: 커밋** `feat(api): 장터`

---

### Task 28: 프론트 — 장터 화면

**Files:**
- Create: `web/app/admin/(shell)/market/page.tsx`,
  `web/app/admin/(shell)/market/listing-sheet.tsx`
- Modify: `nav-sidebar.tsx`, `inventory/page.tsx` (재고에서 내놓기),
  `settings/page.tsx` (지역·사업자등록번호)

- [ ] **Step 1: 설정에 지역·사업자등록번호** — 입력하는 동안 체크섬 통과 여부를
  보여준다. "실제 존재하는 번호인지는 확인하지 않습니다"를 숨기지 않는다.
- [ ] **Step 2: 장터 목록** — 지역·거래방식 필터. 전화 버튼은 누르기 전까지
  번호가 없다. 하단에 "claw-hub는 거래 당사자가 아닙니다" 고정
- [ ] **Step 3: 재고에서 내놓기** — 재고 상세 시트에 `[장터에 내놓기]`.
  보유 수량과 내 원가를 같이 띄운다 (원가보다 싸게 내놓는 걸 막지는 않되,
  모르고 그러는 일은 없게)
- [ ] **Step 4: 내 글 관리** — 내리기, 신고 확인
- [ ] **Step 5: 빌드·타입체크 통과**
- [ ] **Step 6: 커밋** `feat(web): 장터 화면`

---

### Task 29: 실제 기동 검증

- [ ] **Step 1: 매장 두 곳을 만들어 서로의 글을 보는 시나리오** — 이게 이
  기능의 전부다. 한 매장만으로는 아무것도 검증되지 않는다
- [ ] **Step 2: 번호 열람 기록이 실제로 쌓이는지 DB에서 확인**
- [ ] **Step 3: 데스크톱·모바일 스크린샷, 콘솔 에러 0건**
- [ ] **Step 4: `make check` + 통합 테스트 전체 통과**
- [ ] **Step 5: 커밋** `docs: 장터 구현 반영`

---

## 이 단계에서 하지 않는 것

- **결제·에스크로** — 직거래다. 돈에 개입하면 분쟁 처리 조직이 필요해진다.
- **채팅** — 전화가 더 빠르고, 만들면 운영·신고·차단이 딸려온다.
- **안심번호(050)** — 유료 회선 계약이 필요하다. 번호 노출을 버튼 뒤로 숨기고
  열람을 기록하는 것으로 1단계를 가름한다.
- **시·군·구 단위 필터** — 매장이 그만큼 모이기 전에는 늘 0건이다.
- **배송·택배 연동** — 인형 상자는 직거래가 기본이다.
