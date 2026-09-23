"use client";

import { useState } from "react";
import { X } from "@phosphor-icons/react";

import { ApiError, post } from "@/lib/api";
import { krw } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/field";

/** 재고에서 바로 내놓을 때 넘어오는 값. 비어 있으면 빈 칸에서 시작한다. */
export type FromInventory = {
  name: string;
  qtyOnHand: number;
  avgUnitCostKrw: number;
};

const KINDS = [
  { value: "sell", label: "판매", hint: "돈을 받고 넘길게요" },
  { value: "swap", label: "교환", hint: "다른 인형과 바꿀래요" },
  { value: "both", label: "둘 다", hint: "판매도 교환도 좋아요" },
] as const;

/**
 * 장터에 내놓기.
 *
 * 재고에서 들어오면 품명·보유 수량·원가가 이미 있다. 사장님이 새로
 * 정하는 건 거래 방식, 내놓을 수량, 희망 단가 셋뿐이다.
 */
export function ListingSheet({
  from,
  onClose,
  onPosted,
}: {
  from?: FromInventory;
  onClose: () => void;
  onPosted: () => void;
}) {
  const [name, setName] = useState(from?.name ?? "");
  const [kind, setKind] = useState<string>("sell");
  const [qty, setQty] = useState(
    from ? String(Math.max(from.qtyOnHand, 1)) : "",
  );
  const [price, setPrice] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const needsPrice = kind !== "swap";
  const qtyNum = Number(qty || 0);
  const priceNum = Number(price || 0);
  const ready =
    name.trim() !== "" && qtyNum > 0 && (!needsPrice || priceNum > 0) && !busy;

  // 원가보다 싸게 내놓는 걸 막지는 않는다. 안 나가는 재고를 털어내는 게
  // 목적일 때는 그게 정상이다. 다만 모르고 그러는 일은 없게 한다.
  const belowCost =
    from && needsPrice && priceNum > 0 && priceNum < from.avgUnitCostKrw;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ready) return;
    setBusy(true);
    setError("");
    try {
      await post("/api/admin/market", {
        name: name.trim(),
        kind,
        qty: qtyNum,
        unitPriceKrw: needsPrice ? priceNum : 0,
        note: note.trim(),
      });
      onPosted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "올리지 못했습니다.");
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50 sm:items-center">
      <div className="max-h-[92dvh] w-full max-w-lg overflow-y-auto rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] sm:rounded-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold">장터에 내놓기</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="닫기"
            className="p-1"
          >
            <X size={20} weight="bold" />
          </button>
        </div>

        <form onSubmit={submit} className="grid gap-4">
          <Field
            label="인형"
            required
            hint={
              from
                ? `${from.qtyOnHand}개 보유 · 내 원가 개당 ${krw(from.avgUnitCostKrw)}`
                : undefined
            }
          >
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="예) 쿠로미 중형 30cm"
              maxLength={80}
            />
          </Field>

          <Field label="거래 방식" required>
            <div className="grid grid-cols-3 gap-2">
              {KINDS.map((k) => {
                const on = kind === k.value;
                return (
                  <button
                    key={k.value}
                    type="button"
                    onClick={() => setKind(k.value)}
                    aria-pressed={on}
                    className={[
                      "grid min-h-16 content-center gap-0.5 rounded-lg border p-2 text-center",
                      "transition-colors duration-150",
                      on
                        ? "border-accent-600 bg-[var(--tone-accent-bg)] text-[var(--tone-accent-fg)]"
                        : "border-[var(--line)] bg-[var(--surface)]",
                    ].join(" ")}
                  >
                    <span className="font-semibold">{k.label}</span>
                    <span
                      className={`text-[11px] leading-tight ${on ? "text-[var(--tone-accent-fg)]" : "text-[var(--muted)]"}`}
                    >
                      {k.hint}
                    </span>
                  </button>
                );
              })}
            </div>
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field label="내놓을 수량" required>
              <Input
                inputMode="numeric"
                value={qty}
                onChange={(e) => setQty(e.target.value.replace(/[^0-9]/g, ""))}
                placeholder="40"
              />
            </Field>
            {needsPrice ? (
              <Field label="희망 단가" required>
                <Input
                  inputMode="numeric"
                  value={price}
                  onChange={(e) =>
                    setPrice(e.target.value.replace(/[^0-9]/g, ""))
                  }
                  placeholder="1800"
                />
              </Field>
            ) : (
              <div className="grid content-end pb-3 text-sm text-[var(--muted)]">
                교환만 원하시면 가격은 적지 않습니다.
              </div>
            )}
          </div>

          {belowCost && (
            <p className="rounded-lg bg-[var(--tone-warn-bg)] p-3.5 text-sm text-[var(--tone-warn-fg)]">
              내 원가({krw(from!.avgUnitCostKrw)})보다 개당{" "}
              <b>{krw(from!.avgUnitCostKrw - priceNum)}</b> 싸게 내놓는
              값입니다.
            </p>
          )}

          <Field
            label="한마디"
            hint="상태나 조건을 적어주세요. 선택 사항입니다."
          >
            <Textarea
              rows={2}
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="예) 작년 물량이라 태그 있어요. 직거래만"
              maxLength={300}
            />
          </Field>

          {error && (
            <p
              role="alert"
              className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
            >
              {error}
            </p>
          )}

          <Button type="submit" size="lg" full loading={busy} disabled={!ready}>
            올리기
          </Button>
          <p className="text-center text-xs text-[var(--muted)]">
            30일 뒤 자동으로 내려갑니다. 연락처는 상대가 버튼을 눌렀을 때만
            전달됩니다.
          </p>
        </form>
      </div>
    </div>
  );
}
