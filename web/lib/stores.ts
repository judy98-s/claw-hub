/**
 * 매장 종류별 설정. docs/stores.json 과 같은 값이다.
 *
 * 손님 폼의 캐릭터·증상 목록·빠른 금액, 대시보드 상단 바, QR 스티커 문구가
 * 전부 이 표로 갈린다. 서버가 `machine.storeType` 을 내려주면 그 키로 찾는다.
 *
 * TS 로 옮겨 둔 이유: JSON 을 import 하면 타입이 넓어져서 오타가 런타임까지
 * 간다. 매장은 여섯 개뿐이고 자주 바뀌지 않는다.
 */

export const STORE_TYPES = [
  "claw",
  "laundry",
  "karaoke",
  "photo",
  "study",
  "cafe",
] as const;

export type StoreType = (typeof STORE_TYPES)[number];

export type StoreConfig = {
  label: string;
  /** characters/{character}/ 폴더 이름이자 파일 접두사 */
  character: string;
  characterName: string;
  /** tint 는 장식 면에만 쓴다. 버튼 색은 매장과 무관하게 항상 action 이다. */
  tint: string;
  accentText: string;
  unitLabel: string;
  stickerHeadline: string;
  issues: { value: string; label: string; hint: string }[];
  quickAmounts: number[];
};

export const STORES: Record<StoreType, StoreConfig> = {
  claw: {
    label: "인형뽑기",
    character: "ppobi",
    characterName: "뽑이",
    tint: "#F9CDD6",
    accentText: "#C45A74",
    unitLabel: "7번 기계",
    stickerHeadline: "돈만 먹었나요?",
    issues: [
      { value: "doll_stuck", label: "인형 걸림", hint: "집었는데 중간에 걸렸어요" },
      { value: "cash_eaten", label: "돈만 빠짐", hint: "결제는 됐는데 안 움직여요" },
      { value: "claw_broken", label: "집게 불량", hint: "집는 힘이 너무 약해요" },
      { value: "other", label: "기타", hint: "그 외 문제" },
    ],
    quickAmounts: [1000, 2000, 3000, 5000],
  },
  laundry: {
    label: "무인세탁",
    character: "ppoksong",
    characterName: "뽀송이",
    tint: "#C6DEF2",
    accentText: "#3F6E99",
    unitLabel: "3번 세탁기",
    stickerHeadline: "세탁기가 멈췄나요?",
    issues: [
      { value: "no_start", label: "작동 안 함", hint: "결제했는데 시작을 안 해요" },
      { value: "stopped_mid", label: "중간에 멈춤", hint: "돌다가 멈췄어요" },
      { value: "not_dry", label: "건조 덜 됨", hint: "시간이 끝났는데 젖어 있어요" },
      { value: "other", label: "기타", hint: "세제 안 나옴 · 문 안 열림" },
    ],
    quickAmounts: [3000, 4000, 5000, 7000],
  },
  karaoke: {
    label: "코인노래방",
    character: "lala",
    characterName: "랄라",
    tint: "#FFD3A8",
    accentText: "#C26A2A",
    unitLabel: "5번 방",
    stickerHeadline: "곡이 안 올라갔나요?",
    issues: [
      { value: "no_credit", label: "곡 수 안 올라감", hint: "넣었는데 0곡이에요" },
      { value: "mic_broken", label: "마이크 고장", hint: "소리가 안 나와요" },
      { value: "cut_off", label: "중간에 끊김", hint: "부르다가 꺼졌어요" },
      { value: "other", label: "기타", hint: "그 외 문제" },
    ],
    quickAmounts: [500, 1000, 2000, 5000],
  },
  photo: {
    label: "무인사진관",
    character: "chalkak",
    characterName: "찰칵이",
    tint: "#FFF0B8",
    accentText: "#A8821C",
    unitLabel: "2번 부스",
    stickerHeadline: "사진이 안 나왔나요?",
    issues: [
      { value: "no_print", label: "사진 안 나옴", hint: "찍었는데 인화가 안 돼요" },
      { value: "bad_print", label: "인화 불량", hint: "줄이 가거나 색이 이상해요" },
      { value: "no_shoot", label: "촬영 안 됨", hint: "결제했는데 화면이 그대로예요" },
      { value: "other", label: "기타", hint: "QR 다운로드 안 됨 등" },
    ],
    quickAmounts: [4000, 5000, 6000, 8000],
  },
  study: {
    label: "스터디카페",
    character: "ttobak",
    characterName: "또박이",
    tint: "#CFE5C2",
    accentText: "#4F7D43",
    unitLabel: "키오스크",
    stickerHeadline: "결제가 꼬였나요?",
    issues: [
      { value: "no_seat", label: "좌석 배정 안 됨", hint: "결제했는데 좌석이 없어요" },
      { value: "door_locked", label: "출입문 안 열림", hint: "입장 QR이 안 먹혀요" },
      { value: "double_charge", label: "두 번 결제", hint: "한 번 샀는데 두 번 빠졌어요" },
      { value: "other", label: "기타", hint: "그 외 문제" },
    ],
    quickAmounts: [2000, 3000, 5000, 10000],
  },
  cafe: {
    label: "무인카페",
    character: "monggeul",
    characterName: "몽글이",
    tint: "#E6D6F0",
    accentText: "#7A5A92",
    unitLabel: "1번 커피머신",
    stickerHeadline: "음료가 안 나왔나요?",
    issues: [
      { value: "no_drink", label: "음료 안 나옴", hint: "결제했는데 그대로예요" },
      { value: "no_cup", label: "컵 안 나옴", hint: "음료가 그냥 흘러요" },
      { value: "wrong_drink", label: "다른 음료", hint: "고른 것과 달라요" },
      { value: "other", label: "기타", hint: "얼음 안 나옴 등" },
    ],
    quickAmounts: [1500, 2000, 3000, 4500],
  },
};

/** 모르는 값이 와도 화면이 깨지지 않게 인형뽑기로 떨어뜨린다. */
export function storeConfig(type: string | undefined | null): StoreConfig {
  return STORES[(type ?? "") as StoreType] ?? STORES.claw;
}
