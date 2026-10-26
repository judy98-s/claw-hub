"use client";

import { useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Camera,
  CreditCard,
  CurrencyKrw,
  Info,
  Money,
  Phone,
  Trash,
  Warning,
} from "@phosphor-icons/react";

import { ApiError, postForm } from "@/lib/api";
import { shrinkImage } from "@/lib/image";
import { krw } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/field";
import { PreviewImage } from "@/components/ui/preview-image";
import { storeConfig } from "@/lib/stores";

type Machine = {
  code: string;
  label: string;
  storeName: string;
  storePhone: string;
  /** 업종. 캐릭터·빠른 금액·기계를 부르는 말이 이 값으로 갈린다. */
  storeType: string;
  /** 고를 수 있는 증상. 서버가 내려주는 목록을 그대로 쓴다. */
  issues: { value: string; label: string }[];
  reviewThresholdKrw: number;
  maxAmountKrw: number;
  maxPhotos: number;
};

type Bank = { code: string; name: string };

/**
 * 증상 선택지는 서버가 내려준다 (machine.issues).
 *
 * 전에는 여기에 인형뽑기 네 개를 박아 두고 있었다. 그래서 무인세탁
 * 손님에게도 "인형 걸림"이 보였고, 고를 수 있는 게 "기타" 하나였다.
 * 거절하는 쪽(서버)이 목록을 정해야 한다.
 *
 * 설명 문구(hint)만 여기서 붙인다. 서버가 알 필요 없는 copy 다.
 * 서버에 있고 여기 없는 코드는 hint 없이 라벨만 보인다 — 업종을 추가할 때
 * 프런트 배포를 기다리지 않아도 화면이 동작해야 한다.
 *
 * 드롭다운이 아니라 큰 버튼이다. 길에서 한 손으로 쓰는 화면에서 select 를
 * 열고 고르는 건 탭 두 번에 스크롤이고, 그만큼 이탈한다.
 */

function hintFor(storeType: string, value: string): string {
  return storeConfig(storeType).issues.find((i) => i.value === value)?.hint ?? "";
}

/**
 * 결제 수단. 이 선택 하나로 아래 절반이 바뀐다.
 *
 * 현금이면 계좌로 송금하고, 카드면 사장님이 단말기에서 승인을 취소한다.
 * 카드 건에 계좌를 물어보면 받을 이유도 없는 정보를 받으면서 손님에게는
 * 없는 장벽을 세우는 셈이다.
 */
const PAYMENTS = [
  { value: "cash", label: "현금", hint: "지폐·동전을 넣었어요", Icon: Money },
  { value: "card", label: "카드", hint: "카드를 찍었어요", Icon: CreditCard },
] as const;

export function ClaimForm({
  machine,
  banks,
}: {
  machine: Machine;
  banks: Bank[];
}) {
  const router = useRouter();
  // 업종별 빠른 금액. 뽑기방 1,000원과 무인사진관 5,000원은 다른 숫자다.
  const quickAmounts = storeConfig(machine.storeType).quickAmounts;

  const [issueType, setIssueType] = useState<string>("");
  const [amount, setAmount] = useState<string>("");
  const [description, setDescription] = useState("");
  const [phone, setPhone] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<string>("");
  const [cardLast4, setCardLast4] = useState("");
  const [paidAtGuess, setPaidAtGuess] = useState("");
  const [bankCode, setBankCode] = useState("");
  const [accountNo, setAccountNo] = useState("");
  const [holder, setHolder] = useState("");
  const [photos, setPhotos] = useState<File[]>([]);
  const [allBanks, setAllBanks] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);

  /**
   * 멱등키는 폼이 뜰 때 한 번 만들고 재시도 사이에 유지한다.
   * 제출마다 새로 만들면 재시도가 새 접수가 되어 환불이 두 번 나간다.
   */
  const idempotencyKey = useMemo(
    () =>
      typeof crypto !== "undefined" ? crypto.randomUUID() : String(Date.now()),
    [],
  );

  /*
    은행 21개를 한 번에 펼치면 그것만 7줄이고, 폼 전체가 네 화면이 된다.
    인터넷은행 셋과 주요 시중은행 셋이 대부분을 덮으므로 먼저 그만 보이고
    나머지는 접어둔다. 이미 고른 은행이 접힌 쪽에 있으면 펼친 채로 둔다.
  */
  const COMMON_BANKS = 6;
  const shownBanks = allBanks ? banks : banks.slice(0, COMMON_BANKS);

  const amountNum = Number(amount || 0);
  const needsPhoto = amountNum >= machine.reviewThresholdKrw;
  const photoMissing = needsPhoto && photos.length === 0;

  async function addPhotos(files: FileList | null) {
    if (!files?.length) return;
    const room = machine.maxPhotos - photos.length;
    if (room <= 0) return;

    const shrunk = await Promise.all(
      Array.from(files).slice(0, room).map(shrinkImage),
    );
    setPhotos((prev) => [...prev, ...shrunk]);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const form = new FormData();
    form.set("machineCode", machine.code);
    form.set("issueType", issueType);
    form.set("amountKrw", amount);
    form.set("description", description);
    form.set("phone", phone);
    form.set("paymentMethod", paymentMethod);
    if (paymentMethod === "card") {
      form.set("cardLast4", cardLast4);
      form.set("paidAtGuess", paidAtGuess);
    } else {
      form.set("bankCode", bankCode);
      form.set("accountNo", accountNo);
      form.set("holder", holder);
    }
    photos.forEach((p) => form.append("photos", p));

    setSubmitting(true);
    try {
      const res = await postForm<{ receiptCode: string }>(
        "/api/public/claims",
        form,
        idempotencyKey,
      );
      router.push(
        `/r/${machine.code}/done?receipt=${res.receiptCode}&pay=${paymentMethod}`,
      );
    } catch (err) {
      // 입력값은 그대로 둔다. 다 지우고 다시 쓰게 하면 그 자리에서 포기한다.
      setError(
        err instanceof ApiError
          ? err.message
          : "문제가 생겼습니다. 다시 시도해주세요.",
      );
      setSubmitting(false);
    }
  }

  const byCard = paymentMethod === "card";
  const refundReady = byCard
    ? cardLast4.length === 4
    : Boolean(bankCode && accountNo && holder);
  const ready =
    issueType &&
    amountNum > 0 &&
    phone &&
    paymentMethod &&
    refundReady &&
    !photoMissing;

  return (
    <main className="mx-auto min-h-[100dvh] max-w-md px-4 pb-32 pt-6">
      <header className="mb-6 grid gap-1">
        <p className="text-sm text-[var(--muted)]">{machine.storeName}</p>
        <h1 className="text-xl font-bold">{machine.label} 문제 신고</h1>
      </header>

      <form onSubmit={submit} className="grid gap-7">
        {/* 증상 — 큰 터치 타깃 4개 */}
        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-semibold">
            어떤 문제인가요? <span className="text-stop-600">*</span>
          </legend>
          <div className="grid grid-cols-2 gap-2">
            {machine.issues.map((issue) => {
              const on = issueType === issue.value;
              const hint = hintFor(machine.storeType, issue.value);
              return (
                <button
                  key={issue.value}
                  type="button"
                  onClick={() => setIssueType(issue.value)}
                  aria-pressed={on}
                  className={[
                    "grid min-h-20 content-center gap-0.5 rounded-lg border p-3 text-left",
                    "transition-colors duration-150",
                    on
                      ? "border-ink bg-pink-soft text-ink"
                      : "border-[var(--line)] bg-[var(--surface)]",
                  ].join(" ")}
                >
                  <span className="font-semibold">{issue.label}</span>
                  {hint && (
                    <span
                      className={`text-xs ${on ? "text-ink" : "text-[var(--muted)]"}`}
                    >
                      {hint}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </fieldset>

        {/* 금액 */}
        <Field label="잃어버린 금액" required>
          <div className="grid gap-2">
            <div className="grid grid-cols-4 gap-2">
              {quickAmounts.map((a) => (
                <button
                  key={a}
                  type="button"
                  onClick={() => setAmount(String(a))}
                  className={[
                    "h-11 rounded-lg border text-sm font-semibold transition-colors duration-150",
                    amountNum === a
                      ? "border-ink bg-pink-soft text-ink"
                      : "border-[var(--line)] bg-[var(--surface)]",
                  ].join(" ")}
                >
                  {a.toLocaleString("ko-KR")}
                </button>
              ))}
            </div>
            <div className="relative">
              <CurrencyKrw
                size={18}
                weight="regular"
                className="absolute left-3.5 top-1/2 -translate-y-1/2 text-[var(--muted)]"
              />
              <Input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                value={amount}
                onChange={(e) =>
                  setAmount(e.target.value.replace(/[^0-9]/g, ""))
                }
                placeholder="직접 입력"
                className="pl-10"
                required
              />
            </div>
          </div>
        </Field>

        {/*
          금액이 임계값을 넘는 순간 바로 알려준다.
          제출하고 나서 거부당하는 것보다 입력 중에 아는 게 낫다.
        */}
        {needsPhoto && (
          <div className="flex gap-2.5 rounded-lg bg-[var(--tone-warn-bg)] p-3.5 text-[var(--tone-warn-fg)]">
            <Info size={20} weight="fill" className="mt-0.5 shrink-0" />
            <p className="text-sm">
              {krw(machine.reviewThresholdKrw)} 이상은 <b>사진이 필요합니다.</b>{" "}
              확인을 위해 사장님이 전화드릴 수 있습니다.
            </p>
          </div>
        )}

        {/* 사진 */}
        <Field
          label="사진"
          required={needsPhoto}
          hint={`증상이 보이는 사진을 올려주세요. 최대 ${machine.maxPhotos}장.`}
        >
          <div className="grid gap-2">
            {photos.length > 0 && (
              <ul className="grid grid-cols-3 gap-2">
                {photos.map((p, i) => (
                  <li key={`${p.name}-${i}`} className="relative">
                    <PreviewImage
                      file={p}
                      alt={`첨부한 사진 ${i + 1}`}
                      className="aspect-square w-full rounded-lg border border-[var(--line)] object-cover"
                    />
                    <button
                      type="button"
                      onClick={() =>
                        setPhotos((prev) => prev.filter((_, j) => j !== i))
                      }
                      aria-label={`사진 ${i + 1} 삭제`}
                      className="absolute right-1 top-1 grid size-7 place-items-center rounded-md bg-ink-900/75 text-white"
                    >
                      <Trash size={14} weight="bold" />
                    </button>
                  </li>
                ))}
              </ul>
            )}

            {photos.length < machine.maxPhotos && (
              <>
                <input
                  ref={fileInput}
                  type="file"
                  accept="image/*"
                  capture="environment"
                  multiple
                  className="hidden"
                  onChange={(e) => {
                    void addPhotos(e.target.files);
                    e.target.value = "";
                  }}
                />
                <button
                  type="button"
                  onClick={() => fileInput.current?.click()}
                  className={[
                    "flex h-14 items-center justify-center gap-2 rounded-lg border border-dashed",
                    "font-semibold transition-colors duration-150",
                    photoMissing
                      ? "border-stop-600 text-stop-600"
                      : "border-[var(--color-ink-300)] text-[var(--muted)]",
                  ].join(" ")}
                >
                  <Camera size={20} weight="regular" />
                  사진 촬영 / 선택
                </button>
              </>
            )}
          </div>
        </Field>

        <Field label="자세한 설명" hint="선택 사항입니다.">
          <Textarea
            rows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="예) 2번 투입구에 천원 두 장 넣었는데 반응이 없어요"
            maxLength={1000}
          />
        </Field>

        <hr className="border-[var(--line)]" />

        {/* 환불 정보 */}
        <Field
          label="연락처"
          required
          hint="환불 확인을 위해 연락드릴 수 있습니다."
        >
          <div className="relative">
            <Phone
              size={18}
              className="absolute left-3.5 top-1/2 -translate-y-1/2 text-[var(--muted)]"
            />
            <Input
              type="tel"
              inputMode="tel"
              autoComplete="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="010-1234-5678"
              className="pl-10"
              required
            />
          </div>
        </Field>

        {/*
          결제 수단을 고르는 순간 아래가 통째로 바뀐다. 그래서 연락처처럼
          양쪽에 다 필요한 것을 먼저 받고, 갈리는 것은 이 다음에 둔다.
          안내 문구가 선택 바로 아래 붙어야 "계좌는 왜 안 물어보지"가 없다. 이 선택에 따라 아래에서
          물어볼 것이 통째로 바뀐다.
        */}
        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-semibold">
            어떻게 결제하셨나요? <span className="text-stop-600">*</span>
          </legend>
          <div className="grid grid-cols-2 gap-2">
            {PAYMENTS.map(({ value, label, hint, Icon }) => {
              const on = paymentMethod === value;
              return (
                <button
                  key={value}
                  type="button"
                  onClick={() => setPaymentMethod(value)}
                  aria-pressed={on}
                  className={[
                    "grid min-h-20 content-center gap-1 rounded-lg border p-3 text-left",
                    "transition-colors duration-150",
                    on
                      ? "border-ink bg-pink-soft text-ink"
                      : "border-[var(--line)] bg-[var(--surface)]",
                  ].join(" ")}
                >
                  <Icon size={22} weight={on ? "fill" : "regular"} />
                  <span className="font-semibold">{label}</span>
                  <span
                    className={`text-xs ${on ? "text-ink" : "text-[var(--muted)]"}`}
                  >
                    {hint}
                  </span>
                </button>
              );
            })}
          </div>
        </fieldset>

        {/*
          카드 결제는 계좌로 보내지 않는다. 사장님이 단말기에서 승인을
          취소하고, 금액은 결제했던 카드로 돌아간다. 그래서 여기서 필요한
          건 계좌가 아니라 "그 거래가 어느 것인지"를 찾을 단서다.
        */}
        {byCard && (
          <>
            <div className="flex gap-2.5 rounded-lg bg-pink-soft p-3.5 text-ink">
              <Info size={20} weight="fill" className="mt-0.5 shrink-0" />
              <p className="text-sm">
                카드 결제는 <b>결제를 취소해드립니다.</b> 계좌번호는 필요 없고,
                금액은 결제하신 카드로 돌아갑니다. 카드사에 따라 2~5일 걸릴 수
                있습니다.
              </p>
            </div>

            <Field
              label="카드 뒷자리 4자리"
              required
              hint="영수증이나 결제 문자에 있습니다. 전체 번호는 받지 않습니다."
            >
              <Input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                value={cardLast4}
                onChange={(e) =>
                  setCardLast4(
                    e.target.value.replace(/[^0-9]/g, "").slice(0, 4),
                  )
                }
                placeholder="1234"
                maxLength={4}
                required
              />
            </Field>

            <Field
              label="결제하신 시각"
              hint="선택 사항입니다. 기억나는 대로면 충분합니다."
            >
              <Input
                type="datetime-local"
                value={paidAtGuess}
                onChange={(e) => setPaidAtGuess(e.target.value)}
              />
            </Field>
          </>
        )}

        {!byCard && paymentMethod && (
          <>
            <Field label="환불받을 은행" required>
              <div className="grid gap-2">
                <div className="grid grid-cols-3 gap-2">
                  {shownBanks.map((b) => {
                    const on = bankCode === b.code;
                    return (
                      <button
                        key={b.code}
                        type="button"
                        onClick={() => setBankCode(b.code)}
                        aria-pressed={on}
                        className={[
                          "h-12 rounded-lg border px-1 text-sm font-medium transition-colors duration-150",
                          on
                            ? "border-ink bg-pink-soft text-ink"
                            : "border-[var(--line)] bg-[var(--surface)]",
                        ].join(" ")}
                      >
                        {b.name}
                      </button>
                    );
                  })}
                </div>
                {!allBanks && (
                  <button
                    type="button"
                    onClick={() => setAllBanks(true)}
                    className="h-10 text-sm font-semibold text-ink underline underline-offset-4"
                  >
                    다른 은행 찾기
                  </button>
                )}
              </div>
            </Field>

            <Field label="계좌번호" required>
              <Input
                type="text"
                inputMode="numeric"
                value={accountNo}
                onChange={(e) =>
                  setAccountNo(e.target.value.replace(/[^0-9-]/g, ""))
                }
                placeholder="- 없이 입력"
                required
              />
            </Field>

            <Field
              label="예금주"
              required
              hint="계좌에 등록된 이름과 같아야 합니다."
            >
              <Input
                type="text"
                autoComplete="name"
                value={holder}
                onChange={(e) => setHolder(e.target.value)}
                placeholder="홍길동"
                maxLength={40}
                required
              />
            </Field>
          </>
        )}

        {error && (
          <div
            role="alert"
            className="flex gap-2.5 rounded-lg bg-[var(--tone-stop-bg)] p-3.5 text-[var(--tone-stop-fg)]"
          >
            <Warning size={20} weight="fill" className="mt-0.5 shrink-0" />
            <p className="text-sm">{error}</p>
          </div>
        )}
      </form>

      {/*
        제출 버튼을 화면 하단에 고정한다. 폼이 한 화면을 넘어가므로,
        다 채우고 나서 버튼을 찾아 스크롤하게 두면 안 된다.
      */}
      <div className="fixed inset-x-0 bottom-0 border-t border-[var(--line)] bg-[var(--bg)] px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <div className="mx-auto max-w-md">
          <Button
            type="submit"
            size="lg"
            full
            loading={submitting}
            disabled={!ready}
            onClick={submit}
          >
            {submitting ? "접수 중" : "신고 접수하기"}
          </Button>
        </div>
      </div>
    </main>
  );
}
