import Link from "next/link";
import { ClipboardText, GameController, UsersThree } from "@phosphor-icons/react/dist/ssr";

const NAV = [
  { href: "/admin", label: "접수함", Icon: ClipboardText },
  { href: "/admin/machines", label: "기계", Icon: GameController },
  { href: "/admin/contacts", label: "연락처", Icon: UsersThree },
];

/**
 * 대시보드 셸.
 *
 * 하단 탭 바다. 사장님은 매장에서 폰으로 이걸 본다 — 사이드바는 좁은
 * 화면에서 접히고, 접힌 메뉴는 안 눌린다.
 */
export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-[100dvh] pb-[calc(4rem+env(safe-area-inset-bottom))]">
      {children}

      <nav className="fixed inset-x-0 bottom-0 border-t border-[var(--line)] bg-[var(--surface)] pb-[env(safe-area-inset-bottom)]">
        <ul className="mx-auto grid max-w-lg grid-cols-3">
          {NAV.map(({ href, label, Icon }) => (
            <li key={href}>
              <Link
                href={href}
                className="grid h-16 place-items-center gap-0.5 text-[var(--muted)] hover:text-[var(--fg)]"
              >
                <Icon size={22} weight="regular" />
                <span className="text-xs font-medium">{label}</span>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
