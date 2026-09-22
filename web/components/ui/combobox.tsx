"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type InputHTMLAttributes,
} from "react";

/**
 * 자유 입력 + 자동완성.
 *
 * 이 컴포넌트가 재고 장부의 핵심이다. 1단계에는 전국 인형 카탈로그가
 * 없어서 이름을 자유 입력으로 받는데, 그대로 두면 같은 사장님이 같은
 * 인형을 "쿠로미 중형"과 "쿠로미중형"으로 두 번 적고 장부가 두 줄이 된다.
 * 치는 동안 내가 전에 쓴 이름을 보여주고 고르게 하는 것만으로 그 절반이
 * 정리된다.
 *
 * <select> 가 아닌 이유: 없는 이름도 그대로 새 이름이 되어야 한다.
 * 목록에 없다고 등록을 막으면 첫 사용자는 아무것도 못 넣는다.
 */
export function Combobox({
  value,
  onChange,
  options,
  className = "",
  ...props
}: {
  value: string;
  onChange: (v: string) => void;
  options: string[];
} & Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange">) {
  const listId = useId();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const wrap = useRef<HTMLDivElement>(null);

  // 이미 그 이름을 그대로 쳤으면 목록을 띄울 이유가 없다. 고를 게
  // 자기 자신뿐인 목록이 입력칸을 가린다.
  const shown = options.filter((o) => o !== value).slice(0, 8);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!wrap.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  function pick(v: string) {
    onChange(v);
    setOpen(false);
    setActive(-1);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Escape") {
      setOpen(false);
      return;
    }
    if (!shown.length) return;

    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setOpen(true);
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((i) => (i + step + shown.length) % shown.length);
      return;
    }
    // Enter 로 후보를 고르는 건 목록에서 하나를 "가리키고 있을 때"뿐이다.
    // 아무것도 안 가리킨 상태의 Enter 는 지금 친 이름으로 제출하려는 것이다.
    if (e.key === "Enter" && open && active >= 0) {
      e.preventDefault();
      pick(shown[active]);
    }
  }

  return (
    <div ref={wrap} className="relative">
      <input
        {...props}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
          setOpen(true);
          setActive(-1);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
        role="combobox"
        aria-expanded={open && shown.length > 0}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
        autoComplete="off"
        className={
          "h-12 w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3.5 " +
          "text-base text-[var(--fg)] placeholder:text-[var(--color-ink-400)] " +
          "focus:border-accent-600 focus:outline-none " +
          className
        }
      />

      {open && shown.length > 0 && (
        <ul
          id={listId}
          role="listbox"
          className="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-[var(--line)] bg-[var(--surface)] py-1 shadow-lg"
        >
          {shown.map((o, i) => (
            <li
              key={o}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              // onMouseDown 이다. onClick 이면 input 의 blur 가 먼저 나서
              // 목록이 닫히고 클릭이 허공에 떨어진다.
              onMouseDown={(e) => {
                e.preventDefault();
                pick(o);
              }}
              onMouseEnter={() => setActive(i)}
              className={[
                "cursor-pointer px-3.5 py-2.5 text-base",
                i === active ? "bg-[var(--surface-sunken)]" : "",
              ].join(" ")}
            >
              {o}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
Combobox.__labelable = true;
