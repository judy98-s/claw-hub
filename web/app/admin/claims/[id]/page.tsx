"use client";

import { Suspense, useCallback, useEffect, useState, use } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowLeft, PhoneCall, X } from "@phosphor-icons/react";

import { ApiError, getWith, postWith } from "@/lib/api";
import { dateTime, krw, phone as fmtPhone } from "@/lib/format";
import { Badge, toneForStatus } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { RiskReasons, type RiskReason } from "@/components/risk-badge";
import { PayoutSheet } from "@/components/payout-sheet";

type ClaimDetail = {
  id: string;
  receiptCode: string;
  machineLabel: string;
  machineCode: string;
  issueLabel: string;
  amountKrw: number;
  description: string;
  status: string;
  statusLabel: string;
  riskReasons: RiskReason[];
  phone: string;
  bankName: string;
  accountNo: string;
  holder: string;
  callRecommended: boolean;
  photoIds: string[];
  events: { actorKind: string; action: string; from: string; to: string; note: string; at: string }[];
  phoneClaims30d: number;
  phonePaidTotal30d: number;
  createdAt: string;
  allowedTransitions: string[];
};

export default function ClaimDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<DetailSkeleton />}>
      <ClaimDetail params={params} />
    </Suspense>
  );
}

function DetailSkeleton() {
  return (
    <main className="mx-auto max-w-lg px-4 pt-6">
      <div className="h-64 animate-pulse rounded-lg bg-[var(--surface-sunken)]" />
    </main>
  );
}

function ClaimDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const search = useSearchParams();

  /*
    Slack 링크의 접근 토큰. 첫 로드에서 한 번 읽고 주소창에서 지운다.

    남겨두면 새로고침·스크린샷·어깨너머로 계속 노출되고, 사장님이 주소를
    복사해 누군가에게 보내는 순간 그 사람도 손님 계좌를 보게 된다.
    이후 API 호출은 헤더로 보낸다.
  */
  const [token] = useState(() => search.get("t") ?? undefined);

  useEffect(() => {
    if (search.get("t")) {
      window.history.replaceState(null, "", window.location.pathname);
    }
  }, [search]);

  const [claim, setClaim] = useState<ClaimDetail | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [zoom, setZoom] = useState<string | null>(null);
  const [payoutOpen, setPayoutOpen] = useState(false);
  const [confirmReject, setConfirmReject] = useState(false);

  const load = useCallback(async () => {
    try {
      setClaim(await getWith<ClaimDetail>(`/api/admin/claims/${id}`, token));
      setError("");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        // 링크로 들어왔는데 토큰이 만료·위조된 경우는 로그인해도 같은
        // 건으로 못 가므로, 서버가 준 이유를 그대로 보여준다.
        if (token) {
          setError(err.message);
          return;
        }
        router.replace("/admin/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "불러오지 못했습니다.");
    }
  }, [id, router, token]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(action: "approve" | "reject" | "hold", note = "") {
    setBusy(true);
    try {
      await postWith(`/api/admin/claims/${id}/${action}`, { note }, token);
      setConfirmReject(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "처리에 실패했습니다.");
    } finally {
      setBusy(false);
    }
  }

  if (!claim) {
    return (
      <main className="mx-auto max-w-lg px-4 pt-6">
        {error ? (
          <p role="alert" className="rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
            {error}
          </p>
        ) : (
          <div className="h-64 animate-pulse rounded-lg bg-[var(--surface-sunken)]" />
        )}
      </main>
    );
  }

  const can = (s: string) => claim.allowedTransitions.includes(s);

  return (
    <main className="mx-auto max-w-lg px-4 pb-8 pt-4">
      {!token && (
        <Link
          href="/admin"
          className="mb-4 inline-flex items-center gap-1 text-sm font-medium text-[var(--muted)]"
        >
          <ArrowLeft size={16} weight="bold" /> 접수함
        </Link>
      )}

      <header className="mb-4 grid gap-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h1 className="text-xl font-bold">
              {claim.machineLabel} · {claim.issueLabel}
            </h1>
            <p className="mt-0.5 text-sm text-[var(--muted)]">
              접수번호 {claim.receiptCode} · {dateTime(claim.createdAt)}
            </p>
          </div>
          <span className="shrink-0 text-2xl font-bold tabular-nums">
            {krw(claim.amountKrw)}
          </span>
        </div>
        <div>
          <Badge tone={toneForStatus(claim.status)}>{claim.statusLabel}</Badge>
        </div>
      </header>

      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
          {error}
        </p>
      )}

      <div className="grid gap-4">
        <RiskReasons reasons={claim.riskReasons} />

        {/* 사진. 탭하면 전체화면 — 조작 여부를 보려면 확대가 필수다. */}
        {claim.photoIds.length > 0 && (
          <ul className="grid grid-cols-3 gap-2">
            {claim.photoIds.map((pid, i) => {
              // img 태그는 헤더를 못 붙이므로 사진만 쿼리로 토큰을 보낸다.
              const src =
                `/api/admin/claims/${claim.id}/photos/${pid}` +
                (token ? `?t=${encodeURIComponent(token)}` : "");
              return (
                <li key={pid}>
                  <button type="button" onClick={() => setZoom(src)} className="block w-full">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={src}
                      alt={`첨부 사진 ${i + 1}`}
                      className="aspect-square w-full rounded-lg border border-[var(--line)] object-cover"
                    />
                  </button>
                </li>
              );
            })}
          </ul>
        )}

        {claim.description && (
          <div className="surface p-3.5">
            <p className="mb-1 text-xs font-semibold text-[var(--muted)]">손님 설명</p>
            <p className="whitespace-pre-wrap text-sm">{claim.description}</p>
          </div>
        )}

        {/* 연락처와 이력 */}
        <div className="surface grid gap-3 p-3.5">
          <Row label="연락처" value={fmtPhone(claim.phone)} />
          <Row label="환불 계좌" value={`${claim.bankName} ${claim.accountNo}`} />
          <Row label="예금주" value={claim.holder} />
          <hr className="border-[var(--line)]" />
          <Row label="이 번호의 30일 신고" value={`${claim.phoneClaims30d}건`} />
          <Row label="30일 누적 환불" value={krw(claim.phonePaidTotal30d)} />
        </div>

        {/*
          고액 건은 통화가 주요 동작이다. 사진 몇 장보다 한 통의 전화가
          확실하고, 사장님이 번호를 손으로 옮겨 적지 않게 한다.
        */}
        {claim.callRecommended && claim.status !== "paid" && claim.status !== "rejected" && (
          <a
            href={`tel:${claim.phone}`}
            className="flex h-14 items-center justify-center gap-2 rounded-lg bg-accent-600 font-semibold text-white"
          >
            <PhoneCall size={20} weight="fill" />
            {fmtPhone(claim.phone)} 로 전화하기
          </a>
        )}

        {/* 처리 버튼 */}
        <div className="grid gap-2">
          {can("approved") && (
            <Button size="lg" full loading={busy} onClick={() => void act("approve")}>
              승인
            </Button>
          )}
          {claim.status === "approved" && (
            <Button size="lg" full onClick={() => setPayoutOpen(true)}>
              환불 보내기
            </Button>
          )}
          {can("on_hold") && (
            <Button variant="secondary" full loading={busy} onClick={() => void act("hold", "사장님 보류")}>
              나중에 보기 (보류)
            </Button>
          )}
          {can("rejected") && (
            <Button variant="ghost" full onClick={() => setConfirmReject(true)}>
              거절
            </Button>
          )}
        </div>

        <Timeline events={claim.events} />
      </div>

      {/* 되돌릴 수 없는 동작은 확인을 거친다. */}
      {confirmReject && (
        <ConfirmSheet
          title="이 신고를 거절할까요?"
          body="거절한 건은 되돌릴 수 없습니다. 손님에게는 별도로 안내해주세요."
          confirmLabel="거절하기"
          busy={busy}
          onCancel={() => setConfirmReject(false)}
          onConfirm={() => void act("reject", "사장님 거절")}
        />
      )}

      {payoutOpen && (
        <PayoutSheet
          claimId={claim.id}
          token={token}
          onClose={() => setPayoutOpen(false)}
          onPaid={() => {
            setPayoutOpen(false);
            void load();
          }}
        />
      )}

      {zoom && (
        <button
          type="button"
          onClick={() => setZoom(null)}
          className="fixed inset-0 z-50 grid place-items-center bg-ink-950/95 p-4"
          aria-label="사진 닫기"
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={zoom} alt="첨부 사진 확대" className="max-h-full max-w-full object-contain" />
          <X size={24} weight="bold" className="absolute right-4 top-4 text-white" />
        </button>
      )}
    </main>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <span className="text-sm text-[var(--muted)]">{label}</span>
      <span className="text-right text-sm font-semibold">{value}</span>
    </div>
  );
}

const ACTION_LABEL: Record<string, string> = {
  create: "손님이 접수",
  view_contact: "계좌·연락처 열람",
  transition: "상태 변경",
};

/** 누가 언제 무엇을 했는지. 돈이 오가므로 기록이 남아야 한다. */
function Timeline({ events }: { events: ClaimDetail["events"] }) {
  if (events.length === 0) return null;
  return (
    <details className="surface p-3.5">
      <summary className="cursor-pointer text-sm font-semibold">처리 기록</summary>
      <ul className="mt-3 grid gap-2">
        {events.map((e, i) => (
          <li key={i} className="grid gap-0.5 text-sm">
            <span className="text-[var(--fg)]">
              {ACTION_LABEL[e.action] ?? e.action}
              {e.note && ` — ${e.note}`}
            </span>
            <span className="text-xs text-[var(--muted)]">{dateTime(e.at)}</span>
          </li>
        ))}
      </ul>
    </details>
  );
}

function ConfirmSheet({
  title,
  body,
  confirmLabel,
  busy,
  onCancel,
  onConfirm,
}: {
  title: string;
  body: string;
  confirmLabel: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50">
      <div className="grid w-full max-w-lg gap-4 rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
        <div className="grid gap-1">
          <h2 className="text-lg font-bold">{title}</h2>
          <p className="text-sm text-[var(--muted)]">{body}</p>
        </div>
        <div className="grid gap-2">
          <Button variant="danger" size="lg" full loading={busy} onClick={onConfirm}>
            {confirmLabel}
          </Button>
          <Button variant="ghost" full onClick={onCancel}>
            취소
          </Button>
        </div>
      </div>
    </div>
  );
}
