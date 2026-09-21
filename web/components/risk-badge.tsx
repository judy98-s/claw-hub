"use client";

import { useState } from "react";
import { CaretDown, ShieldWarning } from "@phosphor-icons/react";

export type RiskReason = { code: string; message: string };

/**
 * 리스크 사유 표시.
 *
 * 점수를 보여주지 않는다. "위험도 72점"으로는 사장님이 승인할지 전화를
 * 걸지 판단할 수 없다. 발동한 규칙의 문장을 그대로 보여준다.
 */
export function RiskReasons({ reasons }: { reasons: RiskReason[] }) {
  const [open, setOpen] = useState(false);
  if (reasons.length === 0) return null;

  const first = reasons[0];
  const rest = reasons.length - 1;

  return (
    <div className="rounded-lg bg-[var(--tone-warn-bg)] p-3 text-[var(--tone-warn-fg)]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        disabled={rest === 0}
        aria-expanded={open}
        className="flex w-full items-start gap-2 text-left"
      >
        <ShieldWarning size={18} weight="fill" className="mt-0.5 shrink-0" />
        <span className="flex-1 text-sm font-medium">
          {open ? "확인이 필요한 이유" : first.message}
        </span>
        {rest > 0 && (
          <span className="flex shrink-0 items-center gap-1 text-xs font-semibold">
            {open ? "접기" : `+${rest}`}
            <CaretDown
              size={12}
              weight="bold"
              className={open ? "rotate-180 transition-transform" : "transition-transform"}
            />
          </span>
        )}
      </button>

      {open && (
        <ul className="mt-2 grid gap-1.5 pl-6">
          {reasons.map((r) => (
            <li key={r.code} className="text-sm">
              • {r.message}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
