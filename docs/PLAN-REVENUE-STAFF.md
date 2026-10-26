# 매출과 직원관리 — 구현 계획

무인매장 사장님이 "이 기계가 돈을 버는지", "직원이 왔는지"를 알게 만든다.

## 지금 있는 것

빈 땅이 아니다. 이미 깔린 것부터 센다.

| 축 | 상태 | 테이블 |
|---|---|---|
| 민원·환불 | 있다 | `claims`, `claim_events`, `claim_photos` |
| 기계 | 있다 | `machines` |
| 원가 (사입) | 있다 | `purchases` — 단가, 배송비, 영수증 |
| 소진 (실사) | 있다 | `inventory_adjustments` — delta, counted_qty |
| 직원 목록 | 반쯤 | `users` — active 토글만. 권한·근태 없음 |
| 중고 장터 | 있다 | `listings` |
| **매출** | **없다** | — |

**손익의 세 변 중 두 변이 이미 있다.** 원가(사입)와 유출(환불)은 쌓이는데
유입(매출)이 없어서 숫자가 닫히지 않는다. 매출 한 축을 붙이면 "이 기계가
한 달에 얼마 남겼나"가 처음으로 계산된다. 거기가 이 제품의 쐐기다.

## 쐐기를 어디에 박을지

매출 집계만 놓고 보면 캐시노트가 압도적이다. 정면으로 가면 진다.

하지만 캐시노트는 **카드 정산 내역**을 본다. 그래서 기계별로 쪼갤 수 없다.
"3번 기계가 현금을 먹는다", "2번은 인형값보다 덜 번다" 는 질문에 답하지
못한다. 무인매장 사장님이 실제로 돈을 잃는 자리가 정확히 거기다.

그래서 우리의 1번 지표는 매장 월매출이 아니라 **기계별 순익**이다.
매장 합계는 그걸 계산하다 보니 따라 나오는 부산물로 둔다.

---

## Phase 1 — 매출 입력 (1~2주)

자동 연동부터 하면 아무것도 못 내놓는다. 수동 입력이 Day 1 기능이다.

### 스키마

```sql
CREATE TABLE revenues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  -- NULL 이면 매장 전체 매출. 무인세탁·코인노래방은 기계별로 안 쪼갠다.
  machine_id uuid REFERENCES machines(id),

  -- timestamptz 가 아니라 date 다. "그날 매출" 은 날짜 단위 개념이고,
  -- 뽑기방은 새벽까지 영업해서 자정이 영업일 경계가 아니다.
  business_date date NOT NULL,

  source text NOT NULL,  -- manual | counter | card_api
  cash_krw int NOT NULL DEFAULT 0 CHECK (cash_krw >= 0),
  card_krw int NOT NULL DEFAULT 0 CHECK (card_krw >= 0),
  -- 뽑기 횟수. 코인 카운터를 읽어 적는다. 매출 ÷ 1회 요금으로는
  -- 역산하지 않는다 — 무료 서비스 플레이와 요금 변경이 섞여 틀린다.
  plays int CHECK (plays IS NULL OR plays >= 0),

  note text NOT NULL DEFAULT '',
  created_by uuid REFERENCES users(id),
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (store_id, idempotency_key)
);
```

### 결정 세 가지

**1. append-only. 고치지 않고 이어 쓴다.**
`purchases`·`inventory_adjustments` 가 이미 그렇다. 같은 (기계, 영업일)에
행이 여러 개면 **가장 최근 행이 현재 값**이다.

```sql
SELECT DISTINCT ON (machine_id, business_date) *
  FROM revenues WHERE store_id = $1
 ORDER BY machine_id, business_date, created_at DESC
```

왜: 사장님이 "어제 30만이라 적었는데 35만이었다" 를 고치면, UPDATE 는
틀린 기록을 지운다. 세무조사나 직원과의 분쟁에서 필요한 건 **고쳐진
값이 아니라 고쳤다는 사실**이다. 장부는 고쳐 쓰는 게 아니라 이어 쓴다.

**2. 영업일 경계를 매장 설정으로 둔다.**
`stores.business_day_start_hour int NOT NULL DEFAULT 6`.
새벽 2시 매출이 전날에 들어가야 한다. 이걸 안 정하면 월말 숫자가 항상
하루치 틀리고, 원인을 못 찾는다.

**3. `plays` 를 매출에서 역산하지 않는다.**
무료 플레이, 요금 변경, 서비스 지급이 섞여서 반드시 틀린다. 카운터를
안 읽었으면 NULL 로 둔다. 모르는 걸 추정한 숫자로 채우면, 사장님이
그 숫자를 믿고 기계를 뺀다.

### 화면

- `/admin/revenue` — 날짜 선택, 기계별 입력 줄, 합계. 현금/카드 분리.
- 홈 상단: 오늘·이번주·이번달 매출. 미입력일 배지.
- 입력 안 한 날이 3일 이상이면 홈에 알림. **독촉이 아니라 기본값 제시** —
  "지난 7일 평균으로 채울까요?" 는 하지 않는다. 추정값이 장부에 들어가면
  그 장부는 더 이상 장부가 아니다.

### API

```
GET  /api/admin/revenues?from=&to=     기간 조회 (기계별)
POST /api/admin/revenues               입력/수정 (append)
GET  /api/admin/revenues/missing       미입력 영업일 목록
```

---

## Phase 2 — 기계별 손익 (2~3주) ★ 핵심

### 빠진 연결 고리

`inventory_adjustments` 에 `machine_id` 가 없다. 인형이 몇 개 나갔는지는
아는데 **어느 기계에서** 나갔는지 모른다. 기계별 원가를 못 낸다.

```sql
ALTER TABLE inventory_adjustments ADD COLUMN machine_id uuid REFERENCES machines(id);
```

nullable 로 둔다. 기계별로 세는 사장님은 기계별 손익을 보고, 매장 단위로
세는 사장님은 매장 손익만 본다. 전부 다 세라고 강요하면 아무도 안 센다.

### 계산

```
기계 순익 = 매출(revenues)
          − 원가(소진 수량 × 이동평균 단가)
          − 환불(approved claims)
```

**고정비(임대료·전기·통신)는 기계별로 배분하지 않는다.** 배분 기준(면적?
대수? 매출 비례?)을 정하는 순간 숫자가 거짓말을 시작하고, 사장님은 그
거짓말로 기계를 뺀다. 매장 단위로만 보여주고, 기계 순익은 고정비 전(前)
기준임을 화면에 명시한다.

단가는 **이동평균**을 쓴다. FIFO 가 회계적으로 맞지만, 사입 기록이 비는
날이 있는 현실에서 FIFO 는 음수 재고를 만든다. 이동평균은 틀려도 부드럽게
틀린다.

### 화면

- `/admin/profit` — 기계별 카드: 매출 / 원가 / 환불 / 순익 / 회수율
- 기계 상세: 월별 추이, 손익분기까지 남은 플레이 수
- **"뺄까요?" 신호** — 3개월 연속 순익 음수인 기계를 표시. 이게 사장님이
  돈으로 체감하는 유일한 출력이고, 입소문이 나는 지점이다.

---

## Phase 3 — 직원 권한 (1주) ※ Phase 4 의 선행 조건

`internal/store/user.go` 에 이렇게 적혀 있다.

> 권한 구분을 두지 않았다. 한 매장에 두세 명이 쓰는 시스템에서 역할
> 체계는 관리 비용만 늘린다. 필요해지면 그때 넣는다.

**지금이 그때다.** 이유는 하나뿐이다 — 사장님은 아르바이트에게 매출을
보여주고 싶지 않다. 매출을 붙이는 순간 권한이 필수가 된다.

```sql
ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'owner';
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('owner','manager','staff'));
```

| | staff | manager | owner |
|---|---|---|---|
| 민원 보기·처리 | O | O | O |
| 재고 실사·보충 | O | O | O |
| 출퇴근 | O | O | O |
| **매출·손익** | **X** | 보기만 | O |
| 계좌·송금 설정 | X | X | O |
| 직원 추가·급여 | X | X | O |

기존 계정은 전부 `owner` 로 마이그레이션한다. DEFAULT 를 `staff` 로 두면
배포 순간 사장님이 자기 매출을 못 본다.

**서버에서 막는다.** 화면에서 메뉴만 숨기는 건 권한이 아니다.
`s.authed` 옆에 `s.requireRole(domain.RoleOwner)` 를 둔다.

---

## Phase 4 — 근태와 업무 (3~4주)

무인매장 직원은 상주하지 않는다. **순회**한다 — 청소, 인형 보충, 현금통
회수, 쓰레기. 그래서 필요한 건 근태 기록이 아니라 "돌았는지"다.

```sql
CREATE TABLE shifts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  user_id uuid NOT NULL REFERENCES users(id),
  checked_in_at  timestamptz NOT NULL,
  checked_out_at timestamptz,
  method text NOT NULL,  -- qr | manual
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE task_templates (   -- 매장별 순회 체크리스트 정의
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  label text NOT NULL,
  needs_photo boolean NOT NULL DEFAULT false,
  sort_order int NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true
);

CREATE TABLE task_runs (        -- 실제 수행 기록 (append-only)
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  template_id uuid NOT NULL REFERENCES task_templates(id),
  shift_id uuid REFERENCES shifts(id),
  user_id uuid NOT NULL REFERENCES users(id),
  done_at timestamptz NOT NULL DEFAULT now(),
  photo_key text NOT NULL DEFAULT '',
  note text NOT NULL DEFAULT ''
);
```

### GPS 대신 QR

매장에 붙인 QR 을 찍어 출퇴근한다. 기계 QR 인쇄 기능이 이미 있으니
(`/api/admin/machines/{id}/qr.png`) 같은 길을 쓴다.

GPS 를 쓰지 않는 이유:
- **위치정보법** — 개인위치정보 수집·이용에 별도 동의가 필요하고,
  동의 철회·파기 절차까지 갖춰야 한다. QR 은 그 범주에 안 들어간다.
- 실내·지하 정확도가 나쁘다. "출근했는데 안 찍힌다" 분쟁이 매일 생긴다.
- 배터리. 직원이 앱 권한을 끈다.

QR 의 약점은 사진 공유로 원격 출근이 가능한 것이다. QR 에 **시간당
회전하는 토큰**을 넣어 막는다. 완벽하지 않지만, 분쟁 비용이 GPS 보다 훨씬
싸다.

### 사진 첨부

`internal/media` 와 민원 사진 파이프라인을 그대로 쓴다. 다만 보존 정책이
반대다 — 민원 사진은 개인정보라 기간 후 삭제하지만, 청소 인증 사진은
직원과의 분쟁 증빙이다. `purchases.receipt_key` 와 같은 성격으로 둔다.

---

## Phase 5 — 급여 보조 (2주). 지급은 하지 않는다

### 절대 넘지 않는 선

**우리는 돈을 쥐지 않는다.**

지금 환불은 사장님이 자기 계좌에서 직접 송금한다. 그래서 이 서비스는
PG 도 전자금융업자도 아니다. 급여도 똑같이 간다 — 계산하고 명세서를
만들어 주되, 이체는 사장님이 자기 뱅킹에서 한다.

우리가 돈을 보관하거나 이전하는 순간 **선불전자지급수단 발행·관리업 또는
자금이체업 등록** 대상이 되고, 자본금·전산 요건이 개인사업자가 감당할
수준을 넘는다. 이 선을 넘는 기능 요청은 전부 거절한다.

### 계산해 줄 것

- 근무시간 집계 (`shifts` 에서)
- **주휴수당** — 주 15시간 이상 + 소정근로일 개근 시 발생. 5인 미만
  사업장에도 적용된다. 무인매장 알바가 대부분 주 15시간 근처라서, 이
  경계를 자동으로 알려주는 것만으로 가치가 있다.
- **임금명세서** — 근로기준법상 교부 의무가 사업장 규모와 무관하게 적용된다.
  임금 구성항목·계산방법·공제내역을 담아 PDF 로 만들고 전자 교부한다.
  사장님 대부분이 이걸 안 주고 있다. 가장 체감되는 기능이 될 수 있다.
- 5인 미만이면 **연장·야간·휴일 가산수당은 적용되지 않는다**. 계산기에
  5인 미만 여부를 매장 설정으로 두고, 해당 항목을 아예 숨긴다. 안 그러면
  사장님이 안 줘도 되는 돈을 주거나, 반대로 줘야 할 걸 안 준다.

### 개인정보

주민등록번호는 **원천징수를 실제로 할 때까지 받지 않는다.** 개인정보보호법상
고유식별정보는 법령 근거가 있을 때만 처리할 수 있고, 안 받으면 유출될
일도 없다. 받게 되면 `DATA_ENCRYPTION_KEY` 로 암호화해 저장한다 —
계좌번호·전화번호와 같은 경로다.

> ⚠️ 위 노동법·세법 항목은 제품 설계를 위한 정리다. 실제 계산 로직을
> 넣기 전에 조문 원문(law.go.kr)과 노무사 확인을 한 번 받아야 한다.
> 계산기가 틀리면 사장님이 과태료를 맞는다.

---

## Phase 6 — 자동 매출 연동 (4주+). 마지막에 한다

수동 입력으로 쓰는 사람이 생긴 뒤에 붙인다. 순서를 바꾸면 연동을
만드는 동안 아무도 안 쓰는 제품을 들고 있게 된다.

| 경로 | 쓸만한가 | 메모 |
|---|---|---|
| 여신금융협회 가맹점 매출거래정보 통합조회 | 카드 매출의 표준 조회처 | 사장님이 직접 조회·엑셀 업로드부터 시작 |
| CODEF (쿠콘) | 현실적인 1순위 | 카드 매출·입금·현금영수증 API. 카카오페이 비즈니스가 쓰고 있다 |
| 마이데이터 직접 | **불가** | 본인신용정보관리업은 금융위 허가. 개인사업자 범위 밖 |
| 뽑기 기계 코인 카운터 | 대부분 아날로그 | 수동 입력이 현실. IoT 는 하드웨어 사업이라 별 건 |

**현금이 본질적 한계다.** 뽑기방 매출의 큰 몫이 현금이고, 어떤 API 도
현금은 못 본다. 그래서 수동 입력은 "자동화 전 임시방편"이 아니라 **영구
기능**이다. 그 전제로 UI 를 만든다 — 30초 안에 끝나야 한다.

CODEF 는 가격·계약조건·인증 방식을 직접 확인해야 한다. 검색으로는
공식 자료가 안 나왔다.

---

## Phase 7 — 모바일

`docs/ANDROID.md` 에 적은 경로를 그대로 간다.

1. **PWA + Web Push** — 사장님은 민원 알림이 핵심이라 푸시만 되면 앱이
   거의 필요 없다. iOS 도 홈화면 추가 시 푸시를 받는다.
2. **TWA → Play 스토어** — 스토어에 있다는 신뢰가 B2B 영업에서 쓰인다.
3. **iOS 래퍼** — 마지막. 심사와 개발자 계정 비용이 든다.

직원 앱은 사장님 앱과 **화면이 다르다**. 직원은 출퇴근·체크리스트·민원
처리 세 개만 보면 된다. 같은 앱에 역할로 분기한다 — 앱을 두 개 만들면
스토어 심사와 배포를 두 번 하게 된다.

오프라인이 직원 앱에서는 진짜 요구사항이다. 무인세탁소 지하는 신호가
안 터진다. 출퇴근·체크리스트는 로컬에 쌓고 나중에 동기화한다.

---

## 순서와 이유

```
Phase 1 매출 입력      ─┐
Phase 2 기계별 손익    ─┴─ 여기까지가 "돈이 보인다". 1순위.
Phase 3 권한           ─┐
Phase 4 근태·업무      ─┴─ 여기까지가 "사람이 보인다".
Phase 5 급여 보조      ─── 차별점. 경쟁자가 안 한다.
Phase 6 자동 연동      ─── 쓰는 사람 생긴 뒤.
Phase 7 모바일         ─── Phase 4 와 병행 가능 (직원은 모바일만 쓴다)
```

Phase 1+2 만으로 팔 수 있다. 3~5 는 Phase 2 를 쓰는 사장님이 생긴 뒤에
그 사람들에게 물어보고 정한다. 지금 다 설계하면 안 쓰는 기능을 만든다.

## 먼저 할 일 (Phase 1 착수 전)

- [ ] 서버 증상 코드를 업종별로 확장 (§7). 지금 무인세탁·노래방 증상이
      400 으로 떨어져서 claw 외 업종은 손님 폼이 동작하지 않는다.
      **매출보다 이게 먼저다** — 업종 확장이 막혀 있으면 시장이 뽑기방뿐이다.
- [ ] 손님 폼 9단계 재작성 (DESIGN-V2 §6.1)
- [ ] `GET /api/public/claims/{receiptCode}` 접수 조회
