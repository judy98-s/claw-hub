-- 매장별 송금 설정.
--
-- 지금까지는 PAYOUT_DEEPLINK_TEMPLATES 환경변수로만 정할 수 있었다.
-- 사장님이 앱을 바꾸려고 서버 설정을 고치고 재기동해야 하는 건 말이 안 된다.
ALTER TABLE stores ADD COLUMN payout_provider text NOT NULL DEFAULT '';
-- 직접 입력한 딥링크 템플릿. provider 가 'custom' 일 때만 쓴다.
ALTER TABLE stores ADD COLUMN payout_template text NOT NULL DEFAULT '';

-- 사장님이 돈을 보내는 계좌. 기록용이다.
--
-- 딥링크에 넣지 않는다 — 송금 앱은 어느 계좌에서 보낼지를 URL 로 받지
-- 않고, 앱에 로그인한 사람의 주계좌를 쓴다. 직원이 여러 명일 때
-- "어느 계좌에서 나가야 하는지" 를 화면에 띄워주기 위한 값이다.
ALTER TABLE stores ADD COLUMN payout_bank_code text NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN payout_account_enc bytea;

-- 실제로 보낸 금액. 손님이 요청한 금액과 다를 수 있다.
--
-- 부분 환불이 실제로 일어난다 — 3천원 요청인데 확인해보니 2천원만
-- 먹힌 경우 같은 것. 요청액(amount_krw)을 덮어쓰면 "얼마를 요청했는지"가
-- 사라져서 나중에 분쟁이 나면 근거가 없다.
ALTER TABLE claims ADD COLUMN paid_amount_krw int CHECK (paid_amount_krw IS NULL OR paid_amount_krw > 0);
