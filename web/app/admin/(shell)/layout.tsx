import { NavSidebar } from "./nav-sidebar";

/**
 * 대시보드 셸. 왼쪽 메뉴를 붙인다.
 *
 * `(shell)` 은 라우트 그룹이라 주소에 나타나지 않는다. 이 안에 있는 화면만
 * 메뉴를 받는다.
 *
 * 로그인 화면과 신고 상세는 일부러 밖에 뒀다. 로그인 전에 누를 수 없는
 * 메뉴를 보여줄 이유가 없고, 상세는 Slack 링크로 들어온 사람이 보는
 * 화면이라 그 사람에게는 메뉴가 전부 로그인 화면으로 튕긴다. 눌리지 않는
 * 버튼을 그리는 것보다 없는 게 낫다.
 *
 * 쿼리 파라미터를 보고 클라이언트에서 숨기는 방법도 있지만, 라우트 구조로
 * 정하면 렌더 타이밍에 흔들리지 않는다.
 */
export default function ShellLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-[100dvh]">
      <NavSidebar />
      {/* 폰에서는 상단 바(3.5rem) 아래, 데스크톱에서는 메뉴(15rem) 오른쪽.
          넓은 화면에서 본문을 가운데로 띄우면 메뉴와 내용 사이에 빈 띠가
          생긴다. 왼쪽에 붙여서 눈이 메뉴 옆에서 이어지게 한다. */}
      <div className="pt-14 lg:pl-60 lg:pt-0">
        <div className="mx-auto w-full max-w-3xl lg:mx-0">{children}</div>
      </div>
    </div>
  );
}
