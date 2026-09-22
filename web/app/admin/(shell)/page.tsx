"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  CheckCircle,
  Clock,
  GameController,
  GearSix,
  PaperPlaneTilt,
  Warning,
} from "@phosphor-icons/react";

import { ApiError, get } from "@/lib/api";
import { krw } from "@/lib/format";

type HomeAction = {
  kind: string;
  severity: "stop" | "warn" | "neutral";
  title: string;
  detail: string;
  count: number;
  href: string;
};

type Home = {
  storeName: string;
  actions: HomeAction[];
  todayClaims: number;
  todayPaid: number;
  todayPaidKrw: number;
};

/**
 * 종류마다 아이콘이 다르다. 전부 같은 동그라미를 쓰면 목록을 눈으로
 * 훑을 수 없고, 그러면 순서를 아무리 잘 잡아도 소용이 없다.
 */
const ICONS: Record<string, typeof Warning> = {
  payout_pending: PaperPlaneTilt,
  needs_review: Warning,
  pending: Clock,
  stale_on_hold: Clock,
  machine_alert: GameController,
  setup_payout: GearSix,
};

const SEVERITY: Record<HomeAction["severity"], string> = {
  stop: "bg-[var(--tone-stop-bg)] text-[var(--tone-stop-fg)]",
  warn: "bg-[var(--tone-warn-bg)] text-[var(--tone-warn-fg)]",
  neutral: "bg-[var(--surface-sunken)] text-[var(--muted)]",
};

const POLL_MS = 30_000;

/**
 * 홈.
 *
 * 이 화면의 질문은 하나다 — "지금 내가 뭘 해야 하나". 그래서 통계가 아니라
 * 할 일이 먼저 온다. 순서는 서버가 정한다: 승인해놓고 안 보낸 환불이 가장
 * 위다. 손님에게 돈을 약속하고 주지 않은 상태이고, 새 신고보다 급하다.
 *
 * 할 일이 하나도 없으면 그 사실 자체가 화면이다. 빈 목록에 "표시할 항목이
 * 없습니다"를 띄우면 사장님은 뭔가 고장났나 생각한다.
 */
export default function HomePage() {
  const router = useRouter();
  const [home, setHome] = useState<Home | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setHome(await get<Home>("/api/admin/home"));
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
    const timer = setInterval(() => void load(), POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  return (
    <main className="mx-auto max-w-2xl px-4 pt-6">
      <header className="mb-5">
        <p className="text-sm text-[var(--muted)]">{home?.storeName ?? " "}</p>
        <h1 className="text-xl font-bold">확인이 필요한 것</h1>
      </header>

      {error && (
        <p
          role="alert"
          className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
        >
          {error}
        </p>
      )}

      {!home ? (
        <ul className="grid gap-2">
          {[0, 1].map((i) => (
            <li key={i} className="surface h-20 animate-pulse" />
          ))}
        </ul>
      ) : home.actions.length === 0 ? (
        <div className="surface grid justify-items-center gap-2 px-4 py-12 text-center">
          <CheckCircle size={40} weight="fill" className="text-ok-600" />
          <p className="font-semibold">지금 처리할 게 없습니다</p>
          <p className="text-sm text-[var(--muted)]">
            새 신고가 들어오면 Slack 알림이 가고 여기에도 올라옵니다.
          </p>
        </div>
      ) : (
        <ul className="grid gap-2">
          {home.actions.map((a) => {
            const Icon = ICONS[a.kind] ?? Warning;
            return (
              <li key={a.kind + a.href}>
                <Link
                  href={a.href}
                  className="surface flex items-center gap-3.5 p-3.5 transition-colors duration-150 hover:bg-[var(--surface-sunken)]"
                >
                  <span
                    className={`grid size-10 shrink-0 place-items-center rounded-lg ${SEVERITY[a.severity]}`}
                  >
                    <Icon size={20} weight="fill" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block font-semibold">{a.title}</span>
                    <span className="mt-0.5 block text-sm text-[var(--muted)]">
                      {a.detail}
                    </span>
                  </span>
                  <ArrowRight
                    size={18}
                    weight="bold"
                    className="shrink-0 text-[var(--muted)]"
                  />
                </Link>
              </li>
            );
          })}
        </ul>
      )}

      {/* 오늘 숫자는 할 일 아래다. 읽으면 좋지만 누구도 이걸 보러 오지 않는다. */}
      <section className="mt-6">
        <h2 className="mb-2 text-sm font-semibold text-[var(--muted)]">오늘</h2>
        <dl className="surface grid grid-cols-3 divide-x divide-[var(--line)]">
          <Stat label="접수" value={`${home?.todayClaims ?? 0}건`} />
          <Stat label="환불" value={`${home?.todayPaid ?? 0}건`} />
          <Stat label="나간 금액" value={krw(home?.todayPaidKrw ?? 0)} />
        </dl>
      </section>
    </main>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-0.5 px-3 py-3.5">
      <dt className="text-xs text-[var(--muted)]">{label}</dt>
      <dd className="text-lg font-bold tabular-nums">{value}</dd>
    </div>
  );
}
