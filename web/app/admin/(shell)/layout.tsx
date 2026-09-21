import Link from "next/link";
import { ClipboardText, GameController, GearSix, UsersThree } from "@phosphor-icons/react/dist/ssr";

const NAV = [
  { href: "/admin", label: "접수함", Icon: ClipboardText },
  { href: "/admin/machines", label: "기계", Icon: GameController },
  { href: "/admin/contacts", label: "연락처", Icon: UsersThree },
  { href: "/admin/settings", label: "설정", Icon: GearSix },
];

/**
 * 대시보드 셸. 하단 탭 바를 붙인다.
 *
 * `(shell)` 은 라우트 그룹이라 주소에 나타나지 않는다. 이 안에 있는 화면만
 * 탭 바를 받는다.
 *
 * 로그인 화면과 신고 상세는 일부러 밖에 뒀다. 로그인 전에 누를 수 없는 탭을
 * 보여줄 이유가 없고, 상세는 Slack 링크로 들어온 사람이 보는 화면이라 그
 * 사람에게는 탭이 전부 로그인 화면으로 튕긴다. 눌리지 않는 버튼을 그리는
 * 것보다 없는 게 낫다.
 *
 * 쿼리 파라미터를 보고 클라이언트에서 숨기는 방법도 있지만, 라우트 구조로
 * 정하면 렌더 타이밍에 흔들리지 않는다.
 */
export default function ShellLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-[100dvh] pb-[calc(4rem+env(safe-area-inset-bottom))]">
      {children}

      <nav className="fixed inset-x-0 bottom-0 border-t border-[var(--line)] bg-[var(--surface)] pb-[env(safe-area-inset-bottom)]">
        <ul className="mx-auto grid max-w-lg grid-cols-4">
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
