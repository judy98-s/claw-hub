"use client";

import { useEffect, useState } from "react";
import { ArrowSquareOut, Check, Copy, X } from "@phosphor-icons/react";

import { ApiError, get, post } from "@/lib/api";
import { krw } from "@/lib/format";
import { Button } from "@/components/ui/button";

type PayoutLink = { provider: string; label: string; url: string };

type PayoutInfo = {
  links: PayoutLink[];
  bankName: string;
  accountNo: string;
  holder: string;
  amountKrw: number;
  copyText: string;
};

/**
 * 송금 시트.
 *
 * 국내 결제 환경상 서버가 직접 계좌이체를 할 수 없다. 카카오페이·토스
 * 페이먼츠는 결제(수납) API이지 송금 API가 아니고, 실제 송금에는
 * 지급대행·펌뱅킹 계약이 필요하다. 그래서 딥링크로 앱을 열어 사장님이
 * 인증 한 번만 하게 하고, 돌아오면 결과를 묻는다.
 *
 * 딥링크가 없거나(템플릿 미설정) PC에서 열었으면 버튼이 아무것도 하지
 * 않는다. 그래서 계좌 복사와 수동 "송금 완료"를 항상 함께 둔다.
 */
export function PayoutSheet({
  claimId,
  onClose,
  onPaid,
}: {
  claimId: string;
  onClose: () => void;
  onPaid: () => void;
}) {
  const [info, setInfo] = useState<PayoutInfo | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  // 딥링크를 눌러 앱으로 나갔다 돌아온 상태. 이때 "보내셨나요?"를 묻는다.
  const [returned, setReturned] = useState(false);

  useEffect(() => {
    get<PayoutInfo>(`/api/admin/claims/${claimId}/payout-links`)
      .then(setInfo)
      .catch((err) =>
        setError(err instanceof ApiError ? err.message : "송금 정보를 불러오지 못했습니다."),
      );
  }, [claimId]);

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("복사에 실패했습니다. 길게 눌러 직접 복사해주세요.");
    }
  }

  async function markPaid(method: "deeplink" | "manual") {
    setBusy(true);
    try {
      await post(`/api/admin/claims/${claimId}/mark-paid`, { method });
      onPaid();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "기록에 실패했습니다.");
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50">
      <div className="w-full max-w-lg rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold">환불 보내기</h2>
          <button type="button" onClick={onClose} aria-label="닫기" className="p-1">
            <X size={20} weight="bold" />
          </button>
        </div>

        {error && (
          <p role="alert" className="mb-3 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]">
            {error}
          </p>
        )}

        {!info ? (
          <div className="h-40 animate-pulse rounded-lg bg-[var(--surface-sunken)]" />
        ) : (
          <div className="grid gap-4">
            <div className="grid gap-1 rounded-lg bg-[var(--surface-sunken)] p-4">
              <p className="text-2xl font-bold tabular-nums">{krw(info.amountKrw)}</p>
              <p className="text-sm text-[var(--muted)]">
                {info.bankName} {info.accountNo} · {info.holder}
              </p>
            </div>

            {returned ? (
              <div className="grid gap-2">
                <p className="text-center font-semibold">송금을 완료하셨나요?</p>
                <Button size="lg" full loading={busy} onClick={() => void markPaid("deeplink")}>
                  네, 보냈습니다
                </Button>
                <Button variant="ghost" full onClick={() => setReturned(false)}>
                  아직이요
                </Button>
              </div>
            ) : (
              <div className="grid gap-2">
                {info.links.map((l) => (
                  <a
                    key={l.provider}
                    href={l.url}
                    onClick={() => setReturned(true)}
                    className="flex h-14 items-center justify-center gap-2 rounded-lg bg-accent-600 font-semibold text-white"
                  >
                    <ArrowSquareOut size={20} weight="bold" />
                    {l.label}
                  </a>
                ))}

                {/*
                  딥링크가 없거나 PC에서 열었을 때의 유일한 길.
                  링크가 있어도 항상 함께 둔다 — 스킴이 바뀌면 버튼은
                  아무것도 하지 않고, 그 사실을 사장님은 알 수 없다.
                */}
                <Button
                  variant="secondary"
                  size="lg"
                  full
                  onClick={() => void copy(info.copyText)}
                >
                  {copied ? (
                    <>
                      <Check size={18} weight="bold" /> 복사됨
                    </>
                  ) : (
                    <>
                      <Copy size={18} weight="regular" /> 계좌번호 복사
                    </>
                  )}
                </Button>

                <Button
                  variant="ghost"
                  full
                  loading={busy}
                  onClick={() => void markPaid("manual")}
                >
                  다른 방법으로 보냈어요 (완료 기록)
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
