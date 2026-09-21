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

export default function SettingsPage() {
  const router = useRouter();

  const [me, setMe] = useState<Profile | null>(null);
  const [shop, setShop] = useState<StoreInfo | null>(null);
  const [users, setUsers] = useState<Profile[]>([]);
  const [meId, setMeId] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [m, s, u] = await Promise.all([
        get<Profile>("/api/admin/me"),
        get<StoreInfo>("/api/admin/store"),
        get<{ users: Profile[]; meId: string }>("/api/admin/users"),
      ]);
      setMe(m);
      setShop(s);
      setUsers(u.users ?? []);
      setMeId(u.meId);
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
