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
import { Mascot } from "@/components/mascot";
import { STORES, type StoreType } from "@/lib/stores";
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
  // 매장 캐릭터와 기계 이상 말풍선. 홈이 이미 두 값을 다 계산해 준다 —
  // 같은 판단을 화면마다 다시 하면 홈과 목록이 서로 다른 말을 하게 된다.
  const [storeType, setStoreType] = useState<StoreType>("claw");
  const [alert, setAlert] = useState("");
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

  /*
    매장 업종과 기계 이상 문구를 홈에서 가져온다.

    같은 판단을 여기서 다시 하지 않는 이유: 몇 건부터 "이상" 인지는 도메인
    지식이고 서버가 이미 정해 놓았다. 화면마다 따로 세면 홈은 알림을 띄우는데
    목록은 조용한 상태가 생긴다. 실패는 무시한다 — 말풍선이 없다고 목록을
    못 보면 안 된다.
  */
  useEffect(() => {
    void get<{ storeType: string; actions: { kind: string; title: string; detail: string }[] }>(
      "/api/admin/home",
    )
      .then((h) => {
        if (h.storeType && h.storeType in STORES) setStoreType(h.storeType as StoreType);
        const a = (h.actions ?? []).find((x) => x.kind === "machine_alert");
        setAlert(a ? `${a.title}. ${a.detail}` : "");
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    void load(tab);

    // 폴링. 사장님이 화면을 켜둔 채 매장을 보고 있을 때 새 접수가 뜬다.
    const timer = setInterval(() => void load(tab), POLL_MS);
    return () => clearInterval(timer);
  }, [tab, load]);

  return (
    <main className="mx-auto max-w-lg px-3.5 pt-5">
      <header className="mb-3 flex items-baseline justify-between gap-2">
        <h1 className="text-xl font-extrabold">민원</h1>
        <span className="text-[13px] text-ink-soft">30초마다 새로고침</span>
      </header>

      {/*
        기계 이상 말풍선. 서버가 문장을 만들어 준다 — 몇 건부터 이상인지는
        도메인 지식이고, 화면마다 다시 판단하면 홈과 다른 말을 하게 된다.
      */}
      {alert && (
        <div className="mb-3 flex items-center gap-2.5 rounded-md border-2 border-ink bg-pink-soft px-3 py-2">
          <Mascot store={storeType} mood="worried" size={34} className="shrink-0" />
          <p className="text-[14px] font-semibold leading-snug">{alert}</p>
        </div>
      )}

      {/* 기계로 좁혀 들어오면 그 사실이 보여야 한다. 안 보이면 "왜 3건밖에
          없지" 하고 목록을 의심하게 된다. */}
      {machineId && (
        <div className="mb-3 flex items-center justify-between gap-2 rounded-md border-2 border-ink bg-surface px-3 py-2 text-sm">
          <span className="font-semibold">기계 하나로 좁혀 보는 중입니다.</span>
          <Link href="/admin/claims" className="font-bold underline underline-offset-2">
            전체 보기
          </Link>
        </div>
      )}

      <div className="-mx-3.5 mb-3 overflow-x-auto px-3.5">
        <div className="flex gap-1.5">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              aria-pressed={tab === t.key}
              className={[
                "h-[34px] shrink-0 rounded-md border-2 border-ink px-3 text-[14px] font-bold",
                tab === t.key ? "bg-action text-ink" : "bg-surface text-ink-soft",
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
          className="rounded-md border-2 border-stop-600 bg-stop-50 p-3 text-sm font-semibold text-stop-700"
        >
          {error}
        </p>
      )}

      {loading ? (
        <div className="h-40 animate-pulse rounded-md border-2 border-ink/15 bg-surface" />
      ) : claims.length === 0 ? (
        <div className="grid justify-items-center gap-2 rounded-md border-2 border-ink bg-surface px-4 py-10 text-center">
          <Mascot store={storeType} mood="sleep" size={72} />
          <p className="font-bold">여기에 표시할 민원이 없습니다.</p>
        </div>
      ) : (
        /*
          카드 목록이 아니라 한 카드 안의 줄 목록이다 (§6.2 ①).
          하루 수십 번 훑는 화면이라 밀도가 먼저다 — 카드마다 그림자와
          여백을 주면 한 화면에 세 건밖에 안 들어온다.
        */
        <ul className="divide-y divide-[#F2F0EC] overflow-hidden rounded-md border-2 border-ink bg-surface">
          {claims.map((c) => (
            <li key={c.id}>
              <Link
                href={`/admin/claims/${c.id}`}
                // 확인이 필요한 건은 바탕으로 구분한다. 배지만으로는
                // 빠르게 훑을 때 눈에 안 들어온다.
                className={`block px-3 py-2.5 ${
                  c.status === "needs_review" ? "bg-[#FFFDF5]" : ""
                }`}
              >
                <div className="flex items-baseline justify-between gap-3">
                  <p className="truncate text-[15px] font-bold">
                    {c.machineLabel} · {c.issueLabel}
                  </p>
                  <span className="shrink-0 text-[15px] font-bold tabular-nums">
                    {krw(c.amountKrw)}
                  </span>
                </div>

                <div className="mt-1 flex flex-wrap items-center gap-1">
                  <Badge tone={toneForStatus(c.status)}>{c.statusLabel}</Badge>
                  {/* 카드 건은 송금이 아니라 단말기 취소다. 목록에서 갈린다. */}
                  {c.paymentMethod === "card" && (
                    <Badge tone="accent" icon={<CreditCard size={12} weight="regular" />}>
                      카드
                    </Badge>
                  )}
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
                  <span className="ml-auto shrink-0 text-[13px] text-ink-soft">
                    {ago(c.createdAt)}
                  </span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
