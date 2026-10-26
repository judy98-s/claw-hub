"use client";

import { useCallback, useEffect, useState } from "react";
import { DownloadSimple, Wrench } from "@phosphor-icons/react";

import { get } from "@/lib/api";
import { krw } from "@/lib/format";

/**
 * 정산/통계 (docs/DESIGN-V2.md §6.2 ④).
 *
 * 사장님이 "이번 달에 얼마 나갔나"를 보는 화면이다. 기간 경계는 서버가
 * 정한다 — 프런트와 서버가 각자 계산하면 월말과 윤년에서 어긋나고,
 * 어긋난 걸 알아챌 방법이 없다.
 */

type Settlement = {
  period: string;
  from: string;
  to: string;
  claimCount: number;
  paidCount: number;
  paidTotalKrw: number;
  prevClaimCount: number;
  prevPaidCount: number;
  prevPaidTotalKrw: number;
  days: { day: string; claimCount: number; paidTotalKrw: number }[];
  machines: {
    machineId: string;
    label: string;
    claimCount: number;
    paidTotalKrw: number;
    needsCheck: boolean;
  }[];
  paidByCash: number;
  paidByCard: number;
  rejectedCount: number;
  openCount: number;
};

const PERIODS = [
  { key: "week", label: "이번 주" },
  { key: "month", label: "이번 달" },
  { key: "last_month", label: "지난달" },
] as const;

export default function StatsPage() {
  const [period, setPeriod] = useState<string>("week");
  const [data, setData] = useState<Settlement | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async (p: string) => {
    setError("");
    try {
      setData(await get<Settlement>(`/api/admin/stats/settlement?period=${p}`));
    } catch {
      setError("집계를 불러오지 못했습니다.");
    }
  }, []);

  useEffect(() => {
    void load(period);
  }, [period, load]);

  return (
    <div className="grid gap-4 p-4">
      <div className="flex items-center justify-between gap-2">
        <h1 className="text-xl font-bold">정산</h1>
        <a
          href={`/api/admin/stats/settlement.csv?period=${period}`}
          className="flex h-9 items-center gap-1.5 rounded-md border-2 border-ink bg-surface px-3 text-sm font-semibold"
        >
          <DownloadSimple size={16} weight="bold" />
          엑셀로 받기
        </a>
      </div>

      <div className="grid grid-cols-3 gap-2">
        {PERIODS.map((p) => (
          <button
            key={p.key}
            type="button"
            onClick={() => setPeriod(p.key)}
            aria-pressed={period === p.key}
            className={[
              "h-9 rounded-md border-2 border-ink text-sm font-semibold",
              period === p.key ? "bg-action text-ink" : "bg-surface text-ink",
            ].join(" ")}
          >
            {p.label}
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-stop-600">{error}</p>}

      {data && (
        <>
          <p className="text-sm text-ink-soft">
            {data.from} ~ {data.to}
          </p>

          <div className="grid grid-cols-3 gap-2">
            <Stat label="접수" value={`${data.claimCount}건`} delta={data.claimCount - data.prevClaimCount} />
            <Stat label="환불" value={`${data.paidCount}건`} delta={data.paidCount - data.prevPaidCount} />
            <Stat
              label="나간 금액"
              value={krw(data.paidTotalKrw)}
              delta={data.paidTotalKrw - data.prevPaidTotalKrw}
              money
            />
          </div>

          <DayChart days={data.days} />

          <section className="grid gap-2">
            <h2 className="text-sm font-bold">기계별</h2>
            <MachineTable machines={data.machines} />
          </section>

          <section className="grid grid-cols-2 gap-2">
            <Stat label="계좌 송금" value={`${data.paidByCash}건`} />
            <Stat label="카드 취소" value={`${data.paidByCard}건`} />
            <Stat label="거절" value={`${data.rejectedCount}건`} />
            <Stat label="대기" value={`${data.openCount}건`} />
          </section>
        </>
      )}
    </div>
  );
}

/**
 * 요약 칸.
 *
 * 증가가 warn(주황), 감소가 ok(초록)다. 매출 지표와 반대인데 맞다 —
 * 환불액은 늘면 나쁜 숫자다. 나중에 매출을 붙일 때 이 색을 그대로
 * 가져다 쓰면 거꾸로 보이므로, 그때 토큰을 나눠야 한다.
 */
function Stat({
  label,
  value,
  delta,
  money,
}: {
  label: string;
  value: string;
  delta?: number;
  money?: boolean;
}) {
  return (
    <div className="grid gap-0.5 rounded-md border-2 border-ink bg-surface p-3">
      <span className="text-xs text-ink-soft">{label}</span>
      <span className="text-lg font-bold tabular-nums">{value}</span>
      {delta !== undefined && delta !== 0 && (
        <span className={`text-xs font-semibold ${delta > 0 ? "text-warn-600" : "text-ok-600"}`}>
          {delta > 0 ? "▲" : "▼"} {money ? krw(Math.abs(delta)) : Math.abs(delta)}
        </span>
      )}
      {delta === 0 && <span className="text-xs text-ink-soft">전기간과 같음</span>}
    </div>
  );
}

/**
 * 날짜별 나간 금액 막대.
 *
 * 전부 0 이면 막대를 그리지 않는다. 최고값으로 나누면 0/0 이 되고,
 * "아무 일도 없었다"가 "전부 최고치"로 보인다.
 */
function DayChart({ days }: { days: Settlement["days"] }) {
  const max = Math.max(...days.map((d) => d.paidTotalKrw), 0);
  if (days.length === 0) return null;

  // 390px 폭에 한 달치를 그리면 칸이 12px 남짓이라 "26" 이 두 줄로 쪼개진다.
  // 들어갈 만큼만 띄엄띄엄 쓴다. 첫 날과 마지막 날은 항상 쓴다 — 기간이
  // 어디서 시작하고 끝나는지는 보여야 한다.
  const every = days.length > 14 ? 5 : days.length > 8 ? 2 : 1;
  const showLabel = (i: number) => i === 0 || i === days.length - 1 || i % every === 0;

  return (
    <section className="grid gap-2 rounded-md border-2 border-ink bg-surface p-3">
      <h2 className="text-sm font-bold">날짜별 나간 금액</h2>
      {max === 0 ? (
        <p className="py-4 text-center text-sm text-ink-soft">이 기간에 나간 금액이 없습니다.</p>
      ) : (
        <div className="flex h-28 items-end gap-1">
          {days.map((d, i) => {
            const h = (d.paidTotalKrw / max) * 100;
            const top = d.paidTotalKrw === max;
            return (
              <div
                key={d.day}
                className="grid min-w-0 flex-1 justify-items-center gap-1"
                title={`${d.day} ${krw(d.paidTotalKrw)}`}
              >
                <div className="flex h-24 w-full items-end">
                  <div
                    className={`w-full rounded-sm ${top ? "bg-seam" : "bg-pink"}`}
                    style={{ height: `${Math.max(h, d.paidTotalKrw > 0 ? 4 : 0)}%` }}
                  />
                </div>
                <span className="h-3 whitespace-nowrap text-[10px] leading-3 text-ink-soft">
                  {showLabel(i) ? Number(d.day.slice(8)) : ""}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

function MachineTable({ machines }: { machines: Settlement["machines"] }) {
  const max = Math.max(...machines.map((m) => m.paidTotalKrw), 0);
  if (machines.length === 0) {
    return (
      <p className="rounded-md border-2 border-ink bg-surface p-4 text-center text-sm text-ink-soft">
        등록된 기계가 없습니다.
      </p>
    );
  }
  return (
    <div className="divide-y-2 divide-[#F2F0EC] overflow-hidden rounded-md border-2 border-ink bg-surface">
      {machines.map((m) => (
        <div key={m.machineId} className="grid gap-1.5 p-3">
          <div className="flex items-center gap-2">
            {m.needsCheck && <Wrench size={15} weight="bold" className="shrink-0 text-stop-600" />}
            <span className="min-w-0 flex-1 truncate text-[15px] font-bold">{m.label}</span>
            <span className="shrink-0 text-sm text-ink-soft">{m.claimCount}건</span>
            <span className="shrink-0 text-[15px] font-bold tabular-nums">{krw(m.paidTotalKrw)}</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-sm bg-paper-sunken">
            <div
              className={`h-full ${m.needsCheck ? "bg-stop-600" : "bg-pink"}`}
              style={{ width: max > 0 ? `${(m.paidTotalKrw / max) * 100}%` : "0%" }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
