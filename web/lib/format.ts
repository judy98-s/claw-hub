/** 금액을 "2,000원" 으로 만든다. */
export function krw(n: number): string {
  return `${n.toLocaleString("ko-KR")}원`;
}

/** 전화번호를 "010-1234-5678" 로 만든다. */
export function phone(raw: string): string {
  const d = raw.replace(/[^0-9]/g, "");
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

  return then.toLocaleDateString("ko-KR", { month: "long", day: "numeric" });
}

/** 날짜·시각을 "9월 21일 14:32" 로 만든다. */
export function dateTime(iso: string): string {
  return new Date(iso).toLocaleString("ko-KR", {
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
