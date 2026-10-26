-- 웹 푸시 구독.
--
-- 사장님 전용이다. 손님은 앱을 설치하지 않는다 — QR 을 찍고 한 번 쓰고
-- 떠난다 (docs/DESIGN-V2.md §8).
--
-- endpoint 가 사실상의 식별자다. 같은 기기·같은 브라우저가 다시 구독하면
-- 같은 endpoint 가 오므로 UNIQUE 로 묶어 중복 발송을 막는다. 한 사장님이
-- 폰과 PC 를 함께 쓰면 행이 두 개인 게 맞다.
CREATE TABLE push_subscriptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  user_id  uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,

  endpoint text NOT NULL UNIQUE,
  -- 브라우저가 주는 공개키와 인증 비밀. 이게 있어야 본문을 암호화할 수
  -- 있다 (RFC 8291). 우리 비밀이 아니라 그 구독 전용 값이다.
  p256dh text NOT NULL,
  auth   text NOT NULL,

  -- 발송이 404/410 으로 돌아오면 구독이 죽은 것이다. 바로 지우지 않고
  -- 세어 두었다가 지운다 — 일시적인 장애로 멀쩡한 구독을 날리면
  -- 사장님은 알림이 끊긴 걸 한참 뒤에 안다.
  failures int NOT NULL DEFAULT 0,
  last_sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX push_subscriptions_store_idx ON push_subscriptions (store_id);
