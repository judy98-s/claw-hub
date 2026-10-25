import type { ReactNode } from "react";

export type Tone = "neutral" | "ok" | "warn" | "stop" | "accent";

/*
  v2 색 (docs/DESIGN-V2.md §4.1). 전부 바탕 대비 4.5:1 이상이다 —
  사장님이 햇빛 아래 폰으로 본다.

  accent 만 이름과 색이 어긋난다. v2 에서 청록을 쓰지 않기로 해서
  pink-soft 로 바꿨는데, 이름을 바꾸면 호출처를 전부 고쳐야 한다.
  "강조하되 상태는 아님" 이라는 뜻은 그대로다.
*/
const tones: Record<Tone, string> = {
  neutral: "bg-paper-sunken text-ink-soft",
  ok: "bg-ok-50 text-ok-700",
  warn: "bg-warn-50 text-warn-700",
  stop: "bg-stop-50 text-stop-700",
  accent: "bg-pink-soft text-ink",
};

/**
 * Badge 는 색만으로 뜻을 전하지 않는다.
 *
 * 색각 이상이 있는 사장님에게 초록/빨강 배지는 같은 회색으로 보인다.
 * 아이콘과 라벨을 항상 함께 둔다.
 */
export function Badge({
  tone = "neutral",
  icon,
  children,
}: {
  tone?: Tone;
  icon?: ReactNode;
  children: ReactNode;
}) {
  return (
    <span
      className={[
        // 높이 20, 12/600 (§4.3)
        "inline-flex h-5 items-center gap-1 rounded px-1.5",
        "text-[12px] font-semibold whitespace-nowrap",
        tones[tone],
      ].join(" ")}
    >
      {icon}
      {children}
    </span>
  );
}

/** 상태 → 배지 톤. 한곳에서 정해 화면마다 다른 색이 되지 않게 한다. */
export function toneForStatus(status: string): Tone {
  switch (status) {
    case "paid":
      return "ok";
    case "approved":
      return "accent";
    case "needs_review":
      return "warn";
    case "on_hold":
      return "stop";
    case "rejected":
      return "neutral";
    default:
      return "neutral";
  }
}
