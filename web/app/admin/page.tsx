"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Image as ImageIcon, ShieldWarning } from "@phosphor-icons/react";

import { ApiError, get } from "@/lib/api";
import { ago, krw } from "@/lib/format";
import { Badge, toneForStatus } from "@/components/ui/badge";
import type { RiskReason } from "@/components/risk-badge";

type ClaimSummary = {
  id: string;
  machineLabel: string;
  issueLabel: string;
  amountKrw: number;
  status: string;
  statusLabel: string;
  riskReasons: RiskReason[];
  phoneMasked: string;
  photoCount: number;
  createdAt: string;
};

/**
 * 탭 순서는 처리 순서다.
 *
 * 기본 진입이 "확인 필요"인 이유: 사장님이 앱을 열었을 때 가장 먼저 봐야
 * 할 것은 판단이 필요한 건이다. 소액 대기 건은 한 번에 훑어도 된다.
 */
const TABS = [
  { key: "needs_review", label: "확인 필요" },
  { key: "pending", label: "대기" },
  { key: "on_hold", label: "보류" },
  { key: "approved", label: "송금 대기" },
  { key: "paid,rejected", label: "완료" },
] as const;

const POLL_MS = 30_000;

export default function InboxPage() {
  const router = useRouter();
  const [tab, setTab] = useState<string>(TABS[0].key);
  const [claims, setClaims] = useState<ClaimSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(
    async (status: string) => {
      try {
        const res = await get<{ claims: ClaimSummary[] }>(
          `/api/admin/claims?status=${encodeURIComponent(status)}`,
        );
        setClaims(res.claims ?? []);
        setError("");
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          router.replace("/admin/login");
          return;
        }
        setError(err instanceof ApiError ? err.message : "목록을 불러오지 못했습니다.");
      } finally {
        setLoading(false);
      }
    },
    [router],
  );

  useEffect(() => {
    setLoading(true);
    void load(tab);

    // 폴링. 사장님이 화면을 켜둔 채 매장을 보고 있을 때 새 접수가 뜬다.
    const timer = setInterval(() => void load(tab), POLL_MS);
    return () => clearInterval(timer);
  }, [tab, load]);

  return (
    <main className="mx-auto max-w-lg px-4 pt-6">
      <h1 className="mb-4 text-xl font-bold">접수함</h1>

      <div className="-mx-4 mb-4 overflow-x-auto px-4">
        <div className="flex gap-2">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              aria-pressed={tab === t.key}
              className={[
                "h-9 shrink-0 rounded-full px-4 text-sm font-semibold transition-colors duration-150",
                tab === t.key
                  ? "bg-accent-600 text-white"
                  : "bg-[var(--surface-sunken)] text-[var(--muted)]",
              ].join(" ")}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <p role="alert" className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
          {error}
        </p>
      )}

      {loading ? (
        <ul className="grid gap-2">
          {[0, 1, 2].map((i) => (
            <li key={i} className="surface h-24 animate-pulse" />
          ))}
        </ul>
      ) : claims.length === 0 ? (
        <p className="py-16 text-center text-[var(--muted)]">
          여기에 표시할 접수가 없습니다.
        </p>
      ) : (
        <ul className="grid gap-2">
          {claims.map((c) => (
            <li key={c.id}>
              <Link href={`/admin/claims/${c.id}`} className="surface block p-3.5">
                <div className="mb-1.5 flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate font-semibold">
                      {c.machineLabel} · {c.issueLabel}
                    </p>
                    <p className="mt-0.5 text-sm text-[var(--muted)]">
                      {c.phoneMasked} · {ago(c.createdAt)}
                    </p>
                  </div>
                  <span className="shrink-0 text-base font-bold tabular-nums">
                    {krw(c.amountKrw)}
                  </span>
                </div>

                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge tone={toneForStatus(c.status)}>{c.statusLabel}</Badge>
                  {c.riskReasons.length > 0 && (
                    <Badge tone="warn" icon={<ShieldWarning size={12} weight="fill" />}>
                      확인 {c.riskReasons.length}건
                    </Badge>
                  )}
                  {c.photoCount > 0 && (
                    <Badge icon={<ImageIcon size={12} weight="regular" />}>
                      사진 {c.photoCount}
                    </Badge>
                  )}
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
