-- 재고 장부.
--
-- 두 테이블 다 append-only 다. 사입 기록을 고치는 대신 조정을 쌓는다.
-- 장부는 고쳐 쓰는 게 아니라 이어 쓰는 것이다.
--
-- 현재 재고 수량 테이블을 따로 두지 않는다. purchases 합계에서
-- inventory_adjustments 를 반영해 그때그때 계산한다. 이 데이터 규모에서
-- 인덱스 스캔은 밀리초이고, 카운터를 두면 정합성이 깨질 여지만 생긴다.

CREATE TABLE purchases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  created_by uuid REFERENCES users(id),

  -- doll_id 는 1단계에서 늘 NULL 이다. 전국 카탈로그가 생기면 채운다.
  -- NULL 인 행은 매장 간 집계에 들어가지 않는다 — 이 규칙이 코드에
  -- 박혀 있어야 "일단 대충 세자"가 나중에 스며들지 않는다.
  doll_id uuid,

  -- name 은 사장님이 적은 그대로. 화면 표기이자, 나중에 카탈로그를
  -- 만들 때 쓸 "실제로 쓰이는 이름" 표본이다.
  -- name_key 는 domain.NameKey(name). 한 매장 안에서 묶는 기준이다.
  name text NOT NULL,
  name_key text NOT NULL,

  vendor text NOT NULL DEFAULT '',
  unit_price_krw int NOT NULL CHECK (unit_price_krw >= 0),
  qty int NOT NULL CHECK (qty > 0),
  shipping_krw int NOT NULL DEFAULT 0 CHECK (shipping_krw >= 0),
  purchased_at timestamptz NOT NULL,

  -- 영수증은 자동 삭제하지 않는다. 신고 사진은 개인정보라 보존 기간이
  -- 지나면 지우지만, 영수증은 사장님 장부의 증빙이다. 성격이 반대다.
  receipt_key text NOT NULL DEFAULT '',

  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),

  -- 더블탭 방어선. 제출 버튼을 두 번 누르면 60개가 120개가 된다.
  -- 캐시가 아니라 여기가 실제 방어선이다.
  UNIQUE (store_id, idempotency_key)
);

CREATE INDEX purchases_store_key_idx ON purchases (store_id, name_key, purchased_at DESC);
CREATE INDEX purchases_store_date_idx ON purchases (store_id, purchased_at DESC);

CREATE TABLE inventory_adjustments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id),
  created_by uuid REFERENCES users(id),
  name_key text NOT NULL,

  -- delta = 세어본 수량 - 그때까지 계산된 수량.
  -- 보통 음수다 (팔려나갔으니까). 양수면 입고를 빠뜨렸다는 뜻이다.
  delta int NOT NULL,
  counted_qty int NOT NULL CHECK (counted_qty >= 0),

  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX inventory_adj_store_key_idx ON inventory_adjustments (store_id, name_key);
