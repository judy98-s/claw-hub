"use client";

import {
  Children,
  cloneElement,
  isValidElement,
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
} from "react";

/** 레이블을 htmlFor 로 붙일 수 있는 DOM 태그. */
const LABELABLE_TAGS = new Set(["input", "textarea", "select"]);

/**
 * 컴포넌트가 레이블을 받을 수 있음을 표시하는 플래그.
 *
 * 호출자는 <input> 이 아니라 <Input> 을 넘긴다. 태그 이름만 보면
 * 함수 컴포넌트라 전부 group 경로로 빠지고, 레이블이 아무 컨트롤에도
 * 연결되지 않는다.
 */
type Labelable = { __labelable?: boolean };

function isLabelable(type: unknown): boolean {
  if (typeof type === "string") return LABELABLE_TAGS.has(type);
  return (type as Labelable)?.__labelable === true;
}

/**
 * Field 는 레이블을 컨트롤에 실제로 연결한다.
 *
 * 레이블이 떠 있기만 하면 스크린리더가 "편집창"이라고만 읽고, 레이블을
 * 탭해도 포커스가 가지 않는다. 손가락이 큰 사람에게는 그 탭 영역이
 * 입력칸만큼 중요하다.
 *
 * 자식이 단일 입력 요소면 htmlFor 로 묶고, 버튼 묶음처럼 컨트롤이 여럿이면
 * role="group" + aria-labelledby 로 묶는다. label 하나에 컨트롤 여러 개를
 * 넣는 건 유효하지 않다.
 */
export function Field({
  label,
  hint,
  error,
  required,
  children,
}: {
  label: string;
  hint?: ReactNode;
  error?: string;
  required?: boolean;
  children: ReactNode;
}) {
  const id = useId();
  const labelId = `${id}-label`;
  const describedBy = error || hint ? `${id}-desc` : undefined;

  const only = Children.count(children) === 1 ? Children.only(children) : null;
  const canLabel = isValidElement(only) && isLabelable(only.type);

  const title = (
    <>
      {label}
      {required && (
        <span className="ml-1 text-stop-600" aria-hidden>
          *
        </span>
      )}
    </>
  );

  // 에러가 힌트를 대체한다. 둘 다 띄우면 어느 쪽을 따라야 할지 모른다.
  const note = error ? (
    <p id={describedBy} className="text-sm text-stop-600">
      {error}
    </p>
  ) : hint ? (
    <p id={describedBy} className="text-sm text-[var(--muted)]">
      {hint}
    </p>
  ) : null;

  if (canLabel) {
    const control = cloneElement(only as React.ReactElement<Record<string, unknown>>, {
      id,
      "aria-describedby": describedBy,
      "aria-invalid": error ? true : undefined,
      required,
    });
    return (
      <div className="grid gap-1.5">
        <label htmlFor={id} className="text-sm font-semibold text-[var(--fg)]">
          {title}
        </label>
        {control}
        {note}
      </div>
    );
  }

  return (
    <div className="grid gap-1.5" role="group" aria-labelledby={labelId} aria-describedby={describedBy}>
      <span id={labelId} className="text-sm font-semibold text-[var(--fg)]">
        {title}
      </span>
      {children}
      {note}
    </div>
  );
}

const inputBase =
  "w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3.5 " +
  // 16px 미만이면 iOS가 포커스 시 페이지를 확대한다. 확대되면 손님이
  // 폼을 채우다 길을 잃는다.
  "text-base text-[var(--fg)] placeholder:text-[var(--color-ink-400)] " +
  "focus:border-accent-600 focus:outline-none " +
  "aria-[invalid=true]:border-stop-600";

export function Input({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${inputBase} h-12 ${className}`} />;
}
Input.__labelable = true;

export function Textarea({ className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${inputBase} py-3 ${className}`} />;
}
Textarea.__labelable = true;
