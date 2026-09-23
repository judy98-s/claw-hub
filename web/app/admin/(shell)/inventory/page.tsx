"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Plus } from "@phosphor-icons/react";

import { ApiError, get, post } from "@/lib/api";
import { krw, monthLabel, shortDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { PurchaseSheet } from "./purchase-sheet";
import { ListingSheet } from "../market/listing-sheet";

type Item = {
  nameKey: string;
  name: string;
  qtyOnHand: number;
  qtyBought: number;
  avgUnitCostKrw: number;
  valueKrw: number;
  lastVendor: string;
  lastUnitCostKrw: number;
  lastPurchasedAt: string;
  purchaseCount: number;
};

type Inventory = {
  summary: {
    spentKrw: number;
    itemCount: number;
    qtyOnHand: number;
    valueKrw: number;
  };
  items: Item[];
  spentFrom: string;
};

/**
 * 재고 장부.
 *
 * 이 화면은 접수함과 성격이 반대다. 접수함은 길에서 한 손으로 30초 안에
 * 닫는 화면이고, 여기는 앉아서 들여다보는 화면이다. 그래서 카드가 아니라
 * 표다 — 도매를 하는 사람은 엑셀을 본다. 한 화면에 12줄이 들어가는 밀도가
 * 카드 세 개가 여백을 두고 떠 있는 것보다 낫다.
 */
export default function InventoryPage() {
  const router = useRouter();
  const [data, setData] = useState<Inventory | null>(null);
  const [error, setError] = useState("");
  const [sheet, setSheet] = useState(false);
  const [counting, setCounting] = useState<Item | null>(null);
  // 재고에서 바로 내놓기. 품명·보유 수량·원가가 이미 있으니 한 번 탭이다.
  const [listing, setListing] = useState<Item | null>(null);

  const load = useCallback(async () => {
    try {
      setData(await get<Inventory>("/api/admin/inventory"));
      setError("");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace("/admin/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "불러오지 못했습니다.");
    }
  }, [router]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <main className="mx-auto max-w-3xl px-4 pt-6">
      <header className="mb-5 flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">재고</h1>
          <p className="mt-0.5 text-sm text-[var(--muted)]">
            얼마에 사서 몇 개 남았는지
          </p>
        </div>
        <Button onClick={() => setSheet(true)}>
          <Plus size={16} weight="bold" /> 사입 기록
        </Button>
      </header>

      {error && (
        <p
          role="alert"
          className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
        >
          {error}
        </p>
      )}

      {!data ? (
        <div className="h-64 animate-pulse rounded-lg bg-[var(--surface-sunken)]" />
      ) : data.items.length === 0 ? (
        <Empty onStart={() => setSheet(true)} />
      ) : (
        <>
          <dl className="surface mb-4 grid grid-cols-2 divide-x divide-[var(--line)]">
            <Stat
              label={`${monthLabel(data.spentFrom)} 인형값`}
              value={krw(data.summary.spentKrw)}
            />
            <Stat
              label="재고 자산"
              value={krw(data.summary.valueKrw)}
              sub={`${data.summary.itemCount}종 · ${data.summary.qtyOnHand}개`}
            />
          </dl>

          <Table items={data.items} onCount={setCounting} />
        </>
      )}

      {sheet && (
        <PurchaseSheet
          onClose={() => setSheet(false)}
          onSaved={() => void load()}
        />
      )}
      {counting && (
        <CountSheet
          item={counting}
          onClose={() => setCounting(null)}
          onSell={() => {
            setListing(counting);
            setCounting(null);
          }}
          onSaved={() => {
            setCounting(null);
            void load();
          }}
        />
      )}
      {listing && (
        <ListingSheet
          from={{
            name: listing.name,
            qtyOnHand: listing.qtyOnHand,
            avgUnitCostKrw: listing.avgUnitCostKrw,
          }}
          onClose={() => setListing(null)}
          onPosted={() => {
            setListing(null);
            router.push("/admin/market");
          }}
        />
      )}
    </main>
  );
}

function Stat({
  label,
  value,
  sub,
}: {
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <div className="grid gap-0.5 px-4 py-3.5">
      <dt className="text-xs text-[var(--muted)]">{label}</dt>
      <dd className="text-xl font-bold tabular-nums">{value}</dd>
      {sub && (
        <dd className="text-xs text-[var(--muted)] tabular-nums">{sub}</dd>
      )}
    </div>
  );
}

/**
 * 재고 표.
 *
 * 넓은 화면에서는 진짜 표로, 폰에서는 줄당 두 행으로 접는다. 표를 가로로
 * 스크롤시키면 사장님은 오른쪽 끝에 무슨 숫자가 있는지 영영 모른다.
 */
function Table({
  items,
  onCount,
}: {
  items: Item[];
  onCount: (i: Item) => void;
}) {
  return (
    <div className="surface overflow-hidden">
      {/* 넓은 화면 전용 머리글. 폰에서는 각 줄이 스스로를 설명한다. */}
      <div className="hidden border-b border-[var(--line)] px-4 py-2.5 text-xs font-semibold text-[var(--muted)] sm:grid sm:grid-cols-[1fr_5rem_6rem_7rem]">
        <span>품목</span>
        <span className="text-right">보유</span>
        <span className="text-right">평균 원가</span>
        <span className="text-right">마지막 사입</span>
      </div>

      <ul className="divide-y divide-[var(--line)]">
        {items.map((it) => {
          const empty = it.qtyOnHand <= 0;
          return (
            <li key={it.nameKey}>
              <button
                type="button"
                onClick={() => onCount(it)}
                className="grid w-full gap-1 px-4 py-3 text-left transition-colors duration-150 hover:bg-[var(--surface-sunken)] sm:grid-cols-[1fr_5rem_6rem_7rem] sm:items-baseline sm:gap-0"
              >
                <span
                  className={`truncate font-semibold ${empty ? "text-[var(--muted)]" : ""}`}
                >
                  {it.name}
                </span>

                {/* 폰: 한 줄에 몰아서. 넓은 화면: 열마다 따로. */}
                <span className="flex gap-3 text-sm text-[var(--muted)] tabular-nums sm:hidden">
                  <span
                    className={empty ? "" : "font-semibold text-[var(--fg)]"}
                  >
                    {it.qtyOnHand}개
                  </span>
                  <span>평균 {krw(it.avgUnitCostKrw)}</span>
                  <span>
                    {shortDate(it.lastPurchasedAt)}
                    {it.lastVendor && ` · ${it.lastVendor}`}
                  </span>
                </span>

                <span
                  className={`hidden text-right tabular-nums sm:block ${empty ? "text-[var(--muted)]" : "font-semibold"}`}
                >
                  {it.qtyOnHand}개
                </span>
                <span className="hidden text-right tabular-nums sm:block">
                  {krw(it.avgUnitCostKrw)}
                </span>
                <span className="hidden truncate text-right text-sm text-[var(--muted)] tabular-nums sm:block">
                  {shortDate(it.lastPurchasedAt)}
                  {it.lastVendor && ` · ${it.lastVendor}`}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * 빈 상태.
 *
 * "아직 기록이 없습니다" 한 줄로 끝내면 사장님은 여기서 뭘 해야 하는지
 * 모른 채 나간다. 할 행동 하나와, 무엇이 생길지 보여주는 샘플 한 줄을 둔다.
 */
function Empty({ onStart }: { onStart: () => void }) {
  return (
    <div className="surface grid justify-items-center gap-4 px-4 py-10 text-center">
      <div className="grid gap-1">
        <p className="font-semibold">인형을 들이면 여기에 쌓입니다</p>
        <p className="text-sm text-[var(--muted)]">
          얼마에 샀는지 적어두면 다음에 살 때 비싼지 싼지 바로 압니다.
        </p>
      </div>

      {/* 무엇이 생길지 보여주는 샘플. 흐리게 깔아서 진짜 데이터와 구분한다. */}
      <div
        aria-hidden
        className="w-full max-w-sm rounded-lg border border-[var(--line)] px-4 py-3 text-left opacity-40"
      >
        <p className="font-semibold">쿠로미 중형 30cm</p>
        <p className="text-sm text-[var(--muted)] tabular-nums">
          42개 · 평균 2,350원 · 9/14 · 캐치돌
        </p>
      </div>

      <Button size="lg" onClick={onStart}>
        첫 사입 기록하기
      </Button>
    </div>
  );
}

/**
 * 실사 보정.
 *
 * 재고를 매일 차감하게 만드는 설계는 하지 않았다 — 아무도 안 한다.
 * 대신 가끔 세어보고 맞추는 이 화면 하나를 둔다. 실제 매장이 재고를
 * 관리하는 방식이 그거다.
 */
function CountSheet({
  item,
  onClose,
  onSaved,
  onSell,
}: {
  item: Item;
  onClose: () => void;
  onSaved: () => void;
  onSell: () => void;
}) {
  const [counted, setCounted] = useState(String(item.qtyOnHand));
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const n = Number(counted || -1);
  const diff = n >= 0 ? n - item.qtyOnHand : 0;

  async function save() {
    setSaving(true);
    setError("");
    try {
      await post(
        `/api/admin/inventory/${encodeURIComponent(item.nameKey)}/count`,
        {
          countedQty: n,
          note,
        },
      );
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "저장하지 못했습니다.");
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50 sm:items-center">
      <div className="w-full max-w-lg rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] sm:rounded-2xl">
        <h2 className="text-lg font-bold">{item.name}</h2>
        <p className="mt-0.5 text-sm text-[var(--muted)] tabular-nums">
          장부상 {item.qtyOnHand}개 · 평균 원가 {krw(item.avgUnitCostKrw)} · 총{" "}
          {item.purchaseCount}번 사입
        </p>

        <div className="mt-4 grid gap-3">
          <label htmlFor="counted" className="text-sm font-semibold">
            세어보니 몇 개인가요?
          </label>
          <input
            id="counted"
            type="text"
            inputMode="numeric"
            value={counted}
            onChange={(e) => setCounted(e.target.value.replace(/[^0-9]/g, ""))}
            className="h-12 w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3.5 text-base tabular-nums text-[var(--fg)] focus:border-accent-600 focus:outline-none"
          />

          {diff !== 0 && n >= 0 && (
            <p className="text-sm text-[var(--muted)] tabular-nums">
              장부보다 {Math.abs(diff)}개 {diff < 0 ? "적습니다" : "많습니다"}.
              기록해도 지난 사입 내역은 그대로 남습니다.
            </p>
          )}

          <input
            type="text"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="메모 (선택)"
            maxLength={200}
            aria-label="실사 메모"
            className="h-12 w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3.5 text-base text-[var(--fg)] focus:border-accent-600 focus:outline-none"
          />

          {error && (
            <p
              role="alert"
              className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
            >
              {error}
            </p>
          )}

          <Button
            size="lg"
            full
            loading={saving}
            disabled={n < 0}
            onClick={() => void save()}
          >
            기록하기
          </Button>
          <Button variant="ghost" full onClick={onClose}>
            닫기
          </Button>
        </div>
      </div>
    </div>
  );
}
