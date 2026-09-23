"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowsLeftRight,
  Info,
  Phone,
  Plus,
  Warning,
} from "@phosphor-icons/react";

import { ApiError, get, post } from "@/lib/api";
import { ago, krw, phone as fmtPhone } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ListingSheet } from "./listing-sheet";

type Listing = {
  id: string;
  storeName: string;
  name: string;
  kind: string;
  kindLabel: string;
  qty: number;
  unitPriceKrw: number;
  note: string;
  regionName: string;
  regionDetail: string;
  status: string;
  createdAt: string;
  mine: boolean;
  reportCount: number;
};

type Region = { code: string; name: string };

type Contact = {
  storeName: string;
  phone: string;
  bizNo: string;
  remainingToday: number;
};

const KIND_TABS = [
  { value: "", label: "전체" },
  { value: "sell", label: "판매" },
  { value: "swap", label: "교환" },
] as const;

/**
 * 장터.
 *
 * 우리는 거래 당사자가 아니다. 결제도 채팅도 없고, 사장님끼리 전화해서
 * 직거래한다. 화면 어디에도 "안전 거래" 같은 말을 쓰지 않는다 — 지킬 수
 * 없는 약속이고, 그 약속을 믿고 사고가 나면 그건 우리가 만든 사고다.
 */
export default function MarketPage() {
  const router = useRouter();

  const [listings, setListings] = useState<Listing[] | null>(null);
  const [regions, setRegions] = useState<Region[]>([]);
  const [region, setRegion] = useState("");
  const [kind, setKind] = useState("");
  const [mineOnly, setMineOnly] = useState(false);

  const [sheet, setSheet] = useState(false);
  const [error, setError] = useState("");
  // 번호는 눌렀을 때만 받아온다. 목록 응답에는 애초에 들어 있지 않다.
  const [contacts, setContacts] = useState<Record<string, Contact>>({});

  const load = useCallback(async () => {
    try {
      const path = mineOnly
        ? "/api/admin/market/mine"
        : `/api/admin/market?${new URLSearchParams({ region, kind })}`;
      const res = await get<{ listings: Listing[]; regions?: Region[] }>(path);
      setListings(res.listings ?? []);
      if (res.regions) setRegions(res.regions);
      setError("");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace("/admin/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "불러오지 못했습니다.");
    }
  }, [router, region, kind, mineOnly]);

  useEffect(() => {
    void load();
  }, [load]);

  async function reveal(id: string) {
    try {
      const c = await post<Contact>(`/api/admin/market/${id}/contact`);
      setContacts((prev) => ({ ...prev, [id]: c }));
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "연락처를 불러오지 못했습니다.",
      );
    }
  }

  async function act(id: string, what: "close" | "report") {
    const ok = window.confirm(
      what === "close"
        ? "이 글을 내릴까요? 다시 올리려면 새로 써야 합니다."
        : "이 글을 신고할까요? 여러 매장이 신고하면 자동으로 내려갑니다.",
    );
    if (!ok) return;
    try {
      await post(`/api/admin/market/${id}/${what}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "처리하지 못했습니다.");
    }
  }

  return (
    <main className="mx-auto max-w-3xl px-4 pb-20 pt-6">
      <header className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">장터</h1>
          <p className="mt-0.5 text-sm text-[var(--muted)]">
            안 나가는 재고를 다른 매장과 사고팔기
          </p>
        </div>
        <Button onClick={() => setSheet(true)}>
          <Plus size={16} weight="bold" /> 내놓기
        </Button>
      </header>

      {/* 필터. 지역이 먼저다 — 인형 상자는 택배로 보내면 배보다 배꼽이 크다. */}
      <div className="mb-4 grid gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={region}
            onChange={(e) => {
              setRegion(e.target.value);
              setMineOnly(false);
            }}
            aria-label="지역"
            disabled={mineOnly}
            className="h-10 rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3 text-sm font-medium text-[var(--fg)] disabled:opacity-50"
          >
            <option value="">전국</option>
            {regions.map((r) => (
              <option key={r.code} value={r.code}>
                {r.name}
              </option>
            ))}
          </select>

          {KIND_TABS.map((t) => (
            <button
              key={t.value}
              type="button"
              onClick={() => {
                setKind(t.value);
                setMineOnly(false);
              }}
              aria-pressed={!mineOnly && kind === t.value}
              className={[
                "h-10 rounded-full px-4 text-sm font-semibold transition-colors duration-150",
                !mineOnly && kind === t.value
                  ? "bg-accent-600 text-white"
                  : "bg-[var(--surface-sunken)] text-[var(--muted)]",
              ].join(" ")}
            >
              {t.label}
            </button>
          ))}

          <button
            type="button"
            onClick={() => setMineOnly((v) => !v)}
            aria-pressed={mineOnly}
            className={[
              "h-10 rounded-full px-4 text-sm font-semibold transition-colors duration-150",
              mineOnly
                ? "bg-accent-600 text-white"
                : "bg-[var(--surface-sunken)] text-[var(--muted)]",
            ].join(" ")}
          >
            내 글
          </button>
        </div>
      </div>

      {error && (
        <p
          role="alert"
          className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
        >
          {error}
        </p>
      )}

      {!listings ? (
        <ul className="grid gap-2">
          {[0, 1, 2].map((i) => (
            <li key={i} className="surface h-24 animate-pulse" />
          ))}
        </ul>
      ) : listings.length === 0 ? (
        <Empty mineOnly={mineOnly} onStart={() => setSheet(true)} />
      ) : (
        <ul className="grid gap-2">
          {listings.map((l) => (
            <ListingRow
              key={l.id}
              listing={l}
              contact={contacts[l.id]}
              onReveal={() => void reveal(l.id)}
              onClose={() => void act(l.id, "close")}
              onReport={() => void act(l.id, "report")}
            />
          ))}
        </ul>
      )}

      {/*
        지킬 수 있는 말만 쓴다. 우리는 돈에 개입하지 않고, 그래서
        보증할 수 있는 것도 없다. 그 사실을 화면 아래에 늘 둔다.
      */}
      <p className="mt-6 flex gap-2 rounded-lg bg-[var(--surface-sunken)] p-3.5 text-sm text-[var(--muted)]">
        <Info size={18} className="mt-0.5 shrink-0" />
        <span>
          claw-hub는 거래 당사자가 아닙니다. 직거래 시 실물과 수량을 직접
          확인하세요.
        </span>
      </p>

      {sheet && (
        <ListingSheet
          onClose={() => setSheet(false)}
          onPosted={() => {
            setSheet(false);
            setMineOnly(true);
            void load();
          }}
        />
      )}
    </main>
  );
}

function ListingRow({
  listing: l,
  contact,
  onReveal,
  onClose,
  onReport,
}: {
  listing: Listing;
  contact?: Contact;
  onReveal: () => void;
  onClose: () => void;
  onReport: () => void;
}) {
  const closed = l.status !== "open";

  return (
    <li className={`surface p-3.5 ${closed ? "opacity-60" : ""}`}>
      <div className="mb-1.5 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="mb-1 flex flex-wrap items-center gap-1.5">
            <Badge tone={l.kind === "swap" ? "accent" : "neutral"}>
              {l.kind === "swap" && <ArrowsLeftRight size={12} weight="bold" />}
              {l.kindLabel}
            </Badge>
            {l.mine && <Badge tone="ok">내 글</Badge>}
            {closed && (
              <Badge tone="warn">
                {l.status === "removed" ? "신고로 내려감" : "내림"}
              </Badge>
            )}
          </div>
          <p className="truncate font-semibold">{l.name}</p>
          <p className="mt-0.5 text-sm text-[var(--muted)] tabular-nums">
            {l.qty}개 · {l.regionName} {l.regionDetail} · {ago(l.createdAt)}
          </p>
        </div>
        {l.unitPriceKrw > 0 && (
          <span className="shrink-0 text-base font-bold tabular-nums">
            {krw(l.unitPriceKrw)}
          </span>
        )}
      </div>

      {l.note && <p className="mb-2 whitespace-pre-wrap text-sm">{l.note}</p>}

      {l.mine ? (
        <div className="flex items-center gap-2">
          {!closed && (
            <Button variant="secondary" onClick={onClose}>
              내리기
            </Button>
          )}
          {l.reportCount > 0 && (
            <span className="text-sm text-[var(--tone-warn-fg)]">
              신고 {l.reportCount}건
            </span>
          )}
        </div>
      ) : (
        <div className="grid gap-2">
          {contact ? (
            <div className="grid gap-1.5 rounded-lg bg-[var(--surface-sunken)] p-3">
              <a
                href={`tel:${contact.phone}`}
                className="flex h-12 items-center justify-center gap-2 rounded-lg bg-accent-600 font-semibold text-white"
              >
                <Phone size={18} weight="fill" />
                {fmtPhone(contact.phone)}
              </a>
              <p className="text-xs text-[var(--muted)]">
                {contact.storeName} · 사업자 {contact.bizNo} · 오늘{" "}
                {contact.remainingToday}번 더 볼 수 있습니다
              </p>
            </div>
          ) : (
            <div>
              <Button variant="secondary" onClick={onReveal} disabled={closed}>
                <Phone size={16} weight="regular" /> 연락처 보기
              </Button>
            </div>
          )}

          {/*
            신고는 연락처를 본 뒤에도 남아 있어야 한다. 오히려 그때가
            신고할 일이 생기는 순간이다 — 전화해보니 말이 다르거나
            아예 안 받는 경우.
          */}
          <button
            type="button"
            onClick={onReport}
            className="flex h-9 w-fit items-center gap-1 text-sm text-[var(--muted)] hover:text-[var(--tone-stop-fg)]"
          >
            <Warning size={14} weight="regular" /> 신고
          </button>
        </div>
      )}
    </li>
  );
}

function Empty({
  mineOnly,
  onStart,
}: {
  mineOnly: boolean;
  onStart: () => void;
}) {
  if (mineOnly) {
    return (
      <div className="surface grid justify-items-center gap-4 px-4 py-10 text-center">
        <div className="grid gap-1">
          <p className="font-semibold">아직 내놓은 재고가 없습니다</p>
          <p className="text-sm text-[var(--muted)]">
            안 나가는 인형을 올려두면 다른 매장 사장님이 보고 연락합니다.
          </p>
        </div>
        <Button size="lg" onClick={onStart}>
          첫 글 올리기
        </Button>
      </div>
    );
  }
  return (
    <div className="surface grid justify-items-center gap-4 px-4 py-10 text-center">
      <div className="grid gap-1">
        <p className="font-semibold">이 조건에 올라온 글이 없습니다</p>
        <p className="text-sm text-[var(--muted)]">
          지역을 &lsquo;전국&rsquo;으로 넓혀보시거나, 먼저 내놓아보세요.
        </p>
      </div>
      <Button size="lg" onClick={onStart}>
        내 재고 내놓기
      </Button>
    </div>
  );
}
