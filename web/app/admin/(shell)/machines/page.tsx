"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, QrCode, Wrench } from "@phosphor-icons/react";

import { ApiError, get, post } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { STORES, type StoreType } from "@/lib/stores";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { QrSheet } from "./qr-sheet";

type Machine = {
  id: string;
  code: string;
  label: string;
  location: string;
  active: boolean;
  claims24h: number;
  claims7d: number;
  openClaims: number;
  daily7d: number[];
  qrTarget: string;
  needsCheck: boolean;
};

export default function MachinesPage() {
  const router = useRouter();
  const [machines, setMachines] = useState<Machine[]>([]);
  const [error, setError] = useState("");
  const [adding, setAdding] = useState(false);
  const [label, setLabel] = useState("");
  const [location, setLocation] = useState("");
  const [busy, setBusy] = useState(false);
  const [qrFor, setQrFor] = useState<Machine | null>(null);
  // 스티커 바탕색과 캐릭터가 업종으로 갈린다. 홈에서 이미 내려주는 값이다.
  const [storeType, setStoreType] = useState<StoreType>("claw");

  const load = useCallback(async () => {
    try {
      const res = await get<{ machines: Machine[] }>("/api/admin/machines");
      setMachines(res.machines ?? []);
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
    get<{ storeType?: string }>("/api/admin/home")
      .then((h) => {
        if (h.storeType && h.storeType in STORES) setStoreType(h.storeType as StoreType);
      })
      // 업종을 못 읽어도 화면은 떠야 한다. 인형뽑기로 그린다.
      .catch(() => {});
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await post("/api/admin/machines", { label, location });
      setLabel("");
      setLocation("");
      setAdding(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "추가에 실패했습니다.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-lg px-4 pt-6">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold">기계</h1>
        <Button variant="secondary" onClick={() => setAdding((v) => !v)}>
          <Plus size={16} weight="bold" /> 추가
        </Button>
      </div>

      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
          {error}
        </p>
      )}

      {adding && (
        <form onSubmit={create} className="surface mb-4 grid gap-3 p-4">
          <Field label="기계 이름" required hint="손님이 스티커에서 보게 될 이름입니다.">
            <Input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="3번 기계"
              maxLength={50}
              required
              autoFocus
            />
          </Field>
          <Field label="위치">
            <Input
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="입구 왼쪽"
            />
          </Field>
          <Button type="submit" full loading={busy}>
            추가하고 QR 만들기
          </Button>
        </form>
      )}

      {machines.length === 0 ? (
        <p className="py-16 text-center text-[var(--muted)]">
          아직 등록한 기계가 없습니다.
          <br />
          기계를 추가하면 QR 스티커를 인쇄할 수 있습니다.
        </p>
      ) : (
        <ul className="grid gap-2">
          {machines.map((m) => (
            <li key={m.id} className="surface p-3.5">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="flex items-center gap-1.5 font-semibold">
                    {m.label}
                    {!m.active && <Badge>비활성</Badge>}
                  </p>
                  <p className="mt-0.5 text-sm text-[var(--muted)]">
                    {m.location || "위치 미지정"} · 코드 {m.code}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setQrFor(m)}
                  aria-label={`${m.label} QR 보기`}
                  className="shrink-0 rounded-lg border border-[var(--line)] p-2.5"
                >
                  <QrCode size={20} weight="regular" />
                </button>
              </div>

              <div className="mt-3 flex items-center gap-3">
                <Sparkline values={m.daily7d} />
                <div className="flex flex-wrap gap-1.5">
                  {m.needsCheck && (
                    <Badge tone="stop" icon={<Wrench size={12} weight="fill" />}>
                      점검 필요
                    </Badge>
                  )}
                  <Badge>24시간 {m.claims24h}건</Badge>
                  {m.openClaims > 0 && <Badge tone="warn">처리 대기 {m.openClaims}</Badge>}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {qrFor && <QrSheet machine={qrFor} storeType={storeType} onClose={() => setQrFor(null)} />}
    </main>
  );
}

/**
 * 최근 7일 접수 추이.
 *
 * 서버가 접수 없는 날도 0으로 채워 보낸다. 빈 날을 건너뛰면 한산한 주와
 * 바쁜 주가 똑같아 보인다.
 */
function Sparkline({ values }: { values: number[] }) {
  const max = Math.max(1, ...values);
  // 하루 3건 이상이면 stop 색. 주간 합계가 같아도 하루에 몰린 날은
  // 기계 고장이고, 흩어진 주는 그냥 바쁜 주다. 막대가 같은 색이면
  // 그 둘을 구분할 수 없다.
  const BUSY_DAY = 3;
  return (
    <div
      className="flex h-8 shrink-0 items-end gap-0.5"
      role="img"
      aria-label={`최근 7일 접수: ${values.join(", ")}건`}
    >
      {values.map((v, i) => (
        <span
          key={i}
          className={`w-[5px] rounded-sm ${
            v >= BUSY_DAY ? "bg-stop-600" : v > 0 ? "bg-pink" : "bg-[#E5E2DC]"
          }`}
          style={{ height: `${Math.max(12, (v / max) * 100)}%` }}
        />
      ))}
    </div>
  );
}
