# claw-hub

인형뽑기 기계의 고장·환불 문의를 접수하고 처리하는 시스템.

기계마다 붙은 QR 스티커를 손님이 찍으면 모바일 웹 신고 폼이 열린다. 손님은 증상·사진·금액·
전화번호·계좌를 입력하고 제출한다. 사장님은 Slack 알림을 받고 웹 대시보드에서 승인/거절하며,
승인 시 송금 딥링크 버튼으로 환불한다.

## 핵심 기능

- **QR 신고 접수** — 앱 설치·로그인·친구추가 없이 모바일 웹에서 60초 내 제출
- **반자동 환불** — 버튼 한 번으로 토스/카카오뱅크 송금 화면을 계좌·금액이 채워진 상태로 열기
- **체리피커 차단** — 전화번호·계좌 기준 반복 신고를 자동 탐지해 보류하고, 근거를 사장님에게 제시
- **기계 이상 탐지** — "3번 기계 오늘 3건"처럼 점검이 필요한 기계를 먼저 알림

## 스택

| 영역 | 선택 |
|---|---|
| 백엔드 | Go 1.24 · 표준 `net/http` · pgx |
| 데이터 | PostgreSQL · Redis (없으면 인메모리로 폴백) |
| 프론트 | Next.js 15 · Tailwind v4 · Pretendard |
| 알림 | Slack Webhook (알림 채널은 인터페이스로 추상화) |
| 배포 | 단일 VPS + Docker Compose |

## 문서

- [설계 스펙](docs/superpowers/specs/2026-09-21-clawhub-design.md)

## 개발 방법론

[superpowers](https://github.com/obra/superpowers) — 스펙 → 실행계획 → TDD 구현.
프론트엔드 디자인은 [taste-skill](https://github.com/Leonxlnx/taste-skill)의 안티-슬롭 규칙을 따른다.
