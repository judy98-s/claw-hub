"use client";

import { ArrowLeft } from "@phosphor-icons/react";

import { Mascot, type MascotMood } from "@/components/mascot";
import { storeConfig } from "@/lib/stores";
import type { StoreType } from "@/lib/stores";

/**
 * 손님 폼 9단계의 공통 껍데기 (docs/DESIGN-V2.md §6.1).
 *
 * 한 화면에 질문 하나. 애니메이션 없음. 주 버튼은 하단 고정.
 * 고장난 기계 앞에서 한 손으로, 지하 LTE 에서 끝까지 가게 만드는 것이 목적이다.
 */

/** 기계 명패. ink 바탕에 캐릭터와 기계 이름. 모든 단계 상단에 고정된다. */
export function MachinePlate({
  storeType,
  storeName,
  machineLabel,
  mood,
}: {
  storeType: string;
  storeName: string;
  machineLabel: string;
  mood: MascotMood;
}) {
  return (
    <div className="flex items-center gap-2 rounded-md bg-ink py-2 pl-2 pr-3">
      <Mascot store={storeType as StoreType} mood={mood} size={40} />
      <div className="min-w-0 flex-1">
        <p className="font-mono text-xs text-action">{machineLabel}</p>
        <p className="truncate text-[15px] font-bold text-paper">{storeName}</p>
      </div>
      <span className="shrink-0 font-mono text-[11px] text-pink">환불 접수</span>
    </div>
  );
}

/**
 * 진행바와 뒤로 가기.
 *
 * 분모는 입력 단계 수다. 확인 단계는 숫자 대신 "확인"으로 보여준다 —
 * 요약을 보는 중에 "7 / 6" 같은 숫자가 뜨면 아직 더 남은 줄로 읽힌다.
 */
export function StepHeader({
  current,
  total,
  isConfirm,
  onBack,
  children,
}: {
  current: number;
  total: number;
  isConfirm: boolean;
  onBack: (() => void) | null;
  children: React.ReactNode;
}) {
  const filled = isConfirm ? 1 : current / total;

  return (
    <header className="grid gap-3 px-3 pt-3.5">
      <div className="flex items-center justify-between">
        {onBack ? (
          <button
            type="button"
            onClick={onBack}
            aria-label="이전 단계로"
            className="grid size-11 place-items-center rounded-md text-ink"
          >
            <ArrowLeft size={22} weight="bold" />
          </button>
        ) : (
          // 첫 단계에서는 숨기지만 자리는 비워 둔다. 안 그러면 진행 표시가
          // 단계마다 좌우로 튄다.
          <span className="size-11" aria-hidden />
        )}
        <span className="font-mono text-sm text-ink-soft">
          {isConfirm ? "확인" : `${current} / ${total}`}
        </span>
      </div>

      <div
        className="h-2 overflow-hidden rounded-none border-2 border-ink bg-paper-sunken"
        role="progressbar"
        aria-valuemin={1}
        aria-valuemax={total}
        aria-valuenow={isConfirm ? total : current}
      >
        <div className="h-full bg-pink" style={{ width: `${filled * 100}%` }} />
      </div>

      {children}
    </header>
  );
}

/**
 * 하단 고정 행동 바.
 *
 * safe-area 를 더해 띄운다. 아이폰에서 홈 인디케이터에 버튼이 깔리면
 * 마지막 한 탭이 안 눌리고, 손님은 폼이 멈춘 줄 안다.
 */
export function BottomBar({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="fixed inset-x-0 bottom-0 z-30 border-t-2 border-ink bg-paper px-4 pt-3"
      style={{ paddingBottom: "calc(1.25rem + env(safe-area-inset-bottom))" }}
    >
      <div className="mx-auto max-w-md">{children}</div>
    </div>
  );
}

/** 주 버튼. 높이 58, 전체 너비. 비활성은 눌리지 않는 것이 보여야 한다. */
export function PrimaryButton({
  disabled,
  onClick,
  type = "button",
  children,
}: {
  disabled?: boolean;
  onClick?: () => void;
  type?: "button" | "submit";
  children: React.ReactNode;
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={[
        "h-[58px] w-full rounded-md border-2 border-ink text-lg font-extrabold",
        disabled
          ? "cursor-not-allowed bg-paper-sunken text-ink-soft"
          : "bg-action text-ink shadow-[4px_4px_0_#16130F] active:translate-x-[3px] active:translate-y-[3px] active:shadow-[1px_1px_0_#16130F]",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

/**
 * 선택 버튼. 증상·금액·결제수단·은행이 전부 이 모양이다.
 *
 * 드롭다운을 쓰지 않는 이유: select 를 열고 고르는 건 탭 두 번에
 * 스크롤이고, 길에서 한 손으로 쓰는 화면에서는 그만큼 이탈한다.
 */
export function ChoiceButton({
  selected,
  onClick,
  minHeight,
  children,
}: {
  selected: boolean;
  onClick: () => void;
  minHeight?: number;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      style={minHeight ? { minHeight } : undefined}
      className={[
        "grid content-center gap-0.5 rounded-md border-2 border-ink p-3 text-left",
        selected ? "bg-pink-soft text-ink" : "bg-surface text-ink",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

/** 질문 제목과 보조 문장. 본문 맨 위에 온다. */
export function StepTitle({
  title,
  hint,
}: {
  title: string;
  hint?: React.ReactNode;
}) {
  return (
    <div className="grid gap-1.5">
      <h1 className="text-[27px] font-extrabold leading-tight">{title}</h1>
      {hint && <p className="text-base text-ink-soft">{hint}</p>}
    </div>
  );
}

/** 단계별 캐릭터 표정 (§6.1). 1~3 걱정, 4~6 평온, 확인 기쁨. */
export function moodForStep(index: number, isConfirm: boolean): MascotMood {
  if (isConfirm) return "happy";
  return index <= 3 ? "worried" : "calm";
}

export { storeConfig };
