import Link from "next/link";

import { StatusView, type PublicStatus } from "./status-view";

/**
 * 9단계 — 처리 상태 (docs/DESIGN-V2.md §6.1).
 *
 * 손님이 접수번호로 자기 건이 어디까지 갔는지 본다. 이 화면이 없으면
 * 손님은 하루 뒤에 매장으로 전화한다.
 */

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

async function fetchStatus(code: string, receipt: string): Promise<PublicStatus | null> {
  const url = `${apiOrigin}/api/public/claims/${encodeURIComponent(receipt)}?machineCode=${encodeURIComponent(code)}`;
  try {
    // 상태는 사장님이 처리하는 대로 바뀐다. 캐시하면 손님이 옛 상태를 본다.
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as PublicStatus;
  } catch {
    return null;
  }
}

export default async function StatusPage({
  params,
  searchParams,
}: {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ receipt?: string }>;
}) {
  const { code } = await params;
  const { receipt } = await searchParams;

  const status = receipt ? await fetchStatus(code, receipt) : null;
  if (!status) return <NotFound code={code} />;

  return <StatusView code={code} status={status} />;
}

/**
 * 접수번호가 틀렸거나 조회에 실패한 경우.
 *
 * 왜 못 찾았는지는 말하지 않는다 — 번호가 없는 건지 기계가 다른 건지
 * 구분해 알려주면 접수번호를 긁어볼 수 있다.
 */
function NotFound({ code }: { code: string }) {
  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-md place-items-center px-5">
      <div className="grid justify-items-center gap-4 text-center">
        <h1 className="text-xl font-bold">접수 내역을 찾을 수 없어요</h1>
        <p className="text-ink-soft">
          접수번호를 다시 확인해주세요.
          <br />
          접수하신 기계의 QR을 다시 찍으면 새로 신고할 수 있습니다.
        </p>
        <Link
          href={`/r/${code}`}
          className="mt-2 grid h-14 place-items-center rounded-md border-2 border-ink bg-action px-6 font-extrabold text-ink shadow-[4px_4px_0_#16130F]"
        >
          신고 화면으로
        </Link>
      </div>
    </main>
  );
}
