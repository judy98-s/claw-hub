import Link from "next/link";
import { CheckCircle } from "@phosphor-icons/react/dist/ssr";

import { Mascot } from "@/components/mascot";
import type { StoreType } from "@/lib/stores";

/**
 * 8단계 — 접수 완료 (docs/DESIGN-V2.md §6.1).
 *
 * 리스크 평가에서 보류로 분류된 건도 이 화면을 똑같이 본다.
 * 차단 사실을 알려주면 번호를 바꿔가며 우회하는 법을 학습시킨다.
 */

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

/** 업종과 매장 이름만 필요하다. 접수 내용은 다시 보여주지 않는다. */
async function fetchMachine(code: string): Promise<{ storeType: string; storePhone: string }> {
  try {
    const res = await fetch(`${apiOrigin}/api/public/machines/${encodeURIComponent(code)}`, {
      cache: "no-store",
    });
    if (!res.ok) return { storeType: "claw", storePhone: "" };
    return (await res.json()) as { storeType: string; storePhone: string };
  } catch {
    // 접수는 이미 끝났다. 이 조회가 실패해도 접수번호는 보여줘야 한다.
    return { storeType: "claw", storePhone: "" };
  }
}

export default async function DonePage({
  params,
  searchParams,
}: {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ receipt?: string; pay?: string }>;
}) {
  const { code } = await params;
  const { receipt, pay } = await searchParams;
  const { storeType } = await fetchMachine(code);

  /*
    카드 건에 "계좌로 환불해드립니다"라고 쓰면 손님은 오지도 않을 입금을
    기다리고, 하루 뒤에 매장으로 전화한다. 취소는 카드사를 거치므로
    며칠 걸린다는 사실까지 여기서 말해야 그 전화가 오지 않는다.
  */
  const byCard = pay === "card";

  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-md place-items-center px-5 py-10">
      <div className="grid w-full justify-items-center gap-6 text-center">
        <CheckCircle size={64} weight="fill" className="text-ok-600" />

        <div className="grid gap-2">
          <h1 className="text-[27px] font-extrabold leading-tight">접수되었습니다</h1>
          <p className="text-base text-ink-soft">
            {byCard
              ? "사장님이 확인한 뒤 카드 결제를 취소해드립니다. 카드사에 따라 2~5일 걸릴 수 있습니다."
              : "사장님이 확인한 뒤 입력하신 계좌로 환불해드립니다."}
          </p>
        </div>

        {receipt && (
          <div className="grid w-full gap-1.5 rounded-md border-2 border-ink bg-surface p-4">
            <span className="text-sm text-ink-soft">접수번호</span>
            <span className="font-mono text-[28px] font-bold tracking-[0.12em]">{receipt}</span>
            <span className="text-xs text-ink-soft">
              매장에 문의하실 때 이 번호를 알려주세요.
            </span>
          </div>
        )}

        <Mascot store={storeType as StoreType} mood="happy" size={96} />

        <div className="grid w-full gap-3">
          {receipt && (
            <Link
              href={`/r/${code}/status?receipt=${receipt}`}
              className="grid h-14 place-items-center rounded-md border-2 border-ink bg-surface font-bold text-ink"
            >
              처리 상태 보기
            </Link>
          )}
          <Link href={`/r/${code}`} className="py-2 text-sm font-semibold underline">
            다른 문제 신고하기
          </Link>
        </div>
      </div>
    </main>
  );
}
