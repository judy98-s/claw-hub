# claw-hub

인형뽑기 기계의 고장·환불 문의를 접수하고 처리하는 시스템.

기계마다 붙은 QR 스티커를 손님이 찍으면 모바일 웹 신고 폼이 열린다. 손님은
증상·사진·금액·전화번호·계좌를 입력하고 제출한다. 사장님은 Slack 알림을 받고
웹 대시보드에서 승인하며, 송금 버튼으로 환불한다.

## 무엇을 하나

- **QR 신고 접수** — 앱 설치·로그인·친구추가 없이 모바일 웹에서 제출
- **반자동 환불** — 버튼 한 번으로 토스/카카오뱅크 송금 화면이 계좌·금액이 채워진 상태로 열린다
- **체리피커 차단** — 전화번호·계좌 기준 반복 신고를 자동 탐지해 보류하고, 판단 근거를 문장으로 제시
- **기계 이상 탐지** — "3번 기계 오늘 3건"처럼 점검이 필요한 기계를 사장님에게 먼저 알림

## 알아둘 제약: 완전 자동 송금은 못 한다

국내 결제 환경에서 서버가 직접 임의 계좌로 송금하는 것은 1단계에서 불가능하다.

- **카카오페이 API** 는 온라인 *결제*(수납) API 다. 사업자가 개인 계좌로 *송금*하는 공개 API 는 없다.
- **토스페이먼츠** 는 PG(결제대행)다. 송금이 아니다.
- 실제 송금에는 **지급대행 · 펌뱅킹 · 오픈뱅킹 출금이체** 계약이 필요하고, 전부 사업자 심사를 거친다(수 주 소요).

그래서 1단계는 **딥링크 반자동**이다. 사장님이 버튼을 누르면 토스/카카오뱅크 앱이
계좌와 금액이 채워진 송금 화면으로 열리고, 인증 한 번으로 끝난다. 실제 소요는 몇 초다.
딥링크가 없거나 PC 에서 열었으면 계좌 복사와 수동 "송금 완료" 기록으로 떨어진다.

송금 수단은 `internal/payout.Payout` 인터페이스 뒤에 있다. 지급대행 계약이 성사되면
구현체를 하나 추가하면 되고 도메인 코드는 건드리지 않는다.

## 스택

| 영역 | 선택 |
|---|---|
| 백엔드 | Go 1.24 · 표준 `net/http` · pgx |
| 데이터 | PostgreSQL 16 · Redis 7 (없으면 인메모리로 폴백) |
| 프론트 | Next.js 15 · Tailwind v4 · Pretendard · Phosphor Icons |
| 알림 | Slack Webhook (알림 채널은 인터페이스로 추상화) |
| 배포 | 단일 VPS + Docker Compose + Caddy |

## 빠르게 돌려보기

```bash
make dev                      # postgres + redis 기동
cp .env.example .env
make keys                     # 출력된 키를 .env 에 붙여넣기
make api                      # 다른 터미널에서
make web                      # 또 다른 터미널에서

make setup-account STORE="테스트 매장" EMAIL=owner@example.com PASSWORD=secret123
```

`http://localhost:3000/admin/login` 에서 로그인 → 기계 등록 → QR 인쇄.

배포는 [docs/DEPLOY.md](docs/DEPLOY.md).

## 구조

```
cmd/            api · worker · setup 진입점
internal/
  domain/       엔티티와 규칙. 다른 internal 패키지를 import 하지 않는다
  store/        Postgres 리포지토리 (암호화 경계)
  httpapi/      핸들러 · 미들웨어 · 라우팅
  crypto/       AES-GCM 암호화 · HMAC 해시 · 마스킹
  cache/        Cache 인터페이스 + redis/memory
  media/        Storage 인터페이스 + local
  notify/       Notifier 인터페이스 + slack/noop
  payout/       Payout 인터페이스 + deeplink
  worker/       기계 알림 · 일일 요약 · 사진 만료 삭제
web/            Next.js
```

**`internal/domain` 은 다른 내부 패키지를 import 하지 않는다.** 상태 전이와
리스크 평가 — 틀리면 돈이 나가는 로직 — 이 전부 거기 순수 함수로 모여 있어
인프라 없이 경계값을 전수 테스트한다. 이 규칙은 `arch_test.go` 가 강제한다.

## 설계 판단

- **리스크 평가는 점수가 아니라 한국어 문장을 반환한다.** "위험도 72점"으로는
  사장님이 승인할지 전화를 걸지 판단할 수 없다. "이 번호로 30일간 4번째
  신고입니다"를 봐야 한다.
- **차단 대상도 접수는 받고 보류한다.** 손님 화면은 정상 접수와 똑같다.
  차단 사실을 알려주면 번호를 바꿔가며 우회하는 법을 학습시킨다.
- **전화번호는 HMAC 으로 해시한다.** 한국 휴대폰은 경우의 수가 1억 미만이라
  순수 SHA256 이면 DB 유출 시 전수 대입으로 전부 복원된다.
- **이중 환불 방어선은 캐시가 아니라 DB 유니크 제약이다.** 캐시 멱등성은
  장애나 경합에서 뚫리지만 DB 제약은 안 뚫린다.
- **Redis 는 없어도 된다.** 레이트리밋과 멱등성 보조에만 쓰고, 캐시 오류는
  로깅 후 통과시킨다. 고장난 기계 앞에 선 손님이 Redis 때문에 환불을 못
  받으면 안 된다.

## 개발 방법론

[superpowers](https://github.com/obra/superpowers) — 스펙 → 실행계획 → TDD 구현.

- [설계 스펙](docs/superpowers/specs/2026-09-21-clawhub-design.md)
- [구현 계획](docs/superpowers/plans/2026-09-21-clawhub-phase1.md)

프론트엔드 디자인은 [taste-skill](https://github.com/Leonxlnx/taste-skill) 의
안티-슬롭 규칙을 따른다. 다이얼은 손님 폼 `VARIANCE 4 / MOTION 3 / DENSITY 4`,
대시보드 `3 / 2 / 6` — 돈과 개인정보가 오가고 사용자는 이미 화가 나 있으므로
기본값(8/6/4)보다 낮게 잡았다.

계획 단계의 리뷰 관점은 [gstack](https://github.com/garrytan/gstack) 을 참고했다.
