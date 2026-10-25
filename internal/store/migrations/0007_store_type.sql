-- 매장 업종.
--
-- 손님이 보는 것 거의 전부가 이 값으로 갈린다 — 캐릭터, 증상 목록,
-- 빠른 금액, QR 스티커 문구, 기계를 부르는 말.
--
-- DEFAULT 'claw' 인 이유: 이 컬럼이 생기기 전에 만들어진 매장은 전부
-- 인형뽑기였다. NOT NULL 로 두되 기존 행이 깨지지 않게 기본값을 준다.
ALTER TABLE stores ADD COLUMN store_type text NOT NULL DEFAULT 'claw';

-- 아는 값만 들어가게 막는다. 오타가 들어오면 손님 화면이 통째로
-- 기본값으로 떨어지는데, 그건 조용해서 찾기 어렵다.
ALTER TABLE stores ADD CONSTRAINT stores_store_type_check
  CHECK (store_type IN ('claw','laundry','karaoke','photo','study','cafe'));
