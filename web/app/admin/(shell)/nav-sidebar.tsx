"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ClipboardText,
  GameController,
  GearSix,
  House,
  List,
  Package,
  Storefront,
  UsersThree,
  X,
} from "@phosphor-icons/react";

import { get } from "@/lib/api";

const NAV = [
  { href: "/admin", label: "홈", Icon: House },
  { href: "/admin/claims", label: "접수함", Icon: ClipboardText },
  { href: "/admin/inventory", label: "재고", Icon: Package },
  { href: "/admin/market", label: "장터", Icon: Storefront },
  { href: "/admin/machines", label: "기계", Icon: GameController },
  { href: "/admin/contacts", label: "연락처", Icon: UsersThree },
  { href: "/admin/settings", label: "설정", Icon: GearSix },
];

const BADGE_POLL_MS = 60_000;

/**
 * 왼쪽 메뉴.
 *
 * 화면 폭에 따라 형태가 둘이다. 데스크톱에서는 왼쪽에 계속 붙어 있고,
 * 폰에서는 상단 햄버거로 여는 서랍이다. 폰에서 240px짜리 고정 메뉴는
 * 화면의 60%를 먹고, 아이콘만 남긴 좁은 바는 무슨 메뉴인지 알 수 없다.
 *
 * 메뉴 항목은 같다 — 폭에 따라 항목이 달라지면 사장님이 PC에서 본 메뉴를
 * 폰에서 찾다가 없다고 생각한다.
 */
export function NavSidebar() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [todo, setTodo] = useState(0);

  // 주소가 바뀌면 서랍을 닫는다. 안 닫으면 누른 화면이 서랍에 가려 있다.
  useEffect(() => setOpen(false), [pathname]);

  // Esc로 닫힌다. 서랍을 여는 방법이 하나면 닫는 방법도 하나여야 하는 건
  // 아니다 — 빠져나갈 길은 많을수록 좋다.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  /*
    홈 배지. 다른 화면에 있어도 "처리할 게 남았다"가 보여야 한다.
    실패는 조용히 무시한다 — 배지를 못 불러왔다고 에러를 띄우면,
    정작 중요한 화면의 에러와 섞인다.
  */
  useEffect(() => {
    const load = () =>
      get<{ actions: { severity: string }[] }>("/api/admin/home")
        // 설정 안내(neutral)는 빼고 센다. 빨간 숫자는 "지금 해야 할 일"의
        // 개수여야 한다. 안 그러면 늘 1이 떠 있고, 그 순간 배지는 무시된다.
        .then((h) =>
          setTodo(
            (h.actions ?? []).filter((a) => a.severity !== "neutral").length,
          ),
        )
        .catch(() => {});
    void load();
    const timer = setInterval(load, BADGE_POLL_MS);
    return () => clearInterval(timer);
  }, [pathname]);

  const items = (
    <ul className="grid gap-1 p-3">
      {NAV.map(({ href, label, Icon }) => {
        // `/admin` 은 완전히 같을 때만 현재 항목이다. 접두사로 보면
        // 모든 화면에서 홈이 켜진 것처럼 보인다.
        const on =
          href === "/admin" ? pathname === href : pathname.startsWith(href);
        return (
          <li key={href}>
            <Link
              href={href}
              aria-current={on ? "page" : undefined}
              className={[
                "flex h-12 items-center gap-3 rounded-lg px-3 font-medium",
                "transition-colors duration-150",
                on
                  ? "bg-[var(--tone-accent-bg)] text-[var(--tone-accent-fg)]"
                  : "text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]",
              ].join(" ")}
            >
              <Icon size={22} weight={on ? "fill" : "regular"} />
              <span className="flex-1">{label}</span>
              {href === "/admin" && todo > 0 && (
                <span className="grid h-6 min-w-6 place-items-center rounded-full bg-stop-600 px-1.5 text-xs font-bold tabular-nums text-white">
                  {todo}
                </span>
              )}
            </Link>
          </li>
        );
      })}
    </ul>
  );

  return (
    <>
      {/* 폰: 상단 바 + 햄버거 */}
      <div className="fixed inset-x-0 top-0 z-30 flex h-14 items-center gap-2 border-b border-[var(--line)] bg-[var(--surface)] px-2 lg:hidden">
        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label="메뉴 열기"
          aria-expanded={open}
          className="relative grid size-11 place-items-center rounded-lg text-[var(--fg)]"
        >
          <List size={24} weight="bold" />
          {todo > 0 && (
            <span className="absolute right-1.5 top-1.5 size-2.5 rounded-full bg-stop-600" />
          )}
        </button>
        <span className="font-bold">claw-hub</span>
      </div>

      {open && (
        <div
          className="fixed inset-0 z-40 bg-ink-950/50 lg:hidden"
          onClick={() => setOpen(false)}
          aria-hidden
        />
      )}

      <nav
        aria-label="주요 메뉴"
        className={[
          "fixed inset-y-0 left-0 z-50 w-60 border-r border-[var(--line)] bg-[var(--surface)]",
          "transition-transform duration-200 lg:translate-x-0",
          open ? "translate-x-0" : "-translate-x-full",
        ].join(" ")}
      >
        <div className="flex h-14 items-center justify-between px-4">
          <span className="font-bold">claw-hub</span>
          <button
            type="button"
            onClick={() => setOpen(false)}
            aria-label="메뉴 닫기"
            className="grid size-9 place-items-center rounded-lg lg:hidden"
          >
            <X size={20} weight="bold" />
          </button>
        </div>
        {items}
      </nav>
    </>
  );
}
