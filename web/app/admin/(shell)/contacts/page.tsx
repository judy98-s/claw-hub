"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { ApiError, get, patch } from "@/lib/api";
import { ago, krw } from "@/lib/format";
import { Badge, type Tone } from "@/components/ui/badge";

type Contact = {
  identityHash: string;
  phoneMasked: string;
  claimCount: number;
  paidCount: number;
  paidTotalKrw: number;
  manualStatus: string;
  note: string;
  lastClaimAt: string;
};

const STATUS: { value: string; label: string; tone: Tone; help: string }[] = [
  { value: "normal", label: "일반", tone: "neutral", help: "자동 판단에 맡깁니다." },
  { value: "watch", label: "관찰", tone: "warn", help: "신고가 오면 확인 대상으로 올립니다." },
  { value: "blocked", label: "차단", tone: "stop", help: "신고가 오면 자동 보류합니다." },
];

export default function ContactsPage() {
  const router = useRouter();
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [error, setError] = useState("");
  const [busyHash, setBusyHash] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await get<{ contacts: Contact[] }>("/api/admin/contacts");
      setContacts(res.contacts ?? []);
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

  async function setStatus(hash: string, status: string) {
    setBusyHash(hash);
    try {
      await patch(`/api/admin/contacts/${hash}`, { status, kind: "phone" });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "변경에 실패했습니다.");
    } finally {
      setBusyHash("");
    }
  }

  return (
    <main className="mx-auto max-w-lg px-4 pt-6">
      <h1 className="mb-1 text-xl font-bold">연락처</h1>
      <p className="mb-4 text-sm text-[var(--muted)]">
        신고 횟수가 많은 순입니다. 차단해도 접수는 받되 자동으로 보류됩니다.
      </p>

      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
          {error}
        </p>
      )}

      {contacts.length === 0 ? (
        <p className="py-16 text-center text-[var(--muted)]">아직 신고 이력이 없습니다.</p>
      ) : (
        <ul className="grid gap-2">
          {contacts.map((c) => (
            <li key={c.identityHash} className="surface p-3.5">
              <div className="mb-2 flex items-start justify-between gap-3">
                <div>
                  <p className="font-mono font-semibold">{c.phoneMasked}</p>
                  <p className="mt-0.5 text-sm text-[var(--muted)]">
                    신고 {c.claimCount}건 · 환불 {c.paidCount}건 · {krw(c.paidTotalKrw)}
                  </p>
                  <p className="text-xs text-[var(--muted)]">
                    마지막 신고 {ago(c.lastClaimAt)}
                  </p>
                </div>
                {c.manualStatus !== "normal" && (
                  <Badge tone={c.manualStatus === "blocked" ? "stop" : "warn"}>
                    {STATUS.find((s) => s.value === c.manualStatus)?.label}
                  </Badge>
                )}
              </div>

              <div className="grid grid-cols-3 gap-1.5">
                {STATUS.map((s) => (
                  <button
                    key={s.value}
                    type="button"
                    disabled={busyHash === c.identityHash}
                    onClick={() => void setStatus(c.identityHash, s.value)}
                    title={s.help}
                    className={[
                      "h-9 rounded-md text-xs font-semibold transition-colors duration-150",
                      "disabled:opacity-50",
                      c.manualStatus === s.value
                        ? "bg-accent-600 text-white"
                        : "bg-[var(--surface-sunken)] text-[var(--muted)]",
                    ].join(" ")}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
