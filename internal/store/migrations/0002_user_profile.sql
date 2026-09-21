-- 사장님 계정에 연락처를 둔다.
--
-- 직원이 늘면 "누구한테 전화하지"가 생긴다. 매장 대표번호(stores.phone)는
-- 손님에게 안내하는 번호라 역할이 다르다.
ALTER TABLE users ADD COLUMN phone text NOT NULL DEFAULT '';

-- 계정 비활성화. 그만둔 직원의 계정은 지우지 않고 끈다.
-- 지우면 claim_events.resolved_by 가 가리키던 사람이 사라져서
-- "누가 승인했나"를 나중에 따질 수 없다.
ALTER TABLE users ADD COLUMN active boolean NOT NULL DEFAULT true;

CREATE INDEX users_store_idx ON users (store_id, active);
