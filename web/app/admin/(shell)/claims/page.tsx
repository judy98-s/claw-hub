"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  CreditCard,
  Image as ImageIcon,
  ShieldWarning,
} from "@phosphor-icons/react";

import { ApiError, get } from "@/lib/api";
import { ago, krw } from "@/lib/format";
import { Badge, toneForStatus } from "@/components/ui/badge";
import type { RiskReason } from "@/components/risk-badge";

type ClaimSummary = {
  id: string;
  machineLabel: string;
  issueLabel: string;
  paymentMethod: string;
  amountKrw: number;
  status: string;
  statusLabel: string;
  riskReasons: RiskReason[];
  phoneMasked: string;
  photoCount: number;
  createdAt: string;
};

/**
 * 탭은 "내가 지금 뭘 해야 하나"로 나눈다.
 *
 * pending 과 needs_review 를 한 탭에 묶은 이유: 사장님 입장에서는 둘 다
 * "내가 봐야 할 것"이다. 나눠두면 두 군데를 확인해야 하고, 한쪽을 잊는다.
 * 고액·리스크 건이라는 구분은 없애지 않고 배지로 남겨서, 같은 목록 안에서
 * 눈에 띄게 했다.
 */
const TABS = [
  { key: "needs_review,pending", label: "처리 대기" },
  { key: "on_hold", label: "보류" },
  { key: "approved", label: "송금 대기" },
  { key: "paid,rejected", label: "완료" },
] as const;

const POLL_MS = 30_000;

/**
 * 홈에서 `?status=approved` 같은 링크로 들어온다. 그 상태를 담고 있는 탭을
 * 열어준다 — 안 그러면 "송금 대기 3건"을 누르고 처리 대기 목록이 뜬다.
 */
function tabForStatus(status: string | null): string {
  if (!status) return TABS[0].key;
  const hit = TABS.find((t) => t.key.split(",").includes(status));
  return hit?.key ?? TABS[0].key;
}

export default function InboxPage() {
  return (
    <Suspense fallback={<main className="mx-auto max-w-lg px-4 pt-6" />}>
      <Inbox />
    </Suspense>
  );
}

function Inbox() {
  const router = useRouter();
  const params = useSearchParams();
  const machineId = params.get("machineId") ?? "";
  const [tab, setTab] = useState<string>(() =>
    tabForStatus(params.get("status")),
  );
  const [claims, setClaims] = useState<ClaimSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(
    async (status: string) => {
      try {
        const q = new URLSearchParams({ status });
        if (machineId) q.set("machineId", machineId);
        const res = await get<{ claims: ClaimSummary[] }>(
          `/api/admin/claims?${q}`,
        );
        // 확인이 필요한 건을 위로 올린다. 같은 목록에 섞어두면 소액
        // 대기 건 사이에 고액 건이 묻힌다.
        const rank = (c: ClaimSummary) =>
          c.status === "needs_review" || c.riskReasons.length > 0 ? 0 : 1;
        setClaims([...(res.claims ?? [])].sort((a, b) => rank(a) - rank(b)));
        setError("");
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          router.replace("/admin/login");
          return;
        }
        setError(
          err instanceof ApiError ? err.message : "목록을 불러오지 못했습니다.",
        );
      } finally {
        setLoading(false);
      }
    },
    [router, machineId],
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

      {/* 기계로 좁혀 들어오면 그 사실이 보여야 한다. 안 보이면 "왜 3건밖에
          없지" 하고 목록을 의심하게 된다. */}
      {machineId && (
        <div className="mb-4 flex items-center justify-between gap-2 rounded-lg bg-[var(--surface-sunken)] px-3.5 py-2.5 text-sm">
          <span>기계 하나로 좁혀 보는 중입니다.</span>
          <Link href="/admin/claims" className="font-semibold text-accent-600">
            전체 보기
          </Link>
        </div>
      )}

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
        <p
          role="alert"
          className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
        >
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
              <Link
                href={`/admin/claims/${c.id}`}
                className="surface block p-3.5"
              >
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
                  {/* 카드 건은 송금이 아니라 단말기 취소다. 목록에서 갈린다. */}
                  {c.paymentMethod === "card" && (
                    <Badge
                      tone="accent"
                      icon={<CreditCard size={12} weight="regular" />}
                    >
                      카드
                    </Badge>
                  )}
                  {c.riskReasons.length > 0 && (
                    <Badge
                      tone="warn"
                      icon={<ShieldWarning size={12} weight="fill" />}
                    >
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
