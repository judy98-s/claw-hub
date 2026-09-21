-- claw-hub 초기 스키마
--
-- 개인정보(전화번호·계좌번호·예금주)는 *_enc 컬럼에 AES-GCM으로 암호화해 저장하고,
-- 조회와 카운트는 *_hash (HMAC-SHA256) 컬럼으로만 한다. 평문 컬럼은 없다.

CREATE TABLE stores (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    phone      text,                    -- 매장 대표번호. 손님 에러 화면에 안내한다
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id      uuid        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    email         text        NOT NULL UNIQUE,
    password_hash text        NOT NULL,
    name          text        NOT NULL DEFAULT '',
    -- 무차별 대입 방어. 실패가 쌓이면 locked_until 이 설정된다.
    failed_logins int         NOT NULL DEFAULT 0,
    locked_until  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE machines (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   uuid        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    -- QR에 인코딩되는 공개 코드. 전역 유니크여야 QR 하나가 기계 하나를 가리킨다.
    code       text        NOT NULL UNIQUE,
    label      text        NOT NULL,
    location   text        NOT NULL DEFAULT '',
    active     boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX machines_store_idx ON machines (store_id, active);

CREATE TABLE claims (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines(id) ON DELETE RESTRICT,

    issue_type  text NOT NULL,
    -- 0원 이하는 접수 자체가 성립하지 않는다. 애플리케이션 검증과 이중으로 막는다.
    amount_krw  int  NOT NULL CHECK (amount_krw > 0),
    description text NOT NULL DEFAULT '',

    -- 개인정보: 암호문과 조회용 해시만 저장한다
    phone_enc    bytea NOT NULL,
    phone_hash   bytea NOT NULL,
    bank_code    text  NOT NULL,
    account_enc  bytea NOT NULL,
    account_hash bytea NOT NULL,
    holder_enc   bytea NOT NULL,

    status       text  NOT NULL,
    risk_score   int   NOT NULL DEFAULT 0,
    risk_reasons jsonb NOT NULL DEFAULT '[]'::jsonb,

    -- 이중 환불의 최후 방어선.
    -- 캐시 기반 멱등성은 Redis 장애나 경합에서 뚫릴 수 있다. DB 제약은 안 뚫린다.
    idempotency_key text NOT NULL,

    created_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    resolved_by uuid REFERENCES users(id),
    paid_at     timestamptz,
    payout_method text NOT NULL DEFAULT '',

    ip_hash    bytea,
    user_agent text NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX claims_idempotency_uniq ON claims (store_id, idempotency_key);

-- 대시보드 기본 조회
CREATE INDEX claims_store_status_idx ON claims (store_id, status, created_at DESC);
-- 반복 신고 카운트
CREATE INDEX claims_phone_idx ON claims (phone_hash, created_at DESC);
-- 계좌 공유 탐지
CREATE INDEX claims_account_idx ON claims (account_hash, created_at DESC);
-- 기계별 리포트
CREATE INDEX claims_machine_idx ON claims (machine_id, created_at DESC);

CREATE TABLE claim_photos (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    claim_id     uuid        NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    object_key   text        NOT NULL,
    content_type text        NOT NULL,
    size_bytes   bigint      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    -- 보존 기간이 지나면 워커가 파일과 이 행을 함께 지운다
    expires_at   timestamptz NOT NULL
);

CREATE INDEX claim_photos_claim_idx   ON claim_photos (claim_id);
CREATE INDEX claim_photos_expires_idx ON claim_photos (expires_at);

-- 감사 로그. append-only 로만 쓴다 (UPDATE/DELETE 금지, 코드 규약).
-- 돈이 오가므로 누가 언제 무엇을 했는지가 남아야 한다.
CREATE TABLE claim_events (
    id          bigserial PRIMARY KEY,
    claim_id    uuid        NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    actor_kind  text        NOT NULL,
    actor_id    text        NOT NULL DEFAULT '',
    action      text        NOT NULL,
    from_status text        NOT NULL DEFAULT '',
    to_status   text        NOT NULL DEFAULT '',
    note        text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX claim_events_claim_idx ON claim_events (claim_id, created_at);

-- 사장님이 직접 지정한 연락처 상태.
-- 자동 카운트는 claims 에서 실시간 집계하므로 여기 저장하지 않는다.
-- 카운터를 따로 두면 정합성이 깨질 여지만 생긴다.
CREATE TABLE contact_flags (
    identity_hash bytea       NOT NULL,
    identity_type text        NOT NULL,   -- 'phone' | 'account'
    store_id      uuid        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    manual_status text        NOT NULL,   -- 'normal' | 'watch' | 'blocked'
    note          text        NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL DEFAULT now(),
    updated_by    uuid REFERENCES users(id),
    PRIMARY KEY (identity_hash, identity_type, store_id)
);
