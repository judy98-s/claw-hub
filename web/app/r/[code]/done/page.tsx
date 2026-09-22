import Link from "next/link";
import { CheckCircle } from "@phosphor-icons/react/dist/ssr";

/**
 * 접수 완료 화면.
 *
 * 리스크 평가에서 보류로 분류된 건도 이 화면을 똑같이 본다.
 * 차단 사실을 알려주면 번호를 바꿔가며 우회하는 법을 학습시킨다.
 */
export default async function DonePage({
  params,
  searchParams,
}: {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ receipt?: string; pay?: string }>;
}) {
  const { code } = await params;
  const { receipt, pay } = await searchParams;

  /*
    카드 건에 "계좌로 환불해드립니다"라고 쓰면 손님은 오지도 않을 입금을
    기다리고, 하루 뒤에 매장으로 전화한다. 취소는 카드사를 거치므로
    며칠 걸린다는 사실까지 여기서 말해야 그 전화가 오지 않는다.
  */
  const byCard = pay === "card";

  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-md place-items-center px-4 py-10">
      <div className="grid justify-items-center gap-5 text-center">
        <CheckCircle size={56} weight="fill" className="text-ok-600" />

        <div className="grid gap-2">
          <h1 className="text-xl font-bold">접수되었습니다</h1>
          <p className="text-[var(--muted)]">
            {byCard
              ? "사장님이 확인한 뒤 카드 결제를 취소해드립니다. 카드사에 따라 2~5일 걸릴 수 있습니다."
              : "사장님이 확인한 뒤 입력하신 계좌로 환불해드립니다."}
          </p>
        </div>

        {receipt && (
          <div className="surface grid gap-1 px-6 py-4">
            <span className="text-sm text-[var(--muted)]">접수번호</span>
            <span className="font-mono text-2xl font-bold tracking-wider">
              {receipt}
            </span>
          </div>
        )}

        <p className="text-sm text-[var(--muted)]">
          매장에 문의하실 때 이 번호를 말씀해주세요.
        </p>

        <Link
          href={`/r/${code}`}
          className="mt-2 text-sm font-semibold text-accent-600 underline underline-offset-4"
        >
          다른 문제 신고하기
        </Link>
      </div>
    </main>
  );
}
