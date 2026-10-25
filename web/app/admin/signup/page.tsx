"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowRight, CaretLeft } from "@phosphor-icons/react";

import { ApiError, get, post } from "@/lib/api";
import { Mascot } from "@/components/mascot";
import { STORE_TYPES, STORES, type StoreType } from "@/lib/stores";
import { ArcadeButton, ArcadeError, ArcadeInput, ArcadeNote } from "@/components/ui/arcade";

const MIN_PASSWORD = 8;

/*
  매장 만들기.

  두 화면으로 나눈다. 업종 하나만 먼저 묻고, 그 다음에 나머지를 받는다.

  왜 나누는가: 업종은 **손님이 보는 것 거의 전부**를 가른다 — 캐릭터,
  증상 목록, 빠른 금액, 스티커 문구, 기계를 부르는 말. 긴 폼 중간의
  셀렉트 박스로 놓으면 아무거나 고르고 지나가는데, 그러면 손님 화면이
  통째로 엉뚱해지고 사장님은 왜 그런지 모른다.

  타일에 캐릭터를 띄우는 이유도 같다. "무인세탁"이라는 글자보다
  강아지 그림이 "내 가게가 이거다" 를 빠르게 만든다.
*/

export default function SignupPage() {
  const router = useRouter();

  const [step, setStep] = useState<0 | 1>(0);
  const [storeType, setStoreType] = useState<StoreType>("claw");

  const [storeName, setStoreName] = useState("");
  const [phone, setPhone] = useState("");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // 서버가 가입을 닫아 뒀을 수 있다. 눌러 보고 403을 받기보다 먼저 알린다.
  const [open, setOpen] = useState<boolean | null>(null);
  useEffect(() => {
    get<{ open: boolean }>("/api/public/signup")
      .then((r) => setOpen(r.open))
      // 상태를 못 읽으면 막지 않는다. 제출하면 서버가 어차피 거절한다.
      .catch(() => setOpen(true));
  }, []);

  const tooShort = password.length > 0 && password.length < MIN_PASSWORD;
  const cfg = STORES[storeType];

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await post("/api/public/signup", { storeName, phone, storeType, email, password, name });
      // 서버가 세션 쿠키를 함께 내려준다. 방금 정한 비밀번호를 한 번 더
      // 치게 하는 단계는 이탈만 만든다.
      router.replace("/admin");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "가입에 실패했습니다.");
      setBusy(false);
    }
  }

  if (open === false) {
    return (
      <Shell>
        <div className="grid justify-items-center gap-4 text-center">
          <Mascot mood="sleep" size={96} />
          <h1 className="text-2xl font-black">가입을 받지 않습니다</h1>
          <p className="text-ink-soft">
            이 서버는 회원가입이 닫혀 있습니다. 매장 관리자에게 계정을 요청해주세요.
          </p>
          <Link href="/admin/login" className="font-bold underline underline-offset-4">
            로그인으로
          </Link>
        </div>
      </Shell>
    );
  }

  if (step === 0) {
    return (
      <Shell>
        <div className="grid gap-6">
          <div className="grid gap-2">
            <h1 className="text-[27px] font-black leading-tight">어떤 매장인가요?</h1>
            <p className="text-ink-soft">
              손님 화면의 안내 문구와 캐릭터가 업종에 맞게 바뀝니다. 나중에 바꿀 수 있습니다.
            </p>
          </div>

          <ul className="grid grid-cols-2 gap-3">
            {STORE_TYPES.map((key) => {
              const s = STORES[key];
              const on = key === storeType;
              return (
                <li key={key}>
                  <button
                    type="button"
                    onClick={() => setStoreType(key)}
                    aria-pressed={on}
                    className={`grid w-full justify-items-center gap-1 rounded-md border-2 border-ink p-3 transition-transform ${
                      on
                        ? "shadow-[4px_4px_0_#16130F]"
                        : "bg-surface active:translate-x-[2px] active:translate-y-[2px]"
                    }`}
                    style={on ? { backgroundColor: s.tint } : undefined}
                  >
                    <Mascot store={key} mode="main" size={72} />
                    <span className="text-[15px] font-extrabold">{s.label}</span>
                    <span
                      className="text-[12px] font-bold"
                      style={{ color: on ? s.accentText : undefined }}
                    >
                      {s.characterName}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>

          <ArcadeButton size="lg" full onClick={() => setStep(1)}>
            다음 <ArrowRight size={18} weight="bold" />
          </ArcadeButton>

          <p className="text-center text-sm text-ink-soft">
            이미 계정이 있나요?{" "}
            <Link href="/admin/login" className="font-bold underline underline-offset-4">
              로그인
            </Link>
          </p>
        </div>
      </Shell>
    );
  }

  return (
    <Shell>
      <form onSubmit={submit} className="grid gap-5">
        <button
          type="button"
          onClick={() => setStep(0)}
          className="-ml-2 flex size-11 items-center justify-center rounded-md"
          aria-label="업종 다시 고르기"
        >
          <CaretLeft size={22} weight="bold" />
        </button>

        <div className="flex items-center gap-3 rounded-md border-2 border-ink p-3" style={{ backgroundColor: cfg.tint }}>
          <Mascot store={storeType} size={44} />
          <div>
            <p className="text-[15px] font-extrabold">{cfg.label}</p>
            <p className="text-[13px] font-bold" style={{ color: cfg.accentText }}>
              {cfg.characterName}가 손님을 맞이합니다
            </p>
          </div>
        </div>

        <Labeled label="매장 이름" required hint="손님 화면에 보입니다. 나중에 바꿀 수 있습니다.">
          <ArcadeInput
            value={storeName}
            onChange={(e) => setStoreName(e.target.value)}
            maxLength={60}
            required
            autoFocus
          />
        </Labeled>

        <Labeled label="매장 대표번호" hint="손님이 기계를 못 찾을 때 안내됩니다. 비워도 됩니다.">
          <ArcadeInput
            type="tel"
            inputMode="tel"
            autoComplete="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </Labeled>

        <Labeled label="이메일" required hint="로그인 아이디입니다. 나중에 바꿀 수 없습니다.">
          <ArcadeInput
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </Labeled>

        <Labeled
          label="비밀번호"
          required
          hint={`${MIN_PASSWORD}자 이상. 나중에 바꿀 수 없으니 비밀번호 관리자에 저장해두세요.`}
        >
          <ArcadeInput
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={MIN_PASSWORD}
            maxLength={72}
            aria-invalid={tooShort || undefined}
            required
          />
        </Labeled>

        <Labeled label="표시 이름" hint="비우면 '사장님'이 됩니다.">
          <ArcadeInput
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Labeled>

        <ArcadeNote>
          계좌와 전화번호는 암호화해서 보관합니다. 사진은 기본 90일 뒤 지워집니다.
        </ArcadeNote>

        {error && <ArcadeError>{error}</ArcadeError>}

        <ArcadeButton type="submit" size="lg" full loading={busy} disabled={tooShort}>
          매장 만들기
        </ArcadeButton>
      </form>
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <main className="min-h-[100dvh] bg-paper px-5 py-8 text-ink">
      <div className="mx-auto grid w-full max-w-md gap-6">
        <Link href="/" className="flex items-center gap-2 text-lg font-extrabold">
          <Mascot size={28} />
          claw-hub
        </Link>
        {children}
      </div>
    </main>
  );
}

/** 레이블·힌트·필수 표시를 묶는다. Field 와 달리 v2 타이포를 쓴다. */
function Labeled({
  label,
  hint,
  required,
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label className="grid gap-1.5">
      <span className="text-[15px] font-extrabold">
        {label}
        {required && (
          <span aria-hidden className="ml-1 text-stop-600">
            *
          </span>
        )}
      </span>
      {children}
      {hint && <span className="text-[13px] text-ink-soft">{hint}</span>}
    </label>
  );
}
