"use client";

import { DownloadSimple, Printer, X } from "@phosphor-icons/react";

import { Button } from "@/components/ui/button";

type Machine = { id: string; code: string; label: string; qrTarget: string };

/**
 * QR 스티커 시트.
 *
 * QR만 인쇄하면 손님은 그게 뭔지 모르고 지나친다. 기계 이름과
 * "작동 불량 / 오류 시 스캔" 문구를 함께 넣어야 스티커가 일을 한다.
 */
export function QrSheet({ machine, onClose }: { machine: Machine; onClose: () => void }) {
  const qrSrc = `/api/admin/machines/${machine.id}/qr.png`;

  /*
    스티커에 인쇄할 주소. qrTarget 에서 코드를 떼고 프로토콜을 지운다.
    "아래 주소에서 코드를 입력하세요"라고 써놓고 주소를 인쇄하지 않으면,
    QR이 긁혀 안 읽힐 때 손님은 갈 곳이 없다.
  */
  const typeInUrl = machine.qrTarget
    .replace(/^https?:\/\//, "")
    .replace(new RegExp(`/${machine.code}$`), "");

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink-950/50 print:static print:bg-transparent">
      <div className="w-full max-w-lg rounded-t-2xl bg-[var(--surface)] p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] print:rounded-none print:p-0">
        <div className="mb-4 flex items-center justify-between print:hidden">
          <h2 className="text-lg font-bold">{machine.label} QR</h2>
          <button type="button" onClick={onClose} aria-label="닫기" className="p-1">
            <X size={20} weight="bold" />
          </button>
        </div>

        {/* 인쇄되는 스티커 본체. 흰 배경에 검정 글씨로 고정한다 —
            다크 모드에서 인쇄하면 잉크만 먹고 안 읽힌다. */}
        <div
          id="sticker"
          className="mx-auto grid w-64 justify-items-center gap-3 rounded-xl border-2 border-black bg-white p-5 text-black"
        >
          <p className="text-center text-[15px] font-bold leading-tight">
            작동 불량 / 오류 시
            <br />
            QR을 스캔해주세요
          </p>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={qrSrc} alt={`${machine.label} 신고 QR`} className="size-40" />
          <div className="text-center">
            <p className="font-bold">{machine.label}</p>
            <p className="font-mono text-sm tracking-wider">{machine.code}</p>
          </div>
          <p className="text-center text-[11px] leading-snug">
            QR이 안 읽히면
            <br />
            <b className="font-mono">{typeInUrl}</b> 에서
            <br />
            코드 <b>{machine.code}</b> 를 입력하세요
          </p>
        </div>

        <div className="mt-4 grid gap-2 print:hidden">
          <Button size="lg" full onClick={() => window.print()}>
            <Printer size={18} weight="regular" /> 스티커 인쇄
          </Button>
          <a
            href={qrSrc}
            download={`qr-${machine.code}.png`}
            className="flex h-11 items-center justify-center gap-2 rounded-lg border border-[var(--line)] font-semibold"
          >
            <DownloadSimple size={18} weight="regular" /> PNG 저장
          </a>
          <p className="text-center text-xs text-[var(--muted)]">{machine.qrTarget}</p>
        </div>
      </div>

      <style jsx global>{`
        @media print {
          body * {
            visibility: hidden;
          }
          #sticker,
          #sticker * {
            visibility: visible;
          }
          #sticker {
            position: absolute;
            left: 0;
            top: 0;
          }
        }
      `}</style>
    </div>
  );
}
