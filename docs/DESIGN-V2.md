# Handoff: claw-hub v2 — 오락실 톤 + 매장 캐릭터 6종

웹(Next.js `web/`)과 모바일(Android TWA / 네이티브) 개발자용 전달 문서다.
**이 문서만 읽고 구현할 수 있게 썼다.** 의문이 생기면 `prototypes/` 를 브라우저로 열어 확인한다.

---

## 1. Overview

무인매장에서 기계가 돈만 먹었을 때 손님이 QR로 환불을 접수하고 사장님이 승인하는 서비스.
v2에서 바뀐 것:

- 시각 언어: 청록 버튼 + 얇은 회색 테두리 → **노란 오락실 버튼 + 굵은 먹선 + 각진 그림자**
- 캐릭터: 토끼 1종 → **매장 종류별 캐릭터 6종**(같은 그림체 가족). 매장 종류로 캐릭터·증상·금액·스티커 문구가 바뀐다.
- 손님 폼: 한 페이지 긴 폼 → **한 화면에 질문 하나인 단계형** + 처리 상태 조회 화면 신설
- 대시보드: 카드 목록 → **줄 목록**(밀도 상승), 정산/통계 화면 신설

## 2. About the design files

`prototypes/` 의 HTML은 **디자인 레퍼런스**다. 그대로 배포할 코드가 아니다.
기존 코드베이스(`judy98-s/claw-hub`, Next.js + Tailwind v4 + Phosphor)의 패턴으로 **다시 구현**한다.
모바일은 같은 화면을 TWA로 감싸거나(현재 `docs/ANDROID.md`), 네이티브로 옮길 때 이 문서의 값을 쓴다.

열어 보기: `prototypes/*.dc.html` 을 브라우저에서 직접 연다(같은 폴더의 `support.js`, `ppobi3.js` 필요, 인터넷 연결 필요 — 폰트/아이콘 CDN).

## 3. Fidelity

**High-fidelity.** 색·글자·간격·곡률·카피는 최종값이다. 단, **캐릭터 그림은 SVG 스케치**다.
일러스트레이터가 다듬은 최종본이 오면 `characters/` 의 SVG만 교체하면 된다(파일명·뷰박스 유지 요청).

---

## 4. Design tokens

### 4.1 색

| 토큰 | 값 | 역할 |
|---|---|---|
| `ink` | `#16130F` | 글자, 굵은 테두리, 각진 그림자, 상단 바 바탕 |
| `ink-soft` | `#5C554D` | 보조 글자 (paper 위 4.9:1) |
| `paper` | `#FFFBF2` | 화면 바탕 |
| `paper-sunken` | `#E9E1D6` | 비활성 버튼 바탕, 앱 바깥 배경 |
| `surface` | `#FFFFFF` | 입력칸, 카드 |
| `action` | `#FFD21F` | **행동** — 주 버튼, 선택된 탭. 글자는 항상 `ink` (13.9:1) |
| `pink` | `#FF8FB1` | **장식** — 진행바, 강조 띠, 랜딩 면. 누를 수 있는 것의 바탕으로 쓰지 않는다 |
| `pink-soft` | `#FFD6E2` | 선택된 옵션 버튼 바탕, 안내 상자, 캐릭터 말풍선 |
| `seam` | `#D9466F` | 차트 최고값, 소량 강조 |
| `ok` | fg `#065F46` / bg `#ECFDF5` / 아이콘 `#047857` | 환불 완료 |
| `warn` | fg `#92400E` / bg `#FFFBEB` | 확인 필요, 고액 |
| `stop` | fg `#991B1B` / bg `#FEF2F2` / 버튼 `#B91C1C` | 보류, 거절, 점검 필요 |

> 원칙: 노랑 = 누르는 것, 분홍 = 꾸미는 것. 분홍 바탕에 분홍 버튼을 놓지 않는다.
> 기존 `globals.css` 의 `accent-*`(청록)는 v2에서 쓰지 않는다. `ink-*` 회색 스케일은 보조 글자에 그대로 써도 된다.

### 4.2 매장별 색 (`stores.json`)

| store | 캐릭터 | tint (타일·선택 바탕) | accentText (판 글자) |
|---|---|---|---|
| `claw` 인형뽑기 | 뽑이 · 토끼 | `#F9CDD6` | `#C45A74` |
| `laundry` 무인세탁 | 뽀송이 · 강아지 | `#C6DEF2` | `#3F6E99` |
| `karaoke` 코인노래방 | 랄라 · 삼색고양이 | `#FFD3A8` | `#C26A2A` |
| `photo` 무인사진관 | 찰칵이 · 곰 | `#FFF0B8` | `#A8821C` |
| `study` 스터디카페 | 또박이 · 햄스터 | `#CFE5C2` | `#4F7D43` |
| `cafe` 무인카페 | 몽글이 · 양 | `#E6D6F0` | `#7A5A92` |

tint 는 **장식 면에만** 쓴다(캐릭터 타일, 매장 선택 칩). 버튼 색은 매장과 상관없이 항상 `action`.

### 4.3 글꼴

| 용도 | 글꼴 | 비고 |
|---|---|---|
| 본문·UI 전부 | **Pretendard Variable** | 기존과 동일. 자체 호스팅 동적 서브셋 유지 |
| 라벨·숫자 배지·로고 | **Galmuri11** (OFL) | 픽셀 글꼴. 12–16px, 한 화면 2–3곳 이하. 본문에 쓰지 않는다 |
| 캐릭터 판·스티커 글자 | **Jua** (OFL) | QR 스티커, 랜딩 가랜드에만 |

글자 크기 (px / weight)

| 역할 | 손님 폼 | 대시보드 | 랜딩 |
|---|---|---|---|
| 화면 제목 | 27 / 800 | 20 / 700 | clamp(46, 7.4vw, 84) / 900, letter-spacing -0.035em |
| 섹션 제목 | – | 17 / 700 | clamp(30, 4.6vw, 48) / 900 |
| 본문 | 16–17 / 500 | 14–15 / 400–600 | 17–19 / 500, line-height 1.6–1.65 |
| 선택 버튼 라벨 | 18 / 700 + 보조 14 | – | – |
| 입력칸 | 19 (iOS 확대 방지 위해 16 미만 금지) | – | – |
| 주 버튼 | 18 / 800 | 15–16 / 800 | 18 / 800 |
| 목록 줄 | – | 15 / 700 + 보조 13 | – |
| 배지 | – | 12 / 600, 높이 20 | – |

공통: `word-break: keep-all; overflow-wrap: anywhere;` (기존 유지). 금액은 `font-variant-numeric: tabular-nums`.

### 4.4 곡률·선·그림자

| 요소 | 값 |
|---|---|
| 기본 radius | **6px** (손님 폼), **4px** (대시보드), 0–6px (랜딩) |
| 휴대폰 프레임·시트 상단 | 14–18px (프레임 자체는 기기 몫) |
| 테두리 | 손님 폼·대시보드 카드/입력/버튼 `2px solid ink`, 랜딩 3px, 랜딩 기계 4px |
| 목록 구분선 | `1px solid #F2F0EC` (카드 안), 카드 외곽만 2px |
| 주 버튼 그림자 | `4px 4px 0 ink` (폼), `3px 3px 0 ink` (대시보드), `5px 5px 0 ink` (랜딩) |
| 눌림(active) | `transform: translate(3px,3px)` + 그림자 1–2px로 축소. 손님 폼은 눌림 효과 없음 |
| 입력 포커스 | `box-shadow: 3px 3px 0 #FF8FB1` + 테두리 유지 |
| 그라데이션·글래스·블러 | 쓰지 않는다 |

### 4.5 간격

4px 배수. 자주 쓰는 값: 4 · 6 · 8 · 10 · 12 · 14 · 16 · 20 · 24 · 28.
손님 폼 섹션 간 20, 화면 좌우 20. 대시보드 좌우 14, 줄 패딩 10×12.

### 4.6 아이콘

Phosphor (기존과 동일). regular 기본, 선택·상태는 fill, 화살표·닫기는 bold.
사용: caret-left, list, x, plus, camera, info, money, credit-card, lock-simple, check-circle, phone, qr-code, printer, download-simple, warning, clock, pause-circle, paper-plane-tilt, shield-warning, image, wrench, moon, bell-ringing, arrow-right, shield-check, package.

---

## 5. 캐릭터

### 5.1 공통 그림 규칙 (6종이 한 가족으로 보이는 이유)

- 뷰박스 200 기준. 외곽선 `#5A3D33`, 굵기 **2.6**, round cap/join. 표시 크기에 비례해 스케일(선 굵기 고정 아님).
- 머리: 살짝 눌린 원 `x 30–170, y 62–156`. 6종 동일.
- 눈: 짙은 갈색 `#3B2722` 타원 rx 6.4 ry 7.8, 하이라이트 2개. 눈 사이 48.
- 볼: `#F6A3B4` 75% 타원 + 빗금 3줄 `#E2788F`.
- 입: 기본 ω, 기쁨 열린 반원 `#E2788F`.
- 다른 점: 귀 · 무늬 · 소품만.
- **표정은 귀(또는 머리 위 요소)의 각도로 먼저 읽힌다.** 기쁨=선다, 걱정·졸림=처진다.

### 5.2 표정과 쓰는 자리

| mood | 쓰는 자리 |
|---|---|
| `calm` | 헤더 로고, 기본 |
| `happy` | 접수 완료, 환불 완료, 랜딩 마무리 |
| `worried` | 손님 폼 1~3단계 헤더, 대시보드 알림 말풍선, QR 스티커 |
| `sleep` | 대시보드 "처리할 것 없음" 빈 상태, 푸터 |

### 5.3 파일 (`characters/`)

캐릭터별 폴더 6개 × 7파일 = 42개 SVG.

```
characters/{ppobi|ppoksong|lala|chalkak|ttobak|monggeul}/
  {name}_main.svg          소품 포함, 전신, calm   (viewBox 0 0 200 212)
  {name}_calm|happy|worried|sleep.svg   소품 없음, 전신  (작은 자리·아이콘용)
  {name}_peek.svg          소품 포함, 판 잡고 빼꼼 (viewBox 0 0 200 172, 아래 가장자리 = 판 윗선)
  {name}_peek_worried.svg  QR 스티커용
```

- **22–32px 자리에서는 소품 없는 파일**을 쓴다(소품이 뭉개진다).
- peek 는 SVG 아래 가장자리가 판의 윗선이다. 판(흰 카드)을 **SVG 높이의 약 4%만큼 겹쳐** 앞발이 판을 잡은 것처럼 놓는다. 캐릭터를 판 위 레이어(z-index)에 둔다.
- 웹: `web/components/mascot.tsx` 를 `animal`·`prop`·`mood`·`mode` props 를 받는 컴포넌트로 교체. 생성 로직은 `prototypes/ppobi3.js` 가 그대로 참고 소스다(순수 SVG 문자열).
- Android 네이티브: SVG → Vector Drawable 변환(Android Studio Vector Asset). `clipPath` 를 쓰는 고양이·강아지·햄스터 무늬는 변환 후 확인 필요. 문제 시 PNG @1x/2x/3x 로 대체.
- iOS: SVG 를 PDF 벡터 에셋으로 넣거나 SVG 직접 지원(iOS 13+ Asset Catalog).

---

## 6. 화면

### 6.1 손님 환불 폼 `/r/{code}` — `prototypes/손님 환불 폼 v2.dc.html`

**목적**: 고장난 기계 앞에서 한 손으로, 지하 LTE에서 끝까지 접수시키기.
**규칙**: 애니메이션 없음. 한 화면에 질문 하나. 주 버튼은 하단 고정.

레이아웃 (모바일 390 기준, max-width 448 가운데 정렬)

```
[헤더 padding 14 12 0]
  ← 이전(44×44, 첫 단계는 숨김)                 "2 / 6" (Galmuri 14)
  진행바 높이 8, 2px ink 테두리, 채움 pink, radius 0
  기계 명패: ink 바탕, radius 6, padding 8 12 8 8
     [캐릭터 40px, mood=단계별]  {unitLabel}(Galmuri 12 #FFD21F) / {매장명}(15/700 paper)   "환불 접수"(Galmuri 11 pink)
[본문 padding 20, 스크롤]
  질문 제목 27/800
  보조 문장 16 ink-soft
  컨트롤
[하단 바 padding 12 16 20 + safe-area, border-top 2px ink, paper 바탕]
  주 버튼 높이 58, full width
```

단계

| # | 제목 | 컨트롤 | 넘어가는 조건 |
|---|---|---|---|
| 1 | 어떤 문제인가요? | 2열 선택 버튼 4개, 최소 높이 88. 라벨 18/700 + 힌트 14. 매장별 `issues` | 선택 시 **자동으로 다음** (설정 가능) |
| 2 | 얼마를 잃으셨나요? | 2열 금액 버튼 4개(높이 60, 19/700) + "다른 금액" 숫자 입력(높이 56, 오른쪽 "원") | 금액 > 0 |
| 3 | 증상 사진을 올려주세요 | 사진 3열 썸네일(삭제 버튼 32×32) + 점선 추가 버튼 높이 64. 최대 3장 | 기준액(기본 5,000원) 이상이면 1장 필수. 아니면 버튼 문구 "사진 없이 넘어가기" |
| 4 | 어떻게 결제하셨나요? | 세로 선택 버튼 2개(현금/카드), 높이 84, 아이콘 28 | 선택 시 자동으로 다음 |
| 5a | 환불받을 계좌 (현금) | 은행 3열 6개(높이 52) + "다른 은행 찾기" 링크 → 12개, 계좌번호(숫자만, 최대 16), 예금주(최대 20) | 은행 + 계좌 8자리 이상 + 예금주 |
| 5b | 카드 뒷자리 4자리 (카드) | pink-soft 안내 상자 + 4자리 입력(높이 64, 28/700, 가운데, letter-spacing .3em) | 4자리 |
| 6 | 연락받을 번호 | tel 입력 | 숫자 10자리 이상 |
| 7 | 이대로 접수할까요? | 요약 카드: 증상·금액·사진·환불·연락처 줄(높이 52) + 각 줄 "수정"(해당 단계로 이동). 자물쇠 안내 문장 | 주 버튼 "신고 접수하기" |
| 8 | 접수되었습니다 | ok 체크 64, 결제 수단별 안내 문장, 접수번호 박스(mono 28/700 letter-spacing .12em), "처리 상태 보기"(보조 버튼), "다른 문제 신고하기"(링크) | – |
| 9 | 처리 상태 | 캐릭터 56 + 제목, 세로 타임라인 3단(접수됨 → 사장님 확인 → 계좌로 환불/카드 결제 취소), 요약 카드, "매장에 전화하기" | – |

선택 버튼 상태

| 상태 | 바탕 | 테두리 | 글자 |
|---|---|---|---|
| 기본 | `#FFFFFF` | 2px ink | ink / 힌트 ink-soft |
| 선택 | `pink-soft #FFD6E2` | 2px ink | ink |
| 주 버튼 활성 | `action #FFD21F` | 2px ink + 그림자 4px | ink 18/800 |
| 주 버튼 비활성 | `#E9E1D6` | 2px ink | ink-soft, cursor not-allowed |

처리 상태 화면(9) 상태값

| 서버 status | 제목 | 캐릭터 | 타임라인 |
|---|---|---|---|
| `pending`, `needs_review`, `on_hold` | 확인을 기다리고 있어요 | calm | 2단계 현재 |
| `approved` | 승인되었어요 | calm | 3단계 현재 |
| `paid` | 환불이 끝났어요 / 결제가 취소되었어요 | happy | 전부 완료(ok) |
| `rejected` | 환불이 어렵습니다 | worried | 2단계 stop, 3단계 숨김 |

> `on_hold`(리스크 보류)도 손님에게는 "확인을 기다리고 있어요"로 똑같이 보인다. 차단 사실을 알리면 우회를 학습한다(기존 `done/page.tsx` 원칙 유지).

헤더 캐릭터 mood: 1–3단계 worried, 4–6단계 calm, 7단계 happy.

### 6.2 사장님 대시보드 `/admin` — `prototypes/사장님 대시보드 v2.dc.html`

**목적**: 하루 수십 번, 햇빛 아래 폰으로 빠르게 판단. **밀도 우선**.

공통 상단 바: 높이 52, ink 바탕. 메뉴(44×44, 미처리 있으면 빨간 점 9px) · 매장 캐릭터 26px · "claw-hub"(Galmuri 15 action) · 매장명(13/600 pink).

**① 민원 목록**
- 제목 "민원" 20/700 + "30초마다 새로고침" 13.
- 오늘 요약 3칸(접수 · 환불 · 나간 금액), 2px ink 카드.
- **캐릭터 말풍선 줄**: pink-soft, 매장 캐릭터 worried 34px + 한 줄(14/600). 기계 이상 감지 시에만 노출. 문구는 서버가 준다(예: "7번 기계에 오늘 민원 4건째야. 점검해 봐.").
- 탭 4개(처리 대기 · 보류 · 송금 대기 · 완료) 높이 34, 2px ink, 선택 = action 바탕.
- 목록: 한 카드 안에 줄로. 줄 = `[기계 · 증상 15/700] [금액 15/700 오른쪽]` / `[상태 배지][태그 배지들] [n분 전 13]`. `needs_review` 줄은 바탕 `#FFFDF5`.
- 정렬: needs_review·리스크 있음 먼저(기존 로직 유지).

**② 민원 상세**
- 상단 바: ← 민원 · 접수번호(Galmuri, action 색).
- 제목 20/700 + 금액 24/700 오른쪽, 배지(상태 · 결제 수단).
- 확인 이유 상자(warn) — 점수 대신 규칙 문장 그대로.
- 사진 3열, 탭하면 전체화면.
- 정보 줄(높이 40): 연락처(+전화 버튼 32×32 action) · 환불 계좌 · 예금주 · 이 번호 30일 신고 · 30일 누적 환불.
- 처리 기록 접기.
- **하단 고정 행동 바**: 3열 `2fr 1fr 1fr` — 승인(action) · 보류(흰) · 거절(글자 stop). 승인 후에는 "환불 보내기"로 바뀐다.
- 거절은 바텀시트 확인("거절한 건은 되돌릴 수 없습니다").

**③ 기계·QR 관리**
- 줄: 기계명 15/700 + 위치 13 / 배지(점검 필요 · 24시간 n건 · 대기 n) · 7일 막대(5px 폭, 하루 3건 이상 stop 색, 그 외 pink, 0건 #E5E2DC) · QR 버튼 40×40.
- QR 시트: 매장 tint 바탕 스티커 미리보기 + 인쇄하기(action) / 이미지 저장.

**④ 정산/통계**
- 기간 세그먼트(이번 주 · 이번 달 · 지난달).
- 요약 3칸 + 전기간 대비(증가 warn 색, 감소 ok 색).
- 날짜별 나간 금액 막대(최고값 seam).
- 기계별 표: 기계 · 민원 · 환불액 · 비중 막대(점검 필요 기계는 stop + 렌치 아이콘).
- 계좌 송금 / 카드 취소 / 거절·보류 요약. "엑셀로 받기".

### 6.3 랜딩 `/` — `prototypes/랜딩 v2.dc.html`

유일하게 장식을 허용한 화면. 반응형(모바일 1열 → 데스크톱 2열, 경계 약 900px).

1. **전광판 띠**: ink 바탕, Galmuri 13 action, 전구 점 8px(glow). 문구 반복.
2. 헤더: 캐릭터 32 + "claw-hub"(Galmuri 20) · 로그인 · 매장 만들기(action 버튼).
3. **히어로**: 왼쪽 — 배지(pink), 제목("돈만 먹었어요" 부분은 ink 띠 위 pink 글자, -1.5deg), 본문, 주 버튼 "무료로 시작하기"(높이 60, 3px ink, 그림자 5px), 보조 링크, 특징 칩 3개. 오른쪽 — **뽑기 기계 일러스트**(CSS 상자: 4px ink 외곽, 상단 전광판, 유리창 안 캐릭터, 상품 출구, 동전 투입구) + 알림 카드(-3deg, 승인/보류).
4. **세 판이면 끝납니다**: ink 바탕, STAGE 1 / STAGE 2 / CLEAR! (Galmuri), 점선 구분, 오른쪽에 캐릭터(worried → calm → happy). 균등 카드 3개로 만들지 않는다.
5. **매장 종류**: 3열 타일 6개 — 각 매장 캐릭터 96px + 매장명 + 캐릭터 이름, tint 바탕, ±2deg 기울기.
6. 신뢰 2칸(pink-soft 띠, 3px ink 구분).
7. 마무리: ink 상자 + pink 그림자 8px, "INSERT COIN", 캐릭터 happy, 주 버튼.
8. 푸터.

랜딩 애니메이션은 넣어도 된다(전광판 흐름, 전구 깜빡임, 버튼 눌림). `prefers-reduced-motion` 에서는 끈다.

---

## 7. 인터랙션 · 상태

손님 폼 state

```
store: "claw"|"laundry"|...      // QR code → 서버가 machine.storeType 로 내려줌
step: 0..8
issue, amount, photos[], pay: "cash"|"card",
bank, account, holder, last4, phone
allBanks: boolean
idempotencyKey               // 폼 열 때 1회 생성, 재시도 사이 유지 (기존 유지)
```

- 뒤로 가기: 이전 단계로. 입력값은 유지.
- 제출 실패: 입력값 유지, 확인 단계에서 stop 상자로 에러(기존 유지).
- 브라우저 뒤로 가기도 단계 이동으로 처리(history state 또는 `?step=`).

대시보드

- 목록·홈 30초 폴링(기존).
- 승인 → `approved` → 버튼 "환불 보내기" → 기존 PayoutSheet.
- 거절은 확인 시트를 거친다.

**서버에 필요한 추가 값**
- `machine.storeType` (`claw|laundry|karaoke|photo|study|cafe`) — 폼 캐릭터·증상·빠른 금액·스티커 문구 결정. 매장 생성 시 고르게 한다.
- 매장별 `issues` enum 확장 (`stores.json` 참고).
- 공개 상태 조회 API: `GET /api/public/claims/{receiptCode}` → status, 타임라인 시각, 금액, 결제 수단.
- 대시보드 말풍선 문구(선택): 홈 actions 의 `machine_alert` 를 한 줄 문장으로.

## 8. 모바일 개발 메모

- 손님은 100% 모바일 브라우저(앱 설치 없음). TWA/앱은 **사장님용**.
- 하단 고정 버튼은 `env(safe-area-inset-bottom)` 만큼 띄운다.
- 키보드: 금액·계좌·카드 `inputmode="numeric"`, 전화 `type="tel"`. 입력칸 글자 16px 이상(iOS 확대 방지).
- 사진: `accept="image/*" capture="environment"`, 업로드 전 축소(기존 `lib/image.ts`).
- 확대(pinch zoom)를 막지 않는다.
- 터치 타깃 최소 44×44. 대시보드 배지·보조 링크도 감싸는 줄 전체가 눌리게.
- 다크 모드: v2 시안은 라이트만. 다크는 `ink`↔`paper` 를 뒤집고 action 노랑은 유지하는 방향을 권장하나 확정 전.
- 상단 바 ink 바탕이므로 `theme-color` 를 `#16130F` 로.
- 푸시 알림 아이콘: 매장 캐릭터 소품 없는 `calm`, 단색 실루엣 버전 필요(미제작).

## 9. Assets

| 자산 | 위치 | 출처 |
|---|---|---|
| 캐릭터 SVG 42개 | `characters/` | 이 프로젝트에서 제작(스케치). 상용 사용 가능, 최종 일러스트로 교체 예정 |
| 캐릭터 생성 소스 | `prototypes/ppobi3.js` | 동일 |
| 매장 설정 | `stores.json` | 동일 |
| Pretendard | 기존 자체 호스팅 | OFL |
| Galmuri11 | `cdn.jsdelivr.net/npm/galmuri` → 자체 호스팅 권장 | OFL |
| Jua | Google Fonts → 자체 호스팅 권장 | OFL |
| 아이콘 | `@phosphor-icons/react` (기존) | MIT |
| QR | 프로토타입은 줄무늬 자리표시. 실제 QR 로 교체 | – |

## 10. Files

```
design_handoff_claw_hub/
  README.md                     이 문서
  stores.json                   매장 6종 설정 (캐릭터·색·증상·빠른 금액·스티커 문구)
  characters/                   캐릭터 SVG 42개
  prototypes/
    손님 환불 폼 v2.dc.html     클릭 가능. 오른쪽 패널에서 매장·단계·처리 상태 전환
    사장님 대시보드 v2.dc.html  4화면 나란히. 위쪽 칩으로 매장 전환. 탭·승인·거절·기간 동작
    랜딩 v2.dc.html             반응형
    캐릭터 시트.dc.html         6종 × 표정 4 + peek + 크기 + 색 + QR 스티커 6종
    ppobi3.js, support.js       프로토타입 실행용
```

기존 코드와의 대응

| 프로토타입 | 기존 파일 |
|---|---|
| 손님 폼 1–7 | `web/app/r/[code]/claim-form.tsx` |
| 손님 폼 8 | `web/app/r/[code]/done/page.tsx` |
| 손님 폼 9 | 신규 `web/app/r/[code]/status/page.tsx` 권장 |
| 대시보드 ① | `web/app/admin/(shell)/claims/page.tsx` (+ 홈 요약 `admin/(shell)/page.tsx`) |
| 대시보드 ② | `web/app/admin/claims/[id]/page.tsx` |
| 대시보드 ③ | `web/app/admin/(shell)/machines/page.tsx`, `qr-sheet.tsx` |
| 대시보드 ④ | 신규 (데이터는 `internal/store/stats.go`) |
| 상단 바 | `web/app/admin/(shell)/nav-sidebar.tsx` |
| 배지 | `web/components/ui/badge.tsx` (색 값만 교체) |
| 버튼·입력 | `web/components/ui/button.tsx`, `field.tsx` |
| 캐릭터 | `web/components/mascot.tsx` |
| 토큰 | `web/app/globals.css` |
| 랜딩 | `web/app/page.tsx` |
