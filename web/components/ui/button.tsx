import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "md" | "lg";

const variants: Record<Variant, string> = {
  primary: "bg-accent-600 text-white hover:bg-accent-700 active:bg-accent-700",
  secondary:
    "bg-[var(--surface)] text-[var(--fg)] border border-[var(--line)] hover:bg-[var(--surface-sunken)]",
  ghost: "text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]",
  danger: "bg-stop-600 text-white hover:bg-stop-700 active:bg-stop-700",
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
        "inline-flex items-center justify-center gap-2 rounded-lg font-semibold",
        "transition-colors duration-150",
        "disabled:opacity-50 disabled:cursor-not-allowed",
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
