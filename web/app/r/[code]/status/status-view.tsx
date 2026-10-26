import Link from "next/link";
import { Check, Phone, Prohibit } from "@phosphor-icons/react/dist/ssr";

import { Mascot, type MascotMood } from "@/components/mascot";
import { krw } from "@/lib/format";
import type { StoreType } from "@/lib/stores";

/** 서버가 내려주는 것 전부. 개인정보는 들어 있지 않다 (§7). */
export type PublicStatus = {
  receiptCode: string;
  /**
   * 서버가 6개 status 를 4개로 줄여 내려준다. 특히 리스크 보류(on_hold)가
   * waiting 에 섞여 있어서 손님은 보류 여부를 알 수 없다 — 알려주면
   * 번호를 바꿔가며 우회하는 법을 학습시킨다.
   */
  stage: "waiting" | "approved" | "done" | "rejected";
  issueLabel: string;
  amountKrw: number;
  paymentMethod: string;
  storeName: string;
  storePhone: string;
  storeType: string;
  receivedAt: string;
  reviewedAt?: string;
  settledAt?: string;
};

type StageInfo = {
  title: string;
  mood: MascotMood;
  hint: string;
};

function infoFor(s: PublicStatus): StageInfo {
  const byCard = s.paymentMethod === "card";
  switch (s.stage) {
    case "approved":
      return {
        title: "승인되었어요",
        mood: "calm",
        hint: byCard
          ? "사장님이 카드 결제를 취소하는 중입니다. 카드사에 따라 2~5일 걸릴 수 있어요."
          : "사장님이 곧 입력하신 계좌로 보내드립니다.",
      };
    case "done":
      return {
        title: byCard ? "결제가 취소되었어요" : "환불이 끝났어요",
        mood: "happy",
        hint: byCard
          ? "카드사 처리에 며칠 더 걸릴 수 있습니다. 카드 명세서에서 확인해주세요."
          : "입금되지 않았다면 매장에 문의해주세요.",
      };
    case "rejected":
      return {
        title: "환불이 어렵습니다",
        mood: "worried",
        hint: "자세한 사정은 매장에 문의해주세요.",
      };
    default:
      return {
        title: "확인을 기다리고 있어요",
        mood: "calm",
        hint: "사장님이 확인하면 알려드립니다. 보통 하루 안에 처리됩니다.",
      };
  }
}

export function StatusView({ code, status }: { code: string; status: PublicStatus }) {
  const info = infoFor(status);
  const byCard = status.paymentMethod === "card";
  const rejected = status.stage === "rejected";

  // 3단 타임라인. 거절이면 3단은 숨긴다 — 일어나지 않을 일이다.
  const steps = [
    { label: "접수됨", at: status.receivedAt, state: "done" as const },
    {
      label: rejected ? "환불 불가" : "사장님 확인",
      at: status.reviewedAt,
      state: rejected ? ("stop" as const) : status.reviewedAt ? ("done" as const) : ("now" as const),
    },
    ...(rejected
      ? []
      : [
          {
            label: byCard ? "카드 결제 취소" : "계좌로 환불",
            at: status.settledAt,
            state: status.settledAt ? ("done" as const) : ("next" as const),
          },
        ]),
  ];

  return (
    <main className="mx-auto min-h-[100dvh] max-w-md px-5 py-8">
      <div className="grid gap-6">
        <div className="grid justify-items-center gap-3 text-center">
          <Mascot store={status.storeType as StoreType} mood={info.mood} size={56} />
          <h1 className="text-[27px] font-extrabold leading-tight">{info.title}</h1>
          <p className="text-base text-ink-soft">{info.hint}</p>
        </div>

        <ol className="grid gap-0">
          {steps.map((s, i) => (
            <li key={s.label} className="grid grid-cols-[32px_1fr] gap-3">
              <div className="grid justify-items-center">
                <Dot state={s.state} />
                {i < steps.length - 1 && (
                  <div
                    className={`w-0.5 flex-1 ${s.state === "done" ? "bg-ink" : "bg-paper-sunken"}`}
                    style={{ minHeight: 28 }}
                  />
                )}
              </div>
              <div className={`pb-5 ${s.state === "next" ? "text-ink-soft" : ""}`}>
                <p className="font-bold">{s.label}</p>
                {s.at && <p className="text-sm text-ink-soft">{formatStamp(s.at)}</p>}
              </div>
            </li>
          ))}
        </ol>

        <div className="grid gap-0 divide-y-2 divide-[#F2F0EC] overflow-hidden rounded-md border-2 border-ink bg-surface">
          <Row label="접수번호" value={status.receiptCode} mono />
          <Row label="증상" value={status.issueLabel} />
          <Row label="금액" value={krw(status.amountKrw)} />
          <Row label="결제" value={byCard ? "카드" : "현금"} />
        </div>

        {status.storePhone && (
          <a
            href={`tel:${status.storePhone}`}
            className="flex h-14 items-center justify-center gap-2 rounded-md border-2 border-ink bg-surface font-bold text-ink"
          >
            <Phone size={20} weight="bold" />
            매장에 전화하기
          </a>
        )}

        <Link href={`/r/${code}`} className="justify-self-center py-2 text-sm font-semibold underline">
          다른 문제 신고하기
        </Link>
      </div>
    </main>
  );
}

function Dot({ state }: { state: "done" | "now" | "next" | "stop" }) {
  if (state === "done") {
    return (
      <span className="grid size-8 shrink-0 place-items-center rounded-full border-2 border-ink bg-action">
        <Check size={16} weight="bold" />
      </span>
    );
  }
  if (state === "stop") {
    return (
      <span className="grid size-8 shrink-0 place-items-center rounded-full border-2 border-stop-600 bg-surface text-stop-600">
        <Prohibit size={16} weight="bold" />
      </span>
    );
  }
  if (state === "now") {
    // 지금 여기. 채운 점으로 "진행 중"을 보여준다.
    return (
      <span className="grid size-8 shrink-0 place-items-center rounded-full border-2 border-ink bg-pink-soft">
        <span className="size-2.5 rounded-full bg-ink" />
      </span>
    );
  }
  return <span className="size-8 shrink-0 rounded-full border-2 border-paper-sunken bg-surface" />;
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center gap-3 px-3" style={{ minHeight: 48 }}>
      <span className="w-20 shrink-0 text-sm text-ink-soft">{label}</span>
      <span className={`min-w-0 flex-1 truncate font-semibold ${mono ? "font-mono tracking-wider" : ""}`}>
        {value}
      </span>
    </div>
  );
}

/**
 * "10월 26일 오후 1시 32분".
 *
 * 서버가 분 단위 RFC3339 로 준다. 로케일 함수에 맡기지 않는 이유는
 * 서버 렌더와 브라우저의 시간대가 다르면 글자가 바뀌어 hydration 이
 * 깨지기 때문이다. 한국 매장만 쓰므로 KST 로 고정해 직접 만든다.
 */
function formatStamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const kst = new Date(d.getTime() + 9 * 60 * 60 * 1000);
  const month = kst.getUTCMonth() + 1;
  const day = kst.getUTCDate();
  const h24 = kst.getUTCHours();
  const min = kst.getUTCMinutes();
  const ampm = h24 < 12 ? "오전" : "오후";
  const h12 = h24 % 12 === 0 ? 12 : h24 % 12;
  return `${month}월 ${day}일 ${ampm} ${h12}시 ${String(min).padStart(2, "0")}분`;
}
