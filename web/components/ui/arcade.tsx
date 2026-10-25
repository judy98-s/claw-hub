import type { ReactNode } from "react";

/**
 * v2 오락실 톤의 공통 조각. docs/DESIGN-V2.md §4.4.
 *
 * 왜 컴포넌트로 빼는가: 굵은 먹선 + 각진 그림자 + 눌림은 조합이 길어서
 * (`border-2 border-ink shadow-[4px_4px_0_#16130F] active:translate-...`)
 * 화면마다 손으로 쓰면 값이 조금씩 어긋난다. 그림자 4px 와 5px 가 섞이면
 * 같은 화면인데 다른 톤으로 보인다.
 *
 * 색의 역할: action(노랑)은 누르는 것, pink 는 꾸미는 것.
 */

type Tone = "action" | "plain" | "stop";

const TONE: Record<Tone, string> = {
  // 주 버튼. 노랑 위 ink 글자는 대비 13.9:1.
  action: "bg-action text-ink",
  plain: "bg-surface text-ink",
  // 거절처럼 되돌릴 수 없는 행동. 바탕이 아니라 글자로만 경고한다 —
  // 빨간 버튼을 나란히 놓으면 승인보다 먼저 눈에 들어온다.
  stop: "bg-surface text-stop-600",
};

export function ArcadeButton({
  children,
  tone = "action",
  size = "md",
  full,
  disabled,
  loading,
  type = "button",
  onClick,
  className = "",
}: {
  children: ReactNode;
  tone?: Tone;
  size?: "md" | "lg";
  full?: boolean;
  disabled?: boolean;
  loading?: boolean;
  type?: "button" | "submit";
  onClick?: () => void;
  className?: string;
}) {
  const off = disabled || loading;
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={off}
      aria-busy={loading || undefined}
      className={[
        "inline-flex items-center justify-center gap-2 rounded-md border-2 border-ink font-extrabold",
        "transition-transform",
        size === "lg" ? "h-[58px] px-6 text-[18px]" : "h-11 px-4 text-[15px]",
        full ? "w-full" : "",
        off
          ? "cursor-not-allowed bg-paper-sunken text-ink-soft"
          : `${TONE[tone]} shadow-[4px_4px_0_#16130F] active:translate-x-[3px] active:translate-y-[3px] active:shadow-[1px_1px_0_#16130F]`,
        className,
      ].join(" ")}
    >
      {children}
    </button>
  );
}

/** 입력칸. 포커스는 분홍 각진 그림자로 보인다(테두리는 그대로 둔다). */
export function ArcadeInput({
  className = "",
  ...props
}: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={[
        // 19px. iOS 는 16px 미만 입력칸에서 화면을 확대한다.
        "h-14 w-full rounded-md border-2 border-ink bg-surface px-4 text-[19px] font-medium",
        "outline-none placeholder:text-ink-soft/60",
        "focus:shadow-[3px_3px_0_#FF8FB1]",
        "aria-[invalid=true]:shadow-[3px_3px_0_#B91C1C]",
        className,
      ].join(" ")}
    />
  );
}

/** 2px 먹선 카드. */
export function ArcadeCard({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-md border-2 border-ink bg-surface ${className}`}>{children}</div>
  );
}

/** 안내 상자. 경고가 아니라 "읽어 두면 좋은 것" 이다. */
export function ArcadeNote({ children }: { children: ReactNode }) {
  return (
    <p className="rounded-md border-2 border-ink bg-pink-soft px-4 py-3 text-[14px] font-medium leading-relaxed">
      {children}
    </p>
  );
}

/** 오류 상자. 폼 제출이 막혔을 때만 쓴다. */
export function ArcadeError({ children }: { children: ReactNode }) {
  return (
    <p
      role="alert"
      className="rounded-md border-2 border-stop-600 bg-stop-50 px-4 py-3 text-[14px] font-semibold text-stop-700"
    >
      {children}
    </p>
  );
}
