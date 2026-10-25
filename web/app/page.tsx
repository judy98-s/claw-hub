import Link from "next/link";
import { ArrowRight, ShieldCheck, Package } from "@phosphor-icons/react/dist/ssr";

import { Mascot } from "@/components/mascot";
import { STORE_TYPES, STORES } from "@/lib/stores";

export const metadata = {
  title: "claw-hub — 무인매장 환불 접수",
  description:
    "QR 한 장으로 손님이 직접 접수하고, 사장님은 알림으로 확인하고 승인합니다. 인형뽑기·코인노래방·무인세탁.",
};

/*
  랜딩 v2 — docs/DESIGN-V2.md §6.3.

  장식을 허용한 **유일한** 화면이다. 손님 폼은 계좌번호를 넣는 자리라
  화려하면 신뢰가 깎이고, 대시보드는 하루 수십 번 보므로 밀도가 먼저다.
  여기만 다르다.

  오락실 톤: 굵은 먹선 + 각진 그림자 + 노란 버튼. 그라데이션·글래스·블러는
  쓰지 않는다 — 평면이라야 어느 화면에서나 같은 색으로 나온다.

  색의 역할: action(노랑)은 누르는 것, pink 는 꾸미는 것.
*/

export default function Landing() {
  return (
    <div className="bg-paper text-ink">
      <Marquee />
      <Header />
      <Hero />
      <Stages />
      <StoreTypes />
      <Trust />
      <Closing />
      <Footer />
    </div>
  );
}

/** 전광판 띠. 오락실 간판에서 가져온 유일한 움직임이다. */
function Marquee() {
  const line = "무인매장 환불 접수 · QR 한 장이면 끝 · 손님은 앱을 깔지 않습니다";
  return (
    <div className="overflow-hidden border-b-2 border-ink bg-ink py-2">
      <div className="flex items-center gap-3 px-4 text-[13px] font-semibold tracking-wide text-action">
        <Bulbs />
        <span className="whitespace-nowrap">{line}</span>
        <Bulbs />
        <span className="hidden whitespace-nowrap sm:inline">{line}</span>
      </div>
    </div>
  );
}

function Bulbs() {
  return (
    <span aria-hidden className="flex shrink-0 gap-1.5">
      {[0, 1, 2].map((i) => (
        <span key={i} className="size-2 rounded-full bg-action" />
      ))}
    </span>
  );
}

function Header() {
  return (
    <header className="mx-auto flex max-w-5xl items-center justify-between px-5 py-4">
      <span className="flex items-center gap-2 text-xl font-extrabold">
        <Mascot size={32} />
        claw-hub
      </span>
      <nav className="flex items-center gap-2 text-sm">
        <Link href="/admin/login" className="px-3 py-2 font-semibold hover:underline">
          로그인
        </Link>
        <Link
          href="/admin/signup"
          className="rounded-md border-2 border-ink bg-action px-4 py-2 font-extrabold text-ink shadow-[3px_3px_0_#16130F] transition-transform active:translate-x-[2px] active:translate-y-[2px] active:shadow-[1px_1px_0_#16130F]"
        >
          매장 만들기
        </Link>
      </nav>
    </header>
  );
}

function Hero() {
  return (
    <section className="mx-auto max-w-5xl px-5 pb-16 pt-6">
      <div className="grid items-center gap-12 lg:grid-cols-[1.05fr_1fr]">
        <div className="grid gap-6">
          <p className="w-fit rounded-md border-2 border-ink bg-pink-soft px-3 py-1 text-sm font-extrabold">
            무인매장 환불 접수
          </p>

          {/* 헤드라인은 제품이 아니라 사라지는 고통을 말한다.
              "환불 관리 솔루션" 은 사장님의 언어가 아니다. */}
          <h1 className="text-[clamp(2.6rem,7.4vw,4.6rem)] font-black leading-[1.08] tracking-[-0.035em]">
            <span className="inline-block -rotate-[1.5deg] bg-ink px-3 py-1 text-pink">
              &ldquo;돈만 먹었어요&rdquo;
            </span>
            <br />
            전화를 안 받아도
            <br />
            됩니다
          </h1>

          <p className="max-w-sm text-[17px] leading-[1.65] text-ink-soft">
            기계마다 QR 한 장. 손님이 직접 접수하고, 사장님은 알림으로 확인하고
            승인만 하면 됩니다.
          </p>

          <div className="flex flex-wrap items-center gap-4">
            <Link
              href="/admin/signup"
              className="group flex h-[60px] items-center gap-2 rounded-md border-[3px] border-ink bg-action pl-7 pr-5 text-[18px] font-extrabold shadow-[5px_5px_0_#16130F] transition-transform active:translate-x-[3px] active:translate-y-[3px] active:shadow-[2px_2px_0_#16130F]"
            >
              무료로 시작하기
              <ArrowRight size={20} weight="bold" />
            </Link>
            <Link href="/admin/login" className="font-bold underline underline-offset-4">
              이미 쓰고 있어요
            </Link>
          </div>

          <ul className="flex flex-wrap gap-2">
            {["설치 없음", "손님은 앱을 깔지 않습니다", "카드는 매장에서 취소"].map((t) => (
              <li
                key={t}
                className="rounded-md border-2 border-ink bg-surface px-3 py-1.5 text-[13px] font-bold"
              >
                {t}
              </li>
            ))}
          </ul>
        </div>

        <ArcadeMachine />
      </div>
    </section>
  );
}

/*
  뽑기 기계. 스크린샷을 넣지 않는 이유: 보여줄 실적이 아직 없고 가짜
  대시보드 이미지는 금방 들킨다. 기계와 알림 한 장이 더 설명이 된다.
*/
function ArcadeMachine() {
  return (
    <div className="relative mx-auto w-full max-w-[20rem] pb-10">
      <div className="rounded-xl border-4 border-ink bg-pink-soft p-3 shadow-[8px_8px_0_#16130F]">
        {/* 상단 전광판 */}
        <div className="mb-3 flex items-center justify-center gap-2 rounded-md border-2 border-ink bg-ink py-2">
          <span aria-hidden className="size-2 rounded-full bg-action" />
          <span className="text-[13px] font-extrabold tracking-widest text-action">CLAW HUB</span>
          <span aria-hidden className="size-2 rounded-full bg-action" />
        </div>

        {/* 유리창 */}
        <div className="grid place-items-center rounded-md border-2 border-ink bg-surface py-6">
          <Mascot mood="worried" mode="main" size={150} title="집게에 집히지 못한 인형" />
        </div>

        {/* 출구와 투입구 */}
        <div className="mt-3 flex items-center gap-2">
          <div className="h-8 flex-1 rounded-md border-2 border-ink bg-ink" />
          <div className="grid size-8 place-items-center rounded-md border-2 border-ink bg-surface">
            <span aria-hidden className="h-3 w-1 rounded-full bg-ink" />
          </div>
        </div>
      </div>

      {/* 알림 카드. 기계에 살짝 걸치고 기울여 둔다. */}
      <div className="absolute -bottom-2 -left-2 w-[13.5rem] -rotate-[3deg] rounded-md border-2 border-ink bg-surface p-3 shadow-[4px_4px_0_#16130F]">
        <p className="text-[14px] font-extrabold">7번 기계 · 돈만 빠짐</p>
        <p className="mt-0.5 text-[13px] text-ink-soft">1,000원 · 방금</p>
        <div className="mt-2 flex gap-1.5">
          <span className="flex-1 rounded border-2 border-ink bg-action py-1 text-center text-[12px] font-extrabold">
            승인
          </span>
          <span className="flex-1 rounded border-2 border-ink bg-paper-sunken py-1 text-center text-[12px] font-bold text-ink-soft">
            보류
          </span>
        </div>
      </div>
    </div>
  );
}

const STAGES = [
  {
    tag: "STAGE 1",
    title: "손님이 QR을 찍습니다",
    body: "기계에 붙인 스티커를 찍으면 바로 접수 화면이 열립니다. 앱을 깔 필요도, 회원가입도 없습니다.",
    mood: "worried",
  },
  {
    tag: "STAGE 2",
    title: "사장님에게 알림이 갑니다",
    body: "어느 기계에서 얼마가 빠졌는지 사진과 함께 옵니다. 같은 기계에 민원이 몰리면 고장을 따로 알려줍니다.",
    mood: "calm",
  },
  {
    tag: "CLEAR!",
    title: "승인하면 끝납니다",
    body: "현금이면 계좌로, 카드면 매장에서 결제를 취소합니다. 반복 신고와 계좌 돌려쓰기는 자동으로 걸러냅니다.",
    mood: "happy",
  },
] as const;

/*
  3개 균등 카드로 만들지 않는다 (안티-디폴트). 순서가 있는 흐름이라
  균등 카드 셋으로 놓으면 "셋 중 아무거나" 처럼 보인다. 점선으로 끊어
  스테이지가 이어진다는 것을 보이게 한다.
*/
function Stages() {
  return (
    <section className="border-y-2 border-ink bg-ink text-paper">
      <div className="mx-auto max-w-3xl px-5 py-16">
        <h2 className="text-[clamp(1.9rem,4.6vw,3rem)] font-black tracking-[-0.03em]">
          세 판이면 끝납니다
        </h2>

        <ol className="mt-10">
          {STAGES.map((s, i) => (
            <li
              key={s.tag}
              className={`flex items-center gap-5 py-7 ${
                i > 0 ? "border-t-2 border-dashed border-ink-soft" : ""
              }`}
            >
              <div className="grid flex-1 gap-2">
                <span className="text-[13px] font-extrabold tracking-widest text-action">
                  {s.tag}
                </span>
                <h3 className="text-xl font-extrabold">{s.title}</h3>
                <p className="text-pretty leading-relaxed text-paper-sunken">{s.body}</p>
              </div>
              <Mascot mood={s.mood} size={78} className="shrink-0" />
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

/** 매장 6종. 타일마다 그 매장 캐릭터와 tint 를 쓴다. */
function StoreTypes() {
  return (
    <section className="mx-auto max-w-5xl px-5 py-16">
      <h2 className="text-[clamp(1.9rem,4.6vw,3rem)] font-black leading-snug tracking-[-0.03em]">
        사람이 없는 매장은
        <br />
        민원 받을 사람도 없습니다
      </h2>
      <p className="mt-4 max-w-lg text-pretty leading-relaxed text-ink-soft">
        무인으로 돌리는 순간 생기는 문제입니다. 손님은 하소연할 곳이 없고, 사장님은
        퇴근 후에 전화를 받습니다. claw-hub 은 그 사이에 섭니다.
      </p>

      <ul className="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-3">
        {STORE_TYPES.map((key, i) => {
          const s = STORES[key];
          return (
            <li
              key={key}
              className="grid justify-items-center gap-1 rounded-xl border-2 border-ink p-5 shadow-[4px_4px_0_#16130F]"
              style={{
                backgroundColor: s.tint,
                transform: `rotate(${i % 2 === 0 ? -2 : 2}deg)`,
              }}
            >
              <Mascot store={key} mode="main" size={96} />
              <span className="mt-1 font-extrabold">{s.label}</span>
              <span className="text-[13px] font-bold" style={{ color: s.accentText }}>
                {s.characterName}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

const TRUST = [
  {
    icon: ShieldCheck,
    title: "계좌와 번호는 암호화해서 보관합니다",
    body: "사장님 화면에도 필요한 순간에만 보입니다. 사진은 기본 90일 뒤 지워집니다.",
  },
  {
    icon: Package,
    title: "인형 원가까지 같이 봅니다",
    body: "기계별 민원과 매입가를 함께 보면, 어느 기계가 실제로 남기고 있는지 알 수 있습니다.",
  },
];

function Trust() {
  return (
    <section className="border-y-2 border-ink bg-pink-soft">
      <div className="mx-auto grid max-w-5xl divide-y-[3px] divide-ink px-5 sm:grid-cols-2 sm:divide-x-[3px] sm:divide-y-0">
        {TRUST.map((t) => (
          <div key={t.title} className="grid gap-3 py-12 sm:px-8 sm:first:pl-0 sm:last:pr-0">
            <t.icon size={28} weight="duotone" />
            <h3 className="text-lg font-extrabold">{t.title}</h3>
            <p className="text-pretty leading-relaxed text-ink-soft">{t.body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function Closing() {
  return (
    <section className="mx-auto max-w-5xl px-5 py-20">
      <div className="grid justify-items-center gap-5 rounded-2xl border-[3px] border-ink bg-ink px-6 py-14 text-center text-paper shadow-[8px_8px_0_#FF8FB1]">
        <span className="text-[13px] font-extrabold tracking-[0.3em] text-action">INSERT COIN</span>
        <Mascot mood="happy" size={96} />
        <h2 className="text-[clamp(1.7rem,4vw,2.4rem)] font-black leading-snug tracking-[-0.02em]">
          오늘 QR 한 장 붙이고
          <br />
          시작하세요
        </h2>
        <p className="max-w-sm text-pretty text-paper-sunken">
          매장을 만들고 기계를 등록하면 스티커가 바로 인쇄됩니다.
        </p>
        <Link
          href="/admin/signup"
          className="rounded-md border-[3px] border-ink bg-action px-7 py-3.5 text-[18px] font-extrabold text-ink shadow-[5px_5px_0_#FF8FB1] transition-transform active:translate-x-[3px] active:translate-y-[3px] active:shadow-[2px_2px_0_#FF8FB1]"
        >
          매장 만들기
        </Link>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-5 pb-12 text-sm text-ink-soft">
      <span className="flex items-center gap-2 font-bold">
        <Mascot mood="sleep" size={24} />
        claw-hub
      </span>
      <Link href="/privacy" className="hover:underline">
        개인정보 처리방침
      </Link>
    </footer>
  );
}
