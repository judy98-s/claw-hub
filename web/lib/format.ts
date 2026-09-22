/**
 * 매장 시각.
 *
 * 이 앱은 한국 매장이 쓴다. 화면에 뜨는 "9월 1일"은 언제나 한국 시간
 * 9월 1일이어야 하고, 보는 사람의 기기 시간대에 따라 하루가 밀리면 안 된다.
 * 서버는 UTC로 돌고 사입일은 한국 자정으로 저장되므로, 시간대를 명시하지
 * 않으면 한국의 9월 1일이 화면에 8월 31일로 나온다 — 실제로 그랬다.
 */
const KST = "Asia/Seoul";

/** 금액을 "2,000원" 으로 만든다. */
export function krw(n: number): string {
  return `${n.toLocaleString("ko-KR")}원`;
}

/**
 * 전화번호에 하이픈을 넣는다. 휴대폰과 유선 둘 다 받는다.
 *
 * 매장 대표번호는 유선일 수 있어서(02-333-4444) 휴대폰 규칙만으로는
 * 숫자가 그대로 노출된다. 023334444 를 보고 읽을 사장님은 없다.
 */
export function phone(raw: string): string {
  const d = raw.replace(/[^0-9]/g, "");
  if (!d) return raw;

  // 서울 02 는 국번이 2자리다
  if (d.startsWith("02")) {
    if (d.length === 9) return `02-${d.slice(2, 5)}-${d.slice(5)}`;
    if (d.length === 10) return `02-${d.slice(2, 6)}-${d.slice(6)}`;
    return raw;
  }

  // 1588 같은 대표번호는 8자리 4-4
  if (d.length === 8 && /^1[5-9]/.test(d))
    return `${d.slice(0, 4)}-${d.slice(4)}`;

  if (d.length === 11) return `${d.slice(0, 3)}-${d.slice(3, 7)}-${d.slice(7)}`;
  if (d.length === 10) return `${d.slice(0, 3)}-${d.slice(3, 6)}-${d.slice(6)}`;
  return raw;
}

/**
 * 경과 시간을 "3분 전" 으로 만든다.
 *
 * 접수함에서 사장님이 가장 먼저 보는 값이다. "2026-09-21 14:32" 보다
 * "3분 전" 이 급한지 아닌지를 바로 알려준다.
 */
export function ago(iso: string, now: Date = new Date()): string {
  const then = new Date(iso);
  const sec = Math.floor((now.getTime() - then.getTime()) / 1000);

  if (sec < 60) return "방금";
  if (sec < 3600) return `${Math.floor(sec / 60)}분 전`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}시간 전`;
  if (sec < 86400 * 7) return `${Math.floor(sec / 86400)}일 전`;

  return then.toLocaleDateString("ko-KR", {
    month: "long",
    day: "numeric",
    timeZone: KST,
  });
}

/** 날짜·시각을 "9월 21일 14:32" 로 만든다. */
export function dateTime(iso: string): string {
  return new Date(iso).toLocaleString("ko-KR", {
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: KST,
  });
}

/**
 * 날짜를 "9/14" 로 만든다. 표 안에서는 짧을수록 읽힌다.
 *
 * toLocaleDateString 의 결과("9. 1.")를 정규식으로 다듬지 않고 부품을
 * 꺼내 쓴다. 문자열을 깎는 방식은 로캘 표기가 바뀌면 조용히 틀린 값을 낸다.
 */
const shortDateParts = new Intl.DateTimeFormat("ko-KR", {
  month: "numeric",
  day: "numeric",
  timeZone: KST,
});

export function shortDate(iso: string): string {
  const parts = shortDateParts.formatToParts(new Date(iso));
  const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "";
  return `${get("month")}/${get("day")}`;
}

/** "9월". 요약 제목에 쓴다. */
export function monthLabel(iso: string): string {
  return new Date(iso).toLocaleDateString("ko-KR", {
    month: "long",
    timeZone: KST,
  });
}
