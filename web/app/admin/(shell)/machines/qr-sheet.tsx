"use client";

import { DownloadSimple, Printer, X } from "@phosphor-icons/react";

import { Button } from "@/components/ui/button";
import { Mascot } from "@/components/mascot";
import { STORES, type StoreType } from "@/lib/stores";

type Machine = { id: string; code: string; label: string; qrTarget: string };

/**
 * QR 스티커 시트.
 *
 * QR만 인쇄하면 손님은 그게 뭔지 모르고 지나친다. 기계 이름과
 * "작동 불량 / 오류 시 스캔" 문구를 함께 넣어야 스티커가 일을 한다.
 */
export function QrSheet({
  machine,
  storeType = "claw",
  onClose,
}: {
  machine: Machine;
  storeType?: StoreType;
  onClose: () => void;
}) {
  const qrSrc = `/api/admin/machines/${machine.id}/qr.png`;
  const cfg = STORES[storeType] ?? STORES.claw;

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

        {/*
          인쇄되는 스티커 본체.

          크기를 mm 로 둔다. 실제로 잘라 기계에 붙이는 물건이라, 화면
          픽셀로 두면 프린터·배율에 따라 매번 다른 크기로 나온다.
          60mm 는 기계 전면에 붙여 팔 뻗으면 찍히는 크기다.

          글자는 검정으로 고정한다 — 다크 모드에서 인쇄하면 잉크만 먹고
          안 읽힌다. 바탕만 매장 tint 를 쓴다. 흰 스티커는 기계에 붙이면
          묻혀서 손님이 그냥 지나친다.
        */}
        <div
          id="sticker"
          style={{ width: "60mm", background: cfg.tint }}
          className="mx-auto grid justify-items-center gap-2 rounded-xl border-2 border-black p-4 text-black"
        >
          <div className="flex items-center gap-1.5">
            <Mascot store={storeType} mood="worried" size={28} />
            <p className="text-center text-[15px] font-bold leading-tight">
              {cfg.stickerHeadline}
            </p>
          </div>
          {/* QR 은 흰 바탕 위에 둔다. tint 위에 바로 얹으면 대비가
              떨어져 카메라가 못 읽는 프린터가 있다. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={qrSrc}
            alt={`${machine.label} 신고 QR`}
            style={{ width: "34mm", height: "34mm" }}
            className="rounded bg-white p-1"
          />
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
          /* 바탕색은 기본적으로 인쇄되지 않는다. tint 가 빠지면 흰
             스티커가 나와서 미리보기와 다른 물건이 된다. */
          #sticker,
          #sticker * {
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
          }
        }
      `}</style>
    </div>
  );
}
