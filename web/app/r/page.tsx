"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";

/**
 * 기계 코드 직접 입력.
 *
 * QR 스티커가 긁히거나 젖어서 안 읽히는 일이 실제로 생긴다. 스티커에
 * 인쇄된 코드를 손으로 칠 수 있는 곳이 없으면 손님은 거기서 끝이다.
 *
 * 코드 알파벳에서 0/O, 1/I/L 을 뺐지만, 손님은 여전히 0을 칠 수 있다.
 * 서버가 NormalizeMachineCode 로 보정하므로 그대로 보낸다.
 */
export default function CodeEntryPage() {
  const router = useRouter();
  const [code, setCode] = useState("");

  return (
    <main className="mx-auto grid min-h-[100dvh] max-w-sm place-items-center px-4">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          router.push(`/r/${code.trim()}`);
        }}
        className="grid w-full gap-5"
      >
        <div className="grid gap-1.5">
          <h1 className="text-xl font-bold">기계 코드 입력</h1>
          <p className="text-sm text-[var(--muted)]">
            기계에 붙은 스티커의 6자리 코드를 입력해주세요.
          </p>
        </div>

        <Field label="기계 코드" required>
          <Input
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            placeholder="ABCD23"
            autoCapitalize="characters"
            autoComplete="off"
            spellCheck={false}
            maxLength={8}
            autoFocus
            className="text-center font-mono text-xl tracking-[0.3em]"
          />
        </Field>

        <Button type="submit" size="lg" full disabled={code.trim().length < 6}>
          문제 신고하러 가기
        </Button>
      </form>
    </main>
  );
}
