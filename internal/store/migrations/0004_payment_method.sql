-- 손님이 무엇으로 결제했는지.
--
-- 현금이면 계좌로 송금하고, 카드면 사장님이 단말기에서 승인을 취소한다.
-- 처리 방법이 다르므로 접수 단계에서 갈라야 한다.
--
-- 'unknown' 은 이 컬럼이 생기기 전에 들어온 기존 건이다. 전부 현금으로
-- 간주하면 카드 건이 섞여 들어가 잘못된 통계가 된다.
ALTER TABLE claims ADD COLUMN payment_method text NOT NULL DEFAULT 'unknown';

-- 카드 결제 취소에 필요한 단서.
--
-- 사장님이 단말기나 PG 관리자에서 거래를 찾으려면 대략의 결제 시각과
-- 카드 뒷자리가 필요하다. 전체 카드번호는 받지 않는다 — 받을 이유가 없고,
-- 받는 순간 다루기 훨씬 무거운 정보가 된다.
ALTER TABLE claims ADD COLUMN card_last4 text NOT NULL DEFAULT '';
ALTER TABLE claims ADD COLUMN paid_at_guess timestamptz;

CREATE INDEX claims_payment_method_idx ON claims (store_id, payment_method, created_at DESC);
