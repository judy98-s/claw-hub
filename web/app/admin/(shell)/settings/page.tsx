"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Plus, SignOut } from "@phosphor-icons/react";

import { ApiError, get, patch, post } from "@/lib/api";
import { phone as fmtPhone } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";

type Profile = {
  id: string;
  email: string;
  name: string;
  phone: string;
  active: boolean;
};

type StoreInfo = { id: string; name: string; phone: string };

type Provider = { id: string; label: string; prefills: boolean; note: string };
type Bank = { code: string; name: string };

type PayoutSettings = {
  providers: Provider[];
  banks: Bank[];
  provider: string;
  template: string;
  bankCode: string;
  account: string;
};

export default function SettingsPage() {
  const router = useRouter();

  const [me, setMe] = useState<Profile | null>(null);
  const [shop, setShop] = useState<StoreInfo | null>(null);
  const [users, setUsers] = useState<Profile[]>([]);
  const [payout, setPayout] = useState<PayoutSettings | null>(null);
  const [meId, setMeId] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [m, s, u, p] = await Promise.all([
        get<Profile>("/api/admin/me"),
        get<StoreInfo>("/api/admin/store"),
        get<{ users: Profile[]; meId: string }>("/api/admin/users"),
        get<PayoutSettings>("/api/admin/payout-settings"),
      ]);
      setMe(m);
      setShop(s);
      setUsers(u.users ?? []);
      setMeId(u.meId);
      setPayout(p);
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

  async function logout() {
    await post("/api/admin/logout").catch(() => {});
    router.replace("/admin/login");
  }

  if (!me || !shop) {
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

  return (
    <main className="mx-auto max-w-lg px-4 pb-8 pt-6">
      <h1 className="mb-5 text-xl font-bold">설정</h1>

      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
          {error}
        </p>
      )}

      <div className="grid gap-6">
        <MyProfile me={me} onSaved={setMe} onError={setError} />
        <StoreSettings shop={shop} onSaved={setShop} onError={setError} />
        {payout && <PayoutSection value={payout} onSaved={setPayout} onError={setError} />}
        <StaffList users={users} meId={meId} onChanged={load} onError={setError} />

        <Button variant="ghost" full onClick={() => void logout()}>
          <SignOut size={18} weight="regular" /> 로그아웃
        </Button>
      </div>
    </main>
  );
}

function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <section className="surface grid gap-4 p-4">
      <div className="grid gap-0.5">
        <h2 className="font-bold">{title}</h2>
        {hint && <p className="text-sm text-[var(--muted)]">{hint}</p>}
      </div>
      {children}
    </section>
  );
}

function MyProfile({
  me,
  onSaved,
  onError,
}: {
  me: Profile;
  onSaved: (p: Profile) => void;
  onError: (m: string) => void;
}) {
  const [name, setName] = useState(me.name);
  // 서버는 숫자만 저장한다. 화면에서는 읽기 좋게 하이픈을 넣어 보여주고,
  // 저장할 때 서버가 다시 숫자만 남긴다.
  const [phone, setPhone] = useState(fmtPhone(me.phone));
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  const dirty = name !== me.name || phone !== fmtPhone(me.phone);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const p = await patch<Profile>("/api/admin/me", { name, phone });
      onSaved(p);
      setPhone(fmtPhone(p.phone));
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      onError("");
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "저장하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Section title="내 계정">
      <form onSubmit={save} className="grid gap-3">
        <Field label="이름">
          <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={40} />
        </Field>
        <Field label="연락처" hint="선택 사항입니다. 직원이 여러 명일 때 누구인지 알아보기 위한 것입니다.">
          <Input
            type="tel"
            inputMode="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="010-1234-5678"
          />
        </Field>
        {/* 이메일은 로그인 수단이라 여기서 바꾸지 않는다. */}
        <Field label="로그인 이메일">
          <Input value={me.email} disabled readOnly />
        </Field>
        <Button type="submit" full loading={busy} disabled={!dirty}>
          {saved ? "저장됨" : "저장"}
        </Button>
      </form>
    </Section>
  );
}

function StoreSettings({
  shop,
  onSaved,
  onError,
}: {
  shop: StoreInfo;
  onSaved: (s: StoreInfo) => void;
  onError: (m: string) => void;
}) {
  const [name, setName] = useState(shop.name);
  const [phone, setPhone] = useState(fmtPhone(shop.phone));
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  const dirty = name !== shop.name || phone !== fmtPhone(shop.phone);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const s = await patch<StoreInfo>("/api/admin/store", { name, phone });
      onSaved(s);
      setPhone(fmtPhone(s.phone));
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      onError("");
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "저장하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Section title="매장">
      <form onSubmit={save} className="grid gap-3">
        <Field label="매장 이름" hint="손님이 신고 화면에서 보게 됩니다.">
          <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
        </Field>
        <Field
          label="매장 대표번호"
          hint="QR이 안 읽히거나 기계를 못 찾았을 때 손님에게 안내되는 번호입니다."
        >
          <Input
            type="tel"
            inputMode="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="02-1234-5678"
          />
        </Field>
        <Button type="submit" full loading={busy} disabled={!dirty}>
          {saved ? "저장됨" : "저장"}
        </Button>
      </form>
    </Section>
  );
}

function PayoutSection({
  value,
  onSaved,
  onError,
}: {
  value: PayoutSettings;
  onSaved: (p: PayoutSettings) => void;
  onError: (m: string) => void;
}) {
  const [provider, setProvider] = useState(value.provider);
  const [template, setTemplate] = useState(value.template);
  const [bankCode, setBankCode] = useState(value.bankCode);
  const [account, setAccount] = useState(value.account);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  const chosen = value.providers.find((p) => p.id === provider);
  const dirty =
    provider !== value.provider ||
    template !== value.template ||
    bankCode !== value.bankCode ||
    account !== value.account;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const next = await patch<PayoutSettings>("/api/admin/payout-settings", {
        provider,
        template,
        bankCode,
        account,
      });
      onSaved(next);
      setTemplate(next.template);
      setAccount(next.account);
      setBankCode(next.bankCode);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      onError("");
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "저장하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Section title="송금" hint="환불을 보낼 때 어떤 앱을 열지 정합니다.">
      <form onSubmit={save} className="grid gap-4">
        <div className="grid gap-2">
          {value.providers.map((p) => {
            const on = provider === p.id;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => setProvider(p.id)}
                aria-pressed={on}
                className={[
                  "grid gap-0.5 rounded-lg border p-3 text-left transition-colors duration-150",
                  on
                    ? "border-accent-600 bg-[var(--tone-accent-bg)]"
                    : "border-[var(--line)] bg-[var(--surface)]",
                ].join(" ")}
              >
                <span className="flex items-center gap-1.5 font-semibold">
                  {p.label}
                  {p.prefills && p.id !== "custom" && <Badge tone="ok">자동 입력</Badge>}
                  {!p.prefills && p.id !== "none" && <Badge tone="warn">직접 입력</Badge>}
                </span>
                <span className="text-xs text-[var(--muted)]">{p.note}</span>
              </button>
            );
          })}
        </div>

        {provider === "custom" && (
          <Field label="딥링크 주소" required>
            <Input
              value={template}
              onChange={(e) => setTemplate(e.target.value)}
              placeholder="myapp://send?bank={bankShort}&amount={amount}"
              spellCheck={false}
            />
          </Field>
        )}

        {/*
          출금 계좌는 딥링크에 넣지 않는다. 송금 앱은 어느 계좌에서 보낼지를
          URL 로 받지 않고 로그인한 사람의 주계좌를 쓴다. 직원이 여러 명일 때
          "어느 계좌에서 나가야 하는지"를 송금 화면에 띄워주기 위한 값이다.
        */}
        <div className="grid gap-3 border-t border-[var(--line)] pt-4">
          <div className="grid gap-0.5">
            <p className="text-sm font-semibold">출금 계좌 (선택)</p>
            <p className="text-sm text-[var(--muted)]">
              환불이 나가야 할 계좌입니다. 송금 화면에 표시만 됩니다 — 앱이 이
              계좌를 자동으로 고르지는 못합니다. 직원이 여러 명일 때 개인 계좌에서
              나가는 일을 막아줍니다.
            </p>
          </div>
          <Field label="은행">
            <select
              value={bankCode}
              onChange={(e) => setBankCode(e.target.value)}
              className="h-12 w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3 text-base text-[var(--fg)]"
            >
              <option value="">선택 안 함</option>
              {value.banks.map((b) => (
                <option key={b.code} value={b.code}>
                  {b.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label="계좌번호">
            <Input
              inputMode="numeric"
              value={account}
              onChange={(e) => setAccount(e.target.value.replace(/[^0-9-]/g, ""))}
              placeholder="- 없이 입력"
            />
          </Field>
        </div>

        <Button type="submit" full loading={busy} disabled={!dirty}>
          {saved ? "저장됨" : "저장"}
        </Button>

        {chosen && !chosen.prefills && chosen.id !== "none" && (
          <p className="text-sm text-[var(--tone-warn-fg)]">
            {chosen.label}은 계좌와 금액이 자동으로 채워지지 않습니다. 송금 화면의
            계좌번호 복사를 함께 쓰세요.
          </p>
        )}
      </form>
    </Section>
  );
}

function StaffList({
  users,
  meId,
  onChanged,
  onError,
}: {
  users: Profile[];
  meId: string;
  onChanged: () => void;
  onError: (m: string) => void;
}) {
  const [adding, setAdding] = useState(false);
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await post("/api/admin/users", { email, name, phone, password });
      setEmail("");
      setName("");
      setPhone("");
      setPassword("");
      setAdding(false);
      onChanged();
      onError("");
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "추가하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  }

  async function toggle(id: string, active: boolean) {
    try {
      await patch(`/api/admin/users/${id}`, { active });
      onChanged();
      onError("");
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "변경하지 못했습니다.");
    }
  }

  return (
    <Section
      title="직원"
      hint="추가된 계정은 사장님과 같은 것을 할 수 있습니다. 권한 구분은 아직 없습니다."
    >
      <ul className="grid gap-2">
        {users.map((u) => (
          <li
            key={u.id}
            className="flex items-center justify-between gap-3 rounded-lg bg-[var(--surface-sunken)] p-3"
          >
            <div className="min-w-0">
              <p className="flex items-center gap-1.5 font-semibold">
                <span className="truncate">{u.name}</span>
                {u.id === meId && <Badge tone="accent">나</Badge>}
                {!u.active && <Badge>비활성</Badge>}
              </p>
              <p className="truncate text-sm text-[var(--muted)]">
                {u.email}
                {u.phone && ` · ${fmtPhone(u.phone)}`}
              </p>
            </div>
            {/* 자기 계정은 끌 수 없다. 끄는 순간 로그아웃되고 되돌릴 사람이 없을 수 있다. */}
            {u.id !== meId && (
              <Button
                variant="ghost"
                onClick={() => void toggle(u.id, !u.active)}
                className="shrink-0"
              >
                {u.active ? "비활성화" : "다시 활성화"}
              </Button>
            )}
          </li>
        ))}
      </ul>

      {adding ? (
        <form onSubmit={create} className="grid gap-3 border-t border-[var(--line)] pt-4">
          <Field label="이름" required>
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={40} required autoFocus />
          </Field>
          <Field label="로그인 이메일" required>
            <Input
              type="email"
              autoComplete="off"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </Field>
          <Field label="연락처">
            <Input
              type="tel"
              inputMode="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="010-1234-5678"
            />
          </Field>
          <Field label="비밀번호" required hint="8자 이상. 직원에게 직접 전달하세요.">
            <Input
              type="text"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              required
            />
          </Field>
          <div className="grid gap-2">
            <Button type="submit" full loading={busy}>
              추가
            </Button>
            <Button variant="ghost" full onClick={() => setAdding(false)}>
              취소
            </Button>
          </div>
        </form>
      ) : (
        <Button variant="secondary" full onClick={() => setAdding(true)}>
          <Plus size={16} weight="bold" /> 직원 추가
        </Button>
      )}
    </Section>
  );
}
