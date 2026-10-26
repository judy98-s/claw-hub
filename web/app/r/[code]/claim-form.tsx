"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  CreditCard,
  Lock,
  Money,
  PencilSimple,
  Plus,
  Trash,
  Warning,
} from "@phosphor-icons/react";

import { ApiError, postForm } from "@/lib/api";
import { shrinkImage } from "@/lib/image";
import { krw } from "@/lib/format";
import { Input } from "@/components/ui/field";
import { PreviewImage } from "@/components/ui/preview-image";
import { storeConfig } from "@/lib/stores";
import {
  BottomBar,
  ChoiceButton,
  MachinePlate,
  PrimaryButton,
  StepHeader,
  StepTitle,
  moodForStep,
} from "./wizard-ui";

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
 * 단계를 **이름**으로 둔다.
 *
 * 배열 인덱스로 두면 "다음은 +1" 이 되는데, 결제 수단에 따라 환불 단계가
 * 갈려서(계좌 / 카드 뒷자리) 그 규칙이 바로 깨진다. 뒤로 가기에서도
 * 어긋난다. 이름으로 두고 순서는 한 곳에서 계산한다.
 */
type StepKey =
  | "issue"
  | "amount"
  | "photos"
  | "payment"
  | "refund"
  | "phone"
  | "confirm";

/** 입력 단계. 확인은 요약이라 숫자를 세지 않는다. */
const INPUT_STEPS: StepKey[] = [
  "issue",
  "amount",
  "photos",
  "payment",
  "refund",
  "phone",
];
const STEPS: StepKey[] = [...INPUT_STEPS, "confirm"];

/**
 * 결제 수단. 이 선택 하나로 뒤 절반이 바뀐다.
 *
 * 현금이면 계좌로 송금하고, 카드면 사장님이 단말기에서 승인을 취소한다.
 * 카드 건에 계좌를 물어보면 받을 이유도 없는 정보를 받으면서 손님에게는
 * 없는 장벽을 세우는 셈이다.
 */
const PAYMENTS = [
  { value: "cash", label: "현금", hint: "지폐·동전을 넣었어요", Icon: Money },
  { value: "card", label: "카드", hint: "카드를 찍었어요", Icon: CreditCard },
] as const;

export function ClaimForm({ machine, banks }: { machine: Machine; banks: Bank[] }) {
  const router = useRouter();
  const cfg = storeConfig(machine.storeType);

  const [step, setStep] = useState<StepKey>("issue");

  const [issueType, setIssueType] = useState("");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  const [phone, setPhone] = useState("");
  const [paymentMethod, setPaymentMethod] = useState("");
  const [cardLast4, setCardLast4] = useState("");
  const [bankCode, setBankCode] = useState("");
  const [accountNo, setAccountNo] = useState("");
  const [holder, setHolder] = useState("");
  const [photos, setPhotos] = useState<File[]>([]);
  const [allBanks, setAllBanks] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  /**
   * 멱등키는 폼이 뜰 때 한 번 만들고 재시도 사이에 유지한다.
   * 제출마다 새로 만들면 재시도가 새 접수가 되어 환불이 두 번 나간다.
   * 단계 이동으로도 재생성되지 않아야 한다 — useMemo 의 deps 가 빈 배열인 이유다.
   */
  const idempotencyKey = useMemo(
    () => (typeof crypto !== "undefined" ? crypto.randomUUID() : String(Date.now())),
    [],
  );

  const amountNum = Number(amount || 0);
  const needsPhoto = amountNum >= machine.reviewThresholdKrw;
  const byCard = paymentMethod === "card";

  /* 이 손님이 지날 단계 순서. 결제 수단이 정해지기 전에도 길이는 같다. */
  const order = STEPS;
  const index = order.indexOf(step);
  const isConfirm = step === "confirm";
  const inputNumber = Math.min(index + 1, INPUT_STEPS.length);

  /**
   * 안드로이드 뒤로 가기 버튼을 단계 이동으로 만든다.
   *
   * 안 하면 뒤로 가기 한 번에 폼을 벗어나고 입력이 전부 사라진다. 지하에서
   * 10분 걸려 적은 계좌번호가 날아가는 일이다.
   *
   * 단계는 **URL 쿼리**에 싣고, `history.state` 에는 아무것도 넣지 않는다.
   * App Router 가 history.state 를 자기 라우팅에 쓰기 때문에, 거기에 우리
   * 키를 넣으면 popstate 때 라우터가 트리를 되감아 단계가 두 칸씩 튄다
   * (뒤로 한 번에 3단계 → 1단계로 갔다). 그래서 state 는 null 로 넘기고
   * 읽기는 location.search 에서 한다.
   *
   * 단계 전환은 React state 로 즉시 끝낸다. router.push 를 쓰면 쿼리만
   * 바뀌어도 서버 컴포넌트가 다시 돌아 단계마다 왕복이 생긴다 — 지하
   * LTE 에서 정확히 피해야 하는 것이다.
   */
  useEffect(() => {
    // 새로고침이나 링크로 ?step= 이 붙어 들어온 경우. 단계만 복원하고
    // 입력값은 복원되지 않으므로, 빈 폼으로 중간 단계에 떨어뜨리지 않는다.
    if (new URLSearchParams(location.search).has("step")) {
      history.replaceState(null, "", location.pathname);
    }
    const onPop = () => {
      const k = new URLSearchParams(location.search).get("step") as StepKey | null;
      // 쿼리가 없으면 첫 단계다. 우리가 모르는 값이면 폼을 벗어나는 것이니
      // 건드리지 않고 브라우저에 맡긴다.
      if (!k) setStep("issue");
      else if (STEPS.includes(k)) setStep(k);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const go = useCallback((next: StepKey) => {
    setStep(next);
    setError("");
    history.pushState(null, "", `?step=${next}`);
    // 단계가 바뀌면 본문 맨 위부터 읽어야 한다. 긴 단계에서 넘어오면
    // 스크롤이 중간에 남아 질문이 화면 밖에 있다.
    bodyRef.current?.scrollIntoView({ block: "start" });
  }, []);

  const next = useCallback(() => {
    const i = order.indexOf(step);
    if (i >= 0 && i < order.length - 1) go(order[i + 1]);
  }, [order, step, go]);

  const back = index === 0 ? null : () => history.back();

  /** 단계별로 다음으로 갈 수 있는 조건. */
  const canAdvance = (() => {
    switch (step) {
      case "issue":
        return Boolean(issueType);
      case "amount":
        return amountNum > 0 && amountNum <= machine.maxAmountKrw;
      case "photos":
        // 기준액 이상이면 1장 필수. 서버도 원자적으로 막지만(RequiresPhoto),
        // 프런트에서만 막으면 우회되고 서버에서만 막으면 손님이 마지막에
        // 거절당한다. 둘 다 필요하다.
        return !needsPhoto || photos.length > 0;
      case "payment":
        return Boolean(paymentMethod);
      case "refund":
        return byCard
          ? cardLast4.length === 4
          : Boolean(bankCode) && accountNo.replace(/\D/g, "").length >= 8 && holder.trim().length > 0;
      case "phone":
        return phone.replace(/\D/g, "").length >= 10;
      case "confirm":
        return !submitting;
    }
  })();

  async function addPhotos(files: FileList | null) {
    if (!files?.length) return;
    const room = machine.maxPhotos - photos.length;
    if (room <= 0) return;
    const shrunk = await Promise.all(Array.from(files).slice(0, room).map(shrinkImage));
    setPhotos((prev) => [...prev, ...shrunk]);
  }

  async function submit() {
    setError("");
    const form = new FormData();
    form.set("machineCode", machine.code);
    form.set("issueType", issueType);
    form.set("amountKrw", amount);
    form.set("description", description);
    form.set("phone", phone);
    form.set("paymentMethod", paymentMethod);
    if (byCard) {
      form.set("cardLast4", cardLast4);
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
      router.push(`/r/${machine.code}/done?receipt=${res.receiptCode}&pay=${paymentMethod}`);
    } catch (err) {
      // 입력값은 그대로 둔다. 다 지우고 다시 쓰게 하면 그 자리에서 포기한다.
      setError(
        err instanceof ApiError ? err.message : "문제가 생겼습니다. 다시 시도해주세요.",
      );
      setSubmitting(false);
    }
  }

  /* 은행 21개를 한 번에 펼치면 그것만 7줄이다. 인터넷은행 셋과 주요
     시중은행 셋이 대부분을 덮으므로 먼저 그만 보인다. */
  const shownBanks = allBanks ? banks : banks.slice(0, 6);

  const primaryLabel = (() => {
    if (isConfirm) return submitting ? "접수 중…" : "신고 접수하기";
    if (step === "photos" && !needsPhoto && photos.length === 0) return "사진 없이 넘어가기";
    return "다음";
  })();

  return (
    <main className="mx-auto min-h-[100dvh] max-w-md pb-32">
      <StepHeader
        current={inputNumber}
        total={INPUT_STEPS.length}
        isConfirm={isConfirm}
        onBack={back}
      >
        <MachinePlate
          storeType={machine.storeType}
          storeName={machine.storeName}
          machineLabel={machine.label}
          mood={moodForStep(inputNumber, isConfirm)}
        />
      </StepHeader>

      <div ref={bodyRef} className="grid gap-6 p-5">
        {step === "issue" && (
          <>
            <StepTitle title="어떤 문제인가요?" />
            <div className="grid grid-cols-2 gap-2">
              {machine.issues.map((issue) => {
                const hint = cfg.issues.find((i) => i.value === issue.value)?.hint;
                return (
                  <ChoiceButton
                    key={issue.value}
                    selected={issueType === issue.value}
                    minHeight={88}
                    onClick={() => {
                      setIssueType(issue.value);
                      // 선택 즉시 다음으로. 이 단계는 되돌아와도 자동으로
                      // 넘어가지 않는다 — 효과가 아니라 클릭 핸들러에
                      // 있으므로, 뒤로 와서 머무를 수 있다.
                      next();
                    }}
                  >
                    <span className="text-lg font-bold">{issue.label}</span>
                    {hint && <span className="text-sm text-ink-soft">{hint}</span>}
                  </ChoiceButton>
                );
              })}
            </div>
          </>
        )}

        {step === "amount" && (
          <>
            <StepTitle title="얼마를 잃으셨나요?" hint="기계에 넣은 금액이에요." />
            <div className="grid grid-cols-2 gap-2">
              {cfg.quickAmounts.map((a) => (
                <ChoiceButton
                  key={a}
                  selected={amountNum === a}
                  minHeight={60}
                  onClick={() => setAmount(String(a))}
                >
                  <span className="text-center text-[19px] font-bold">{krw(a)}</span>
                </ChoiceButton>
              ))}
            </div>
            <label className="grid gap-1.5">
              <span className="text-sm font-semibold">다른 금액</span>
              <div className="relative">
                <Input
                  type="text"
                  inputMode="numeric"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value.replace(/\D/g, ""))}
                  placeholder="0"
                  className="h-14 pr-10 text-right"
                  aria-label="금액 직접 입력"
                />
                <span className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-ink-soft">
                  원
                </span>
              </div>
              {amountNum > machine.maxAmountKrw && (
                <span className="text-sm text-stop-600">
                  {krw(machine.maxAmountKrw)}까지 접수할 수 있어요. 더 큰 금액은 매장에 직접
                  문의해주세요.
                </span>
              )}
            </label>
          </>
        )}

        {step === "photos" && (
          <>
            <StepTitle
              title="증상 사진을 올려주세요"
              hint={
                needsPhoto
                  ? `${krw(machine.reviewThresholdKrw)} 이상은 사진이 필요해요. 1장이면 충분합니다.`
                  : "없어도 접수할 수 있어요. 있으면 처리가 빨라집니다."
              }
            />
            <div className="grid grid-cols-3 gap-2">
              {photos.map((p, i) => (
                <div key={i} className="relative">
                  <PreviewImage
                    file={p}
                    alt={`첨부한 사진 ${i + 1}`}
                    className="aspect-square w-full rounded-md border-2 border-ink object-cover"
                  />
                  <button
                    type="button"
                    onClick={() => setPhotos((prev) => prev.filter((_, j) => j !== i))}
                    aria-label={`${i + 1}번째 사진 삭제`}
                    className="absolute -right-1.5 -top-1.5 grid size-8 place-items-center rounded-full border-2 border-ink bg-paper"
                  >
                    <Trash size={15} weight="bold" />
                  </button>
                </div>
              ))}
            </div>
            {photos.length < machine.maxPhotos && (
              <>
                <button
                  type="button"
                  onClick={() => fileInput.current?.click()}
                  className="grid h-16 place-items-center rounded-md border-2 border-dashed border-ink text-ink"
                >
                  <span className="flex items-center gap-2 font-semibold">
                    <Plus size={20} weight="bold" />
                    사진 추가 ({photos.length}/{machine.maxPhotos})
                  </span>
                </button>
                <input
                  ref={fileInput}
                  type="file"
                  accept="image/*"
                  capture="environment"
                  multiple
                  hidden
                  onChange={(e) => {
                    void addPhotos(e.target.files);
                    e.target.value = "";
                  }}
                />
              </>
            )}
          </>
        )}

        {step === "payment" && (
          <>
            <StepTitle title="어떻게 결제하셨나요?" />
            <div className="grid gap-2">
              {PAYMENTS.map(({ value, label, hint, Icon }) => (
                <ChoiceButton
                  key={value}
                  selected={paymentMethod === value}
                  minHeight={84}
                  onClick={() => {
                    setPaymentMethod(value);
                    next();
                  }}
                >
                  <span className="flex items-center gap-3">
                    <Icon size={28} weight="bold" />
                    <span className="grid">
                      <span className="text-lg font-bold">{label}</span>
                      <span className="text-sm text-ink-soft">{hint}</span>
                    </span>
                  </span>
                </ChoiceButton>
              ))}
            </div>
          </>
        )}

        {step === "refund" && !byCard && (
          <>
            <StepTitle title="환불받을 계좌" hint="사장님이 이 계좌로 바로 보내드려요." />
            <div className="grid gap-2">
              <span className="text-sm font-semibold">은행</span>
              <div className="grid grid-cols-3 gap-2">
                {shownBanks.map((b) => (
                  <ChoiceButton
                    key={b.code}
                    selected={bankCode === b.code}
                    minHeight={52}
                    onClick={() => setBankCode(b.code)}
                  >
                    <span className="text-center text-sm font-semibold">{b.name}</span>
                  </ChoiceButton>
                ))}
              </div>
              {!allBanks && banks.length > 6 && (
                <button
                  type="button"
                  onClick={() => setAllBanks(true)}
                  className="justify-self-start py-2 text-sm font-semibold underline"
                >
                  다른 은행 찾기
                </button>
              )}
            </div>
            <label className="grid gap-1.5">
              <span className="text-sm font-semibold">계좌번호</span>
              <Input
                type="text"
                inputMode="numeric"
                value={accountNo}
                maxLength={16}
                onChange={(e) => setAccountNo(e.target.value.replace(/\D/g, ""))}
                placeholder="- 없이 숫자만"
                className="h-14"
              />
            </label>
            <label className="grid gap-1.5">
              <span className="text-sm font-semibold">예금주</span>
              <Input
                type="text"
                value={holder}
                maxLength={20}
                onChange={(e) => setHolder(e.target.value)}
                placeholder="통장에 적힌 이름"
                className="h-14"
              />
            </label>
          </>
        )}

        {step === "refund" && byCard && (
          <>
            <StepTitle title="카드 뒷자리 4자리" />
            <p className="rounded-md bg-pink-soft p-3 text-sm">
              사장님이 단말기에서 그 결제를 찾아 취소합니다. 카드사에 따라 2~5일 걸릴 수
              있어요.
            </p>
            <Input
              type="text"
              inputMode="numeric"
              value={cardLast4}
              maxLength={4}
              onChange={(e) => setCardLast4(e.target.value.replace(/\D/g, ""))}
              placeholder="0000"
              aria-label="카드 뒷자리 4자리"
              className="h-16 text-center text-[28px] font-bold tracking-[0.3em]"
            />
          </>
        )}

        {step === "phone" && (
          <>
            <StepTitle
              title="연락받을 번호"
              hint="처리 결과를 알려드릴 때만 씁니다."
            />
            <Input
              type="tel"
              inputMode="numeric"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="010-0000-0000"
              aria-label="휴대폰 번호"
              className="h-14"
            />
            <label className="grid gap-1.5">
              <span className="text-sm font-semibold">
                더 알려주실 것 <span className="text-ink-soft">(선택)</span>
              </span>
              <Input
                type="text"
                value={description}
                maxLength={200}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="예: 천원 두 번 넣었어요"
                className="h-14"
              />
            </label>
          </>
        )}

        {step === "confirm" && (
          <>
            <StepTitle title="이대로 접수할까요?" />
            <div className="divide-y-2 divide-[#F2F0EC] overflow-hidden rounded-md border-2 border-ink bg-surface">
              <SummaryRow
                label="증상"
                value={machine.issues.find((i) => i.value === issueType)?.label ?? ""}
                onEdit={() => go("issue")}
              />
              <SummaryRow label="금액" value={krw(amountNum)} onEdit={() => go("amount")} />
              <SummaryRow
                label="사진"
                value={photos.length ? `${photos.length}장` : "없음"}
                onEdit={() => go("photos")}
              />
              <SummaryRow
                label="환불"
                value={
                  byCard
                    ? `카드 ****${cardLast4}`
                    : `${banks.find((b) => b.code === bankCode)?.name ?? ""} ${maskAccount(accountNo)}`
                }
                onEdit={() => go("refund")}
              />
              <SummaryRow label="연락처" value={phone} onEdit={() => go("phone")} />
            </div>

            <p className="flex items-start gap-2 text-sm text-ink-soft">
              <Lock size={18} weight="bold" className="mt-0.5 shrink-0" />
              계좌번호와 연락처는 암호화해 저장하고, 환불 처리에만 씁니다.
            </p>

            {error && (
              <p
                role="alert"
                className="flex items-start gap-2 rounded-md border-2 border-stop-600 bg-surface p-3 text-sm text-stop-600"
              >
                <Warning size={18} weight="bold" className="mt-0.5 shrink-0" />
                {error}
              </p>
            )}
          </>
        )}
      </div>

      <BottomBar>
        <PrimaryButton
          disabled={!canAdvance}
          onClick={() => {
            if (isConfirm) void submit();
            else next();
          }}
        >
          {primaryLabel}
        </PrimaryButton>
      </BottomBar>
    </main>
  );
}

/** 확인 단계의 요약 한 줄. 줄 전체가 아니라 "수정"만 눌린다. */
function SummaryRow({
  label,
  value,
  onEdit,
}: {
  label: string;
  value: string;
  onEdit: () => void;
}) {
  return (
    <div className="flex items-center gap-3 px-3" style={{ minHeight: 52 }}>
      <span className="w-16 shrink-0 text-sm text-ink-soft">{label}</span>
      <span className="min-w-0 flex-1 truncate font-semibold">{value}</span>
      <button
        type="button"
        onClick={onEdit}
        className="flex shrink-0 items-center gap-1 py-2 pl-2 text-sm font-semibold text-ink-soft underline"
      >
        <PencilSimple size={14} weight="bold" />
        수정
      </button>
    </div>
  );
}

/**
 * 요약 화면에서 계좌번호를 전부 보여주지 않는다. 길에서 폼을 열어 둔 채
 * 화면을 남에게 보일 수 있고, 손님이 맞는지 확인하는 데는 뒤 4자리로 충분하다.
 */
function maskAccount(no: string): string {
  if (no.length <= 4) return no;
  return "*".repeat(Math.min(no.length - 4, 8)) + no.slice(-4);
}
