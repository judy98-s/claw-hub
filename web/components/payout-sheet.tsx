"use client";

import { useEffect, useState } from "react";
import {
  ArrowSquareOut,
  Check,
  Copy,
  CreditCard,
  X,
} from "@phosphor-icons/react";

import { ApiError, getWith, postWith } from "@/lib/api";
import { krw } from "@/lib/format";
import { Button } from "@/components/ui/button";

type PayoutLink = { provider: string; label: string; url: string };

type PayoutMethod = "deeplink" | "manual" | "card_void";

type PayoutInfo = {
  /** "transfer" 면 계좌로 보내고, "card_void" 면 단말기에서 승인을 취소한다. */
  mode: "transfer" | "card_void";

  /** 카드 취소 건에서만 채워진다. */
  cardLast4: string;
  paidAtGuess: string | null;
  receiptCode: string;

  links: PayoutLink[];
  bankName: string;
  accountNo: string;
  holder: string;
  amountKrw: number;
  copyText: string;
  /** 사장님이 설정에 적어둔 출금 계좌. 비어 있을 수 있다. */
  fromAccount: string;
};

/**
 * 요청액과 그 아래의 "고를 만한" 금액들.
 *
 * 1,000원씩 빼면 35,000원짜리에서 34,000 / 33,000 같은 쓸모없는 값이 나온다.
 * 실제로 부분 환불할 때 고르는 건 3만·2만·1만 같은 동그란 숫자다.
 */
const AMOUNT_LADDER = [1000, 2000, 3000, 5000, 10000, 20000, 30000, 50000];

function amountChoices(requested: number): number[] {
  const below = AMOUNT_LADDER.filter((v) => v < requested)
    .sort((a, b) => b - a)
    .slice(0, 3);
  return [requested, ...below];
}

/** 손님이 적은 결제 시각. 없으면 접수 시각으로 찾으면 된다. */
function paidAtText(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString("ko-KR", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * 환불 처리 시트.
 *
 * 두 가지 모드가 있고, 손님이 무엇으로 결제했는지에 따라 정해진다.
 *
 * 현금(transfer): 국내 결제 환경상 서버가 직접 계좌이체를 할 수 없다.
 * 카카오페이·토스페이먼츠는 결제(수납) API이지 송금 API가 아니고, 실제
 * 송금에는 지급대행·펌뱅킹 계약이 필요하다. 그래서 딥링크로 앱을 열어
 * 사장님이 인증 한 번만 하게 하고, 돌아오면 결과를 묻는다. 딥링크가
 * 없거나 PC에서 열었으면 버튼이 아무것도 하지 않으므로, 계좌 복사와
 * 수동 "송금 완료"를 항상 함께 둔다.
 *
 * 카드(card_void): 보낼 계좌가 없다. 사장님이 단말기에서 승인을 취소하고,
 * 금액은 손님 카드로 돌아간다. 이 화면이 할 일은 그 거래를 찾을 단서를
 * 보여주고, 취소했다는 사실을 기록하는 것뿐이다.
 */
export function PayoutSheet({
  claimId,
  token,
  onClose,
  onPaid,
}: {
  claimId: string;
  /** Slack 링크로 들어온 경우의 접근 토큰. 로그인 상태면 없다. */
  token?: string;
  onClose: () => void;
  onPaid: () => void;
}) {
  const [info, setInfo] = useState<PayoutInfo | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  // 딥링크를 눌러 앱으로 나갔다 돌아온 상태. 이때 "보내셨나요?"를 묻는다.
  const [returned, setReturned] = useState(false);
  // 실제로 보낼(취소할) 금액. 요청액에서 줄일 수 있다 — 3천원 요청인데
  // 확인해보니 2천원만 먹힌 경우가 실제로 있다.
  const [amount, setAmount] = useState(0);
  // 되돌릴 수 없는 동작이므로 한 번 더 묻는다.
  const [confirming, setConfirming] = useState<null | PayoutMethod>(null);

  useEffect(() => {
    getWith<PayoutInfo>(`/api/admin/claims/${claimId}/payout-links`, token)
      .then((d) => {
        setInfo(d);
        setAmount(d.amountKrw);
      })
      .catch((err) =>
        setError(
          err instanceof ApiError
            ? err.message
            : "송금 정보를 불러오지 못했습니다.",
        ),
      );
  }, [claimId, token]);

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("복사에 실패했습니다. 길게 눌러 직접 복사해주세요.");
    }
  }

  async function markPaid(method: PayoutMethod) {
    setBusy(true);
    try {
      await postWith(
        `/api/admin/claims/${claimId}/mark-paid`,
        { method, amountKrw: amount },
        token,
      );
      onPaid();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "기록에 실패했습니다.");
      setBusy(false);
      setConfirming(null);
    }
  }

  const byCard = info?.mode === "card_void";
  const valid = info !== null && amount > 0 && amount <= info.amountKrw;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50">
      <div className="w-full max-w-lg rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold">
            {byCard ? "카드 결제 취소" : "환불 보내기"}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="닫기"
            className="p-1"
          >
            <X size={20} weight="bold" />
          </button>
        </div>

        {error && (
          <p
            role="alert"
            className="mb-3 rounded-lg bg-[var(--tone-stop-bg)] p-3 text-sm text-[var(--tone-stop-fg)]"
          >
            {error}
          </p>
        )}

        {!info ? (
          <div className="h-40 animate-pulse rounded-lg bg-[var(--surface-sunken)]" />
        ) : (
          <div className="grid gap-4">
            <div className="grid gap-1 rounded-lg bg-[var(--surface-sunken)] p-4">
              <p className="text-2xl font-bold tabular-nums">{krw(amount)}</p>
              {byCard ? (
                <p className="flex items-center gap-1.5 text-sm text-[var(--muted)]">
                  <CreditCard size={16} weight="regular" />
                  카드 끝 {info.cardLast4}
                  {paidAtText(info.paidAtGuess) &&
                    ` · ${paidAtText(info.paidAtGuess)} 결제`}
                </p>
              ) : (
                <p className="text-sm text-[var(--muted)]">
                  {info.bankName} {info.accountNo} · {info.holder}
                </p>
              )}
              {amount !== info.amountKrw && (
                <p className="text-sm text-[var(--tone-warn-fg)]">
                  손님 요청은 {krw(info.amountKrw)}입니다.
                </p>
              )}
              {!byCard && info.fromAccount && (
                <p className="mt-1 text-sm text-[var(--muted)]">
                  출금 계좌: {info.fromAccount}
                </p>
              )}
            </div>

            {/*
              카드 건에서 사장님이 실제로 하는 일은 이 화면 밖에 있다.
              단말기 앞에서 보고 따라갈 수 있게 순서대로 적는다.
            */}
            {byCard && (
              <ol className="grid gap-2 rounded-lg border border-[var(--line)] p-4 text-sm">
                <li className="flex gap-2">
                  <span className="font-bold text-accent-600">1</span>
                  <span>
                    카드 단말기에서 <b>거래내역 조회</b> 또는 <b>승인취소</b>를
                    누릅니다.
                  </span>
                </li>
                <li className="flex gap-2">
                  <span className="font-bold text-accent-600">2</span>
                  <span>
                    끝 <b>{info.cardLast4}</b>
                    {paidAtText(info.paidAtGuess) && (
                      <>
                        , <b>{paidAtText(info.paidAtGuess)}</b>
                      </>
                    )}{" "}
                    결제 건을 찾습니다.
                  </span>
                </li>
                <li className="flex gap-2">
                  <span className="font-bold text-accent-600">3</span>
                  <span>
                    <b>{krw(amount)}</b>을 취소합니다.
                    {amount !== info.amountKrw && " (부분취소)"}
                  </span>
                </li>
                <li className="flex gap-2">
                  <span className="font-bold text-accent-600">4</span>
                  <span>취소 영수증이 나오면 아래 버튼으로 기록합니다.</span>
                </li>
              </ol>
            )}

            {/* 보낼 금액. 요청액이 기본이고 줄일 수 있다. */}
            <div className="grid gap-2">
              <p className="text-sm font-semibold">
                {byCard ? "취소할 금액" : "보낼 금액"}
              </p>
              <div className="grid grid-cols-4 gap-2">
                {amountChoices(info.amountKrw).map((v) => (
                  <button
                    key={v}
                    type="button"
                    onClick={() => setAmount(v)}
                    className={[
                      "h-11 rounded-lg border text-sm font-semibold transition-colors duration-150",
                      amount === v
                        ? "border-accent-600 bg-[var(--tone-accent-bg)] text-[var(--tone-accent-fg)]"
                        : "border-[var(--line)] bg-[var(--surface)]",
                    ].join(" ")}
                  >
                    {v.toLocaleString("ko-KR")}
                  </button>
                ))}
              </div>
              <input
                type="text"
                inputMode="numeric"
                value={amount === 0 ? "" : String(amount)}
                onChange={(e) =>
                  setAmount(Number(e.target.value.replace(/[^0-9]/g, "")) || 0)
                }
                placeholder="직접 입력"
                aria-label={
                  byCard ? "취소할 금액 직접 입력" : "보낼 금액 직접 입력"
                }
                className="h-12 w-full rounded-lg border border-[var(--line)] bg-[var(--surface)] px-3.5 text-base text-[var(--fg)]"
              />
              {amount > info.amountKrw && (
                <p className="text-sm text-[var(--tone-stop-fg)]">
                  요청 금액보다 많이 {byCard ? "취소" : "보낼"} 수 없습니다.
                </p>
              )}
            </div>

            {byCard ? (
              <div className="grid gap-2">
                <Button
                  size="lg"
                  full
                  disabled={!valid}
                  onClick={() => setConfirming("card_void")}
                >
                  취소 완료로 기록
                </Button>
                <Button
                  variant="secondary"
                  full
                  onClick={() =>
                    void copy(
                      `접수 ${info.receiptCode} · 카드 끝 ${info.cardLast4} · ${krw(amount)}`,
                    )
                  }
                >
                  {copied ? (
                    <>
                      <Check size={18} weight="bold" /> 복사됨
                    </>
                  ) : (
                    <>
                      <Copy size={18} weight="regular" /> 조회 정보 복사
                    </>
                  )}
                </Button>
              </div>
            ) : returned ? (
              <div className="grid gap-2">
                <p className="text-center font-semibold">
                  {krw(amount)}을 보내셨나요?
                </p>
                <Button
                  size="lg"
                  full
                  onClick={() => setConfirming("deeplink")}
                >
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
                    href={l.url.replace(/amount=\d+/, `amount=${amount}`)}
                    onClick={(e) => {
                      if (!valid) {
                        e.preventDefault();
                        return;
                      }
                      setReturned(true);
                    }}
                    aria-disabled={!valid}
                    className={[
                      "flex h-14 items-center justify-center gap-2 rounded-lg font-semibold text-white",
                      valid
                        ? "bg-accent-600"
                        : "pointer-events-none bg-accent-600/40",
                    ].join(" ")}
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
                  disabled={!valid}
                  onClick={() => setConfirming("manual")}
                >
                  다른 방법으로 보냈어요 (완료 기록)
                </Button>
              </div>
            )}
          </div>
        )}

        {/*
          되돌릴 수 없는 기록이다. 한 번 더 묻는다.
          paid 상태는 되돌릴 수 없게 막아뒀으므로 잘못 누르면 고칠 길이 없다.
        */}
        {confirming && info && (
          <div className="fixed inset-0 z-10 flex items-end justify-center bg-ink-950/50">
            <div className="grid w-full max-w-lg gap-4 rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
              <div className="grid gap-1">
                <h3 className="text-lg font-bold">
                  {byCard ? "정말 취소하셨나요?" : "정말 보내셨나요?"}
                </h3>
                <p className="text-sm text-[var(--muted)]">
                  {byCard ? (
                    <>
                      카드 끝 {info.cardLast4} 결제 중{" "}
                      <b className="text-[var(--fg)]">{krw(amount)}</b>을 취소한
                      것으로 기록합니다.
                    </>
                  ) : (
                    <>
                      {info.bankName} {info.accountNo} ({info.holder}) 로{" "}
                      <b className="text-[var(--fg)]">{krw(amount)}</b> 보낸
                      것으로 기록합니다.
                    </>
                  )}{" "}
                  기록은 되돌릴 수 없습니다.
                </p>
              </div>
              <div className="grid gap-2">
                <Button
                  size="lg"
                  full
                  loading={busy}
                  onClick={() => void markPaid(confirming)}
                >
                  {byCard ? "네, 취소했습니다" : "네, 보냈습니다"}
                </Button>
                <Button
                  variant="ghost"
                  full
                  onClick={() => setConfirming(null)}
                >
                  취소
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
