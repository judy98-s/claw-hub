"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { ApiError, get, post } from "@/lib/api";
import { Mascot } from "@/components/mascot";
import { ArcadeButton, ArcadeError, ArcadeInput } from "@/components/ui/arcade";

/*
  로그인.

  핸드오프에 이 화면은 없다(§6 은 손님 폼·대시보드·랜딩만 다룬다).
  같은 언어로 맞춰 그렸다 — 먹선 2px, 각진 그림자, 노란 주 버튼,
  포커스는 분홍 그림자.

  여기만 다르게 둔 것: 캐릭터를 sleep 으로 둔다. 로그인 화면은 아직
  아무 일도 일어나지 않은 자리고, calm 으로 쳐다보면 재촉하는 느낌이 된다.
*/

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // 가입이 닫힌 서버에서는 링크를 아예 띄우지 않는다.
  const [signupOpen, setSignupOpen] = useState(false);
  useEffect(() => {
    get<{ open: boolean }>("/api/public/signup")
      .then((r) => setSignupOpen(r.open))
      .catch(() => setSignupOpen(false));
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await post("/api/admin/login", { email, password });
      router.replace("/admin");
    } catch (err) {
      // 서버가 이메일/비밀번호 중 무엇이 틀렸는지 구분해주지 않는다.
      // 구분하면 "이 이메일은 가입되어 있다"가 새어 나간다.
      setError(err instanceof ApiError ? err.message : "로그인에 실패했습니다.");
      setBusy(false);
    }
  }

  return (
    <main className="grid min-h-[100dvh] place-items-center bg-paper px-5 py-8 text-ink">
      <div className="grid w-full max-w-sm gap-7">
        <div className="grid justify-items-center gap-2">
          <Mascot mood="sleep" size={84} />
          <Link href="/" className="text-xl font-extrabold">
            claw-hub
          </Link>
        </div>

        <form onSubmit={submit} className="grid gap-4">
          <label className="grid gap-1.5">
            <span className="text-[15px] font-extrabold">이메일</span>
            <ArcadeInput
              type="email"
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
            />
          </label>

          <label className="grid gap-1.5">
            <span className="text-[15px] font-extrabold">비밀번호</span>
            <ArcadeInput
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-invalid={error ? true : undefined}
              required
            />
          </label>

          {error && <ArcadeError>{error}</ArcadeError>}

          <ArcadeButton type="submit" size="lg" full loading={busy}>
            로그인
          </ArcadeButton>
        </form>

        {signupOpen && (
          <p className="text-center text-sm text-ink-soft">
            매장이 아직 없나요?{" "}
            <Link href="/admin/signup" className="font-bold underline underline-offset-4">
              매장 만들기
            </Link>
          </p>
        )}
      </div>
    </main>
  );
}
