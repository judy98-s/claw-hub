"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Camera, Trash, X } from "@phosphor-icons/react";

import { ApiError, get, postForm } from "@/lib/api";
import { shrinkImage } from "@/lib/image";
import { krw } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Combobox } from "@/components/ui/combobox";
import { Field, Input } from "@/components/ui/field";
import { PreviewImage } from "@/components/ui/preview-image";

type Suggestions = { names: string[]; vendors: string[] };

type Created = {
  unitCostKrw: number;
  totalKrw: number;
  qtyOnHand: number;
  previousUnitCostKrw: number;
  existing: boolean;
};

/** 오늘 날짜를 <input type="date"> 가 쓰는 형식으로. 매장 시계 기준이다. */
function today(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/**
 * 사입 기록 시트.
 *
 * 30초를 넘기면 안 된다. 넘기면 아무도 두 번째 건을 넣지 않고, 그러면
 * 이 기능 전체가 없는 것과 같다. 그래서 필수 칸은 이름·단가·수량 셋뿐이고
 * 나머지는 전부 선택이거나 기본값이 있다.
 */
export function PurchaseSheet({
  onClose,
  onSaved,
}: {
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState("");
  const [vendor, setVendor] = useState("");
  const [unitPrice, setUnitPrice] = useState("");
  const [qty, setQty] = useState("");
  const [shipping, setShipping] = useState("");
  const [purchasedAt, setPurchasedAt] = useState(today);
  const [receipt, setReceipt] = useState<File | null>(null);

  const [sug, setSug] = useState<Suggestions>({ names: [], vendors: [] });
  const [lastCost, setLastCost] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState<Created | null>(null);

  /**
   * 멱등키는 시트가 열릴 때 한 번 만든다. 제출마다 새로 만들면 더블탭이
   * 두 건으로 들어가고, 그 오차는 재고를 세어보기 전까지 안 드러난다.
   * 저장에 성공하면 새로 만든다 — 다음 건은 다른 건이니까.
   */
  const [idemKey, setIdemKey] = useState(() => newKey());

  const load = useCallback(async (q: string) => {
    try {
      setSug(
        await get<Suggestions>(
          `/api/admin/suggestions?q=${encodeURIComponent(q)}`,
        ),
      );
    } catch {
      // 자동완성이 안 뜨는 건 등록을 막을 이유가 아니다. 조용히 넘어간다.
    }
  }, []);

  useEffect(() => {
    void load("");
  }, [load]);

  // 이름을 칠 때마다 후보를 다시 받는다. 250ms 쉬었을 때만 — 한 글자마다
  // 요청을 보내면 느린 회선에서 응답이 순서 없이 도착한다.
  useEffect(() => {
    const t = setTimeout(() => void load(name), 250);
    return () => clearTimeout(t);
  }, [name, load]);

  // 고른 인형에 과거 기록이 있으면 직전 단가를 띄운다. 이게 있어야
  // "지난번보다 비싸게 샀다"를 등록하기 전에 안다.
  useEffect(() => {
    if (!name.trim()) {
      setLastCost(null);
      return;
    }
    let alive = true;
    const t = setTimeout(async () => {
      try {
        const res = await get<{ purchases: { unitCostKrw: number }[] }>(
          `/api/admin/purchases?name=${encodeURIComponent(name)}&limit=1`,
        );
        if (alive) setLastCost(res.purchases?.[0]?.unitCostKrw ?? null);
      } catch {
        if (alive) setLastCost(null);
      }
    }, 350);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [name]);

  const priceNum = Number(unitPrice || 0);
  const qtyNum = Number(qty || 0);
  const shipNum = Number(shipping || 0);

  // 입력하는 동안 개당 실단가를 보여준다. 배송비를 왜 적는지가 그때 보인다.
  // 서버와 같은 반올림을 쓴다 — 다르면 등록 직후 숫자가 1원 바뀐다.
  const preview = useMemo(() => {
    if (qtyNum <= 0) return null;
    const total = priceNum * qtyNum + shipNum;
    return {
      unit: Math.floor((total + Math.floor(qtyNum / 2)) / qtyNum),
      total,
    };
  }, [priceNum, qtyNum, shipNum]);

  const ready = name.trim() !== "" && qtyNum > 0 && unitPrice !== "" && !saving;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ready) return;
    setError("");
    setSaving(true);

    const form = new FormData();
    form.set("name", name);
    form.set("vendor", vendor);
    form.set("unitPriceKrw", unitPrice);
    form.set("qty", qty);
    form.set("shippingKrw", shipping || "0");
    form.set("purchasedAt", purchasedAt);
    if (receipt) form.append("photos", receipt);

    try {
      const res = await postForm<Created>(
        "/api/admin/purchases",
        form,
        idemKey,
      );
      setDone(res);
      onSaved();
    } catch (err) {
      // 입력값은 그대로 둔다. 다 지우고 다시 쓰게 하면 그 자리에서 포기한다.
      setError(
        err instanceof ApiError
          ? err.message
          : "저장하지 못했습니다. 다시 시도해주세요.",
      );
      setSaving(false);
    }
  }

  function again() {
    setDone(null);
    setName("");
    setUnitPrice("");
    setQty("");
    setShipping("");
    setReceipt(null);
    setIdemKey(newKey());
    setSaving(false);
    void load("");
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50 sm:items-center">
      <div className="max-h-[92dvh] w-full max-w-lg overflow-y-auto rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] sm:rounded-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold">
            {done ? "기록했습니다" : "사입 기록"}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="닫기"
            className="p-1"
          >
            <X size={20} weight="bold" />
          </button>
        </div>

        {done ? (
          <Saved result={done} onAgain={again} onClose={onClose} />
        ) : (
          <form onSubmit={submit} className="grid gap-4">
            <Field
              label="인형"
              required
              hint={
                lastCost !== null
                  ? `지난번엔 개당 ${krw(lastCost)}에 사셨어요`
                  : "전에 적은 이름은 치는 동안 아래에 뜹니다."
              }
            >
              <Combobox
                value={name}
                onChange={setName}
                options={sug.names}
                placeholder="예) 쿠로미 중형 30cm"
                maxLength={80}
              />
            </Field>

            <Field label="어디서" hint="선택 사항입니다.">
              <div className="grid gap-2">
                {sug.vendors.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {sug.vendors.map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setVendor(vendor === v ? "" : v)}
                        aria-pressed={vendor === v}
                        className={[
                          "h-10 rounded-lg border px-3 text-sm font-medium",
                          "transition-colors duration-150",
                          vendor === v
                            ? "border-accent-600 bg-[var(--tone-accent-bg)] text-[var(--tone-accent-fg)]"
                            : "border-[var(--line)] bg-[var(--surface)]",
                        ].join(" ")}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                )}
                <Input
                  type="text"
                  value={vendor}
                  onChange={(e) => setVendor(e.target.value)}
                  placeholder="캐치돌, 도매꾹, 알리…"
                  maxLength={80}
                />
              </div>
            </Field>

            <div className="grid grid-cols-2 gap-3">
              <Field label="개당 단가" required>
                <Input
                  type="text"
                  inputMode="numeric"
                  value={unitPrice}
                  onChange={(e) => setUnitPrice(digits(e.target.value))}
                  placeholder="2300"
                />
              </Field>
              <Field label="수량" required>
                <Input
                  type="text"
                  inputMode="numeric"
                  value={qty}
                  onChange={(e) => setQty(digits(e.target.value))}
                  placeholder="60"
                />
              </Field>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <Field label="배송비" hint="선택">
                <Input
                  type="text"
                  inputMode="numeric"
                  value={shipping}
                  onChange={(e) => setShipping(digits(e.target.value))}
                  placeholder="0"
                />
              </Field>
              <Field label="사입한 날">
                <Input
                  type="date"
                  value={purchasedAt}
                  max={today()}
                  onChange={(e) => setPurchasedAt(e.target.value)}
                />
              </Field>
            </div>

            {preview && (
              <dl className="flex items-baseline justify-between rounded-lg bg-[var(--surface-sunken)] px-4 py-3">
                <dt className="text-sm text-[var(--muted)]">개당 실단가</dt>
                <dd className="text-right">
                  <span className="text-xl font-bold tabular-nums">
                    {krw(preview.unit)}
                  </span>
                  <span className="ml-2 text-sm text-[var(--muted)] tabular-nums">
                    합계 {krw(preview.total)}
                  </span>
                </dd>
              </dl>
            )}

            <Field
              label="영수증"
              hint="선택 사항입니다. 나중에 단가를 확인할 때 씁니다."
            >
              {receipt ? (
                <div className="flex items-center gap-2 rounded-lg border border-[var(--line)] p-2.5">
                  <PreviewImage
                    file={receipt}
                    alt="첨부한 영수증"
                    className="size-12 rounded object-cover"
                  />
                  <span className="flex-1 truncate text-sm text-[var(--muted)]">
                    {receipt.name}
                  </span>
                  <button
                    type="button"
                    onClick={() => setReceipt(null)}
                    aria-label="영수증 삭제"
                    className="grid size-9 place-items-center rounded-lg"
                  >
                    <Trash size={16} weight="bold" />
                  </button>
                </div>
              ) : (
                <label className="flex h-12 cursor-pointer items-center justify-center gap-2 rounded-lg border border-dashed border-[var(--color-ink-300)] font-medium text-[var(--muted)]">
                  <Camera size={18} />
                  영수증 촬영 / 선택
                  <input
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={async (e) => {
                      const f = e.target.files?.[0];
                      e.target.value = "";
                      if (f) setReceipt(await shrinkImage(f));
                    }}
                  />
                </label>
              )}
            </Field>

            {error && (
              <p
                role="alert"
                className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
              >
                {error}
              </p>
            )}

            <Button
              type="submit"
              size="lg"
              full
              loading={saving}
              disabled={!ready}
            >
              기록하기
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}

/**
 * 저장 직후 화면.
 *
 * 그냥 닫아버리면 사장님은 방금 넣은 게 맞게 들어갔는지 알 수 없다.
 * 특히 "지난번보다 얼마 비싸게 샀는지"는 이 순간에 보여줘야 쓸모가 있다.
 */
function Saved({
  result,
  onAgain,
  onClose,
}: {
  result: Created;
  onAgain: () => void;
  onClose: () => void;
}) {
  const diff = result.previousUnitCostKrw
    ? result.unitCostKrw - result.previousUnitCostKrw
    : 0;

  return (
    <div className="grid gap-4">
      <dl className="grid gap-1 rounded-lg bg-[var(--surface-sunken)] p-4">
        <dt className="text-sm text-[var(--muted)]">개당 실단가</dt>
        <dd className="text-2xl font-bold tabular-nums">
          {krw(result.unitCostKrw)}
        </dd>
        <dd className="text-sm text-[var(--muted)] tabular-nums">
          합계 {krw(result.totalKrw)} · 현재 재고 {result.qtyOnHand}개
        </dd>
      </dl>

      {diff !== 0 && (
        <p
          className={[
            "rounded-lg p-3.5 text-sm",
            diff > 0
              ? "bg-[var(--tone-warn-bg)] text-[var(--tone-warn-fg)]"
              : "bg-[var(--tone-ok-bg)] text-[var(--tone-ok-fg)]",
          ].join(" ")}
        >
          지난번({krw(result.previousUnitCostKrw)})보다 개당{" "}
          <b>
            {krw(Math.abs(diff))} {diff > 0 ? "비싸게" : "싸게"}
          </b>{" "}
          샀습니다.
        </p>
      )}

      <div className="grid gap-2">
        <Button size="lg" full onClick={onAgain}>
          하나 더 기록
        </Button>
        <Button variant="ghost" full onClick={onClose}>
          재고 목록으로
        </Button>
      </div>
    </div>
  );
}

function digits(v: string): string {
  return v.replace(/[^0-9]/g, "");
}

function newKey(): string {
  return typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : String(Date.now());
}
