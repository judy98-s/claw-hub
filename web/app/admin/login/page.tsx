"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { ApiError, post } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

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
    <main className="mx-auto grid min-h-[100dvh] max-w-sm place-items-center px-4">
      <form onSubmit={submit} className="grid w-full gap-5">
        <h1 className="text-xl font-bold">claw-hub</h1>

        <Field label="이메일">
          <Input
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
          />
        </Field>

        <Field label="비밀번호" error={error}>
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>

        <Button type="submit" size="lg" full loading={busy}>
          로그인
        </Button>
      </form>
    </main>
  );
}
