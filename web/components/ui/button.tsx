import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "md" | "lg";

/*
  v2 (§4.4): 먹선 2px + 각진 그림자 + 눌리면 그림자가 줄며 밀린다.
  색의 역할은 action(노랑) = 누르는 것, pink = 꾸미는 것이다.

  ghost 만 테두리와 그림자가 없다. 보조 동작이 주 버튼과 같은 무게로
  보이면 무엇을 눌러야 하는지 흐려진다.
*/
const SHADOW =
  "shadow-[4px_4px_0_#16130F] active:translate-x-[3px] active:translate-y-[3px] " +
  "active:shadow-[1px_1px_0_#16130F]";

const variants: Record<Variant, string> = {
  primary: `border-2 border-ink bg-action text-ink ${SHADOW}`,
  secondary: `border-2 border-ink bg-surface text-ink ${SHADOW}`,
  ghost: "text-ink-soft hover:bg-paper-sunken hover:text-ink",
  // 되돌릴 수 없는 동작. 바탕이 아니라 글자로 경고한다 — 빨간 버튼을
  // 나란히 놓으면 승인보다 먼저 눈에 들어온다.
  danger: `border-2 border-ink bg-surface text-stop-600 ${SHADOW}`,
};

const sizes: Record<Size, string> = {
  md: "h-11 px-4 text-[15px]",
  // 손님 폼의 주요 동작. 길에서 한 손으로 누른다 — 44px 터치 타깃 하한을
  // 넉넉히 넘긴다.
  lg: "h-14 px-6 text-base",
};

export function Button({
  variant = "primary",
  size = "md",
  full,
  loading,
  children,
  className = "",
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  full?: boolean;
  loading?: boolean;
  children: ReactNode;
}) {
  return (
    <button
      {...props}
      disabled={disabled || loading}
      className={[
        "inline-flex items-center justify-center gap-2 rounded-md font-extrabold",
        "transition-transform",
        // 비활성은 투명도가 아니라 색으로 꺼진 것을 보인다. 투명하게만
        // 하면 햇빛 아래에서 눌리는 버튼과 구분이 안 된다.
        "disabled:cursor-not-allowed disabled:border-ink disabled:bg-paper-sunken",
        "disabled:text-ink-soft disabled:shadow-none disabled:translate-x-0 disabled:translate-y-0",
        variants[variant],
        sizes[size],
        full ? "w-full" : "",
        className,
      ].join(" ")}
    >
      {loading && (
        <span
          aria-hidden
          className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent"
        />
      )}
      {children}
    </button>
  );
}
