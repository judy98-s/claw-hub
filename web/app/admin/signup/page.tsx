"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { ApiError, get, post } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";

const MIN_PASSWORD = 8;

export default function SignupPage() {
  const router = useRouter();
  const [storeName, setStoreName] = useState("");
  const [phone, setPhone] = useState("");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // 서버가 가입을 닫아 뒀을 수 있다. 눌러 보고 403을 받기보다 먼저 알려준다.
  const [open, setOpen] = useState<boolean | null>(null);
  useEffect(() => {
    get<{ open: boolean }>("/api/public/signup")
      .then((r) => setOpen(r.open))
      // 상태를 못 읽으면 막지 않는다. 제출하면 서버가 어차피 거절한다.
      .catch(() => setOpen(true));
  }, []);

  const tooShort = password.length > 0 && password.length < MIN_PASSWORD;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await post("/api/public/signup", { storeName, phone, email, password, name });
      // 가입하면 서버가 세션 쿠키를 함께 내려준다. 방금 정한 비밀번호를
      // 한 번 더 치게 하는 단계는 이탈만 만든다.
      router.replace("/admin");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "가입에 실패했습니다.");
      setBusy(false);
    }
  }

  if (open === false) {
    return (
      <main className="mx-auto grid min-h-[100dvh] max-w-sm place-items-center px-4">
        <div className="grid gap-4 text-center">
          <h1 className="text-xl font-bold">가입을 받지 않습니다</h1>
          <p className="text-sm text-neutral-600">
            이 서버는 회원가입이 닫혀 있습니다. 매장 관리자에게 계정을 요청해주세요.
          </p>
          <Link href="/admin/login" className="text-sm font-medium underline">
            로그인으로
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-sm place-items-center px-4 py-10">
      <form onSubmit={submit} className="grid w-full gap-5">
        <div className="grid gap-1">
          <h1 className="text-xl font-bold">매장 만들기</h1>
          <p className="text-sm text-neutral-600">
            매장과 사장님 계정을 함께 만듭니다.
          </p>
        </div>

        <Field label="매장 이름" required hint="손님 화면에 보입니다. 나중에 바꿀 수 있습니다.">
          <Input
            value={storeName}
            onChange={(e) => setStoreName(e.target.value)}
            maxLength={60}
            required
            autoFocus
          />
        </Field>

        <Field
          label="매장 대표번호"
          hint="손님이 기계를 못 찾을 때 안내됩니다. 비워도 됩니다."
        >
          <Input
            type="tel"
            inputMode="tel"
            autoComplete="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </Field>

        <Field label="이메일" required hint="로그인 아이디입니다. 나중에 바꿀 수 없습니다.">
          <Input
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </Field>

        <Field
          label="비밀번호"
          required
          error={tooShort ? `${MIN_PASSWORD}자 이상이어야 합니다.` : undefined}
          hint="나중에 바꿀 수 없습니다. 비밀번호 관리자에 저장해두세요."
        >
          <Input
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={MIN_PASSWORD}
            maxLength={72}
            required
          />
        </Field>

        <Field label="표시 이름" hint="비우면 '사장님'이 됩니다.">
          <Input
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>

        {error && (
          <p role="alert" className="text-sm text-stop-600">
            {error}
          </p>
        )}

        <Button type="submit" size="lg" full loading={busy} disabled={tooShort}>
          매장 만들기
        </Button>

        <p className="text-center text-sm text-neutral-600">
          이미 계정이 있나요?{" "}
          <Link href="/admin/login" className="font-medium underline">
            로그인
          </Link>
        </p>
      </form>
    </main>
  );
}
