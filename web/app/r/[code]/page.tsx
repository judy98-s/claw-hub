import { ClaimForm } from "./claim-form";

type MachineInfo = {
  code: string;
  label: string;
  storeName: string;
  storePhone: string;
  reviewThresholdKrw: number;
  maxAmountKrw: number;
  maxPhotos: number;
};

type Bank = { code: string; name: string };

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

async function fetchMachine(code: string): Promise<MachineInfo | { storePhone: string } | null> {
  const res = await fetch(`${apiOrigin}/api/public/machines/${encodeURIComponent(code)}`, {
    cache: "no-store",
  });
  if (res.status === 404) return (await res.json().catch(() => null)) as { storePhone: string };
  if (!res.ok) return null;
  return res.json();
}

async function fetchBanks(): Promise<Bank[]> {
  const res = await fetch(`${apiOrigin}/api/public/banks`, { next: { revalidate: 3600 } });
  if (!res.ok) return [];
  const body = (await res.json()) as { banks: Bank[] };
  return body.banks;
}

export default async function ReportPage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  const [machine, banks] = await Promise.all([fetchMachine(code), fetchBanks()]);

  if (!machine || !("label" in machine)) {
    const phone = machine && "storePhone" in machine ? machine.storePhone : "";
    return <MachineNotFound storePhone={phone} />;
  }

  return <ClaimForm machine={machine} banks={banks} />;
}

/**
 * 스티커가 떼여 다른 곳에 붙었거나 기계가 폐기된 경우.
 *
 * 빈 화면이나 에러 코드를 띄우면 손님은 여기서 끝이다. 전화할 곳을 준다.
 */
function MachineNotFound({ storePhone }: { storePhone: string }) {
  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-md place-items-center px-4">
      <div className="grid gap-4 text-center">
        <h1 className="text-xl font-bold">이 스티커의 기계를 찾을 수 없어요</h1>
        <p className="text-[var(--muted)]">
          스티커가 떨어졌거나 기계가 교체되었을 수 있습니다.
          <br />
          매장에 직접 연락해주세요.
        </p>
        {storePhone && (
          <a
            href={`tel:${storePhone}`}
            className="mt-2 inline-flex h-14 items-center justify-center rounded-lg bg-accent-600 px-6 font-semibold text-white"
          >
            매장에 전화하기
          </a>
        )}
      </div>
    </main>
  );
}
