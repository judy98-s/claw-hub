-- 재고 교환 장터.
--
-- 우리는 거래 당사자가 아니다. 결제도 채팅도 없고, 사장님끼리 전화해서
-- 직거래한다. 그래서 이 스키마가 할 일은 셋뿐이다: 글을 보관하고,
-- 번호를 누가 봤는지 남기고, 신고를 세는 것.

-- 매장 설정 확장. 장터에 글을 쓰려면 둘 다 있어야 한다.
ALTER TABLE stores ADD COLUMN region_code text NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN region_detail text NOT NULL DEFAULT '';

-- 사업자등록번호는 평문으로 둔다. 개인정보가 아니라 공개된 사업자 식별자이고,
-- 거래 상대가 확인할 수 있어야 하는 값이다. 전화번호·계좌와 성격이 다르다.
ALTER TABLE stores ADD COLUMN biz_no text NOT NULL DEFAULT '';

CREATE TABLE listings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  created_by uuid REFERENCES users(id),

  name text NOT NULL,
  name_key text NOT NULL,
  kind text NOT NULL,
  qty int NOT NULL CHECK (qty > 0),
  unit_price_krw int NOT NULL DEFAULT 0 CHECK (unit_price_krw >= 0),
  note text NOT NULL DEFAULT '',

  -- 게시 시점의 지역을 글에 박아둔다. 매장이 이사해도 옛 글이 따라
  -- 움직이면 안 되고, 목록을 그릴 때 매번 조인할 이유도 없다.
  region_code text NOT NULL,
  region_detail text NOT NULL,

  status text NOT NULL DEFAULT 'open',  -- open | closed | removed
  report_count int NOT NULL DEFAULT 0,

  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz
);

CREATE INDEX listings_open_idx ON listings (status, expires_at, created_at DESC);
CREATE INDEX listings_store_idx ON listings (store_id, created_at DESC);

-- 누가 누구 번호를 열어봤나. 환불 건의 계좌 열람과 같은 원칙이다.
--
-- 같은 사람이 두 번 봐도 두 줄이 남는다. 횟수 자체가 신호다 —
-- 한 계정이 하루에 스무 명 번호를 열었다면 그건 거래가 아니다.
CREATE TABLE listing_contacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  viewer_store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  viewer_user_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX listing_contacts_viewer_idx ON listing_contacts (viewer_store_id, created_at DESC);

CREATE TABLE listing_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  reporter_store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),

  -- 한 매장 한 번. 이게 없으면 한 명이 세 번 눌러 남의 글을 내릴 수 있다.
  UNIQUE (listing_id, reporter_store_id)
);
