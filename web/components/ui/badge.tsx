import type { ReactNode } from "react";

export type Tone = "neutral" | "ok" | "warn" | "stop" | "accent";

const tones: Record<Tone, string> = {
  neutral: "bg-[var(--surface-sunken)] text-[var(--muted)]",
  ok: "bg-[var(--tone-ok-bg)] text-[var(--tone-ok-fg)]",
  warn: "bg-[var(--tone-warn-bg)] text-[var(--tone-warn-fg)]",
  stop: "bg-[var(--tone-stop-bg)] text-[var(--tone-stop-fg)]",
  accent: "bg-[var(--tone-accent-bg)] text-[var(--tone-accent-fg)]",
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
        "inline-flex items-center gap-1 rounded-md px-2 py-0.5",
        "text-xs font-semibold whitespace-nowrap",
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
