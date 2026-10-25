import Link from "next/link";
import {
  QrCode,
  BellRinging,
  CheckCircle,
  ShieldCheck,
  Package,
  ArrowRight,
} from "@phosphor-icons/react/dist/ssr";

import { Mascot } from "@/components/mascot";

export const metadata = {
  title: "claw-hub — 무인매장 환불 접수",
  description:
    "QR 한 장으로 손님이 직접 접수하고, 사장님은 알림으로 확인하고 승인합니다. 인형뽑기·코인노래방·무인세탁.",
};

/*
  랜딩 페이지.

  읽는 사람: 무인매장 사장님. 기술에 관심이 없고 지금 민원 전화에
  시달리고 있다. 기능을 나열하지 않고 **그 전화가 사라진다**는 한 가지를
  먼저 말한다.

  색의 역할을 나눠 둔다. 로즈는 **장식**(배경·마스코트·강조면), 액센트
  청록은 **행동**(버튼·링크). 분홍 바탕에 분홍 버튼을 놓으면 누를 것이
  어디인지 사라진다.

  앱 안쪽보다 시각적 변주를 올린 유일한 화면이다. 안쪽은 계좌와 돈을
  다루는 자리라 화려함이 신뢰를 깎지만 여기는 처음 보는 사람을 붙잡아야
  한다. 그래도 globals.css 의 안티-디폴트는 여기에도 적용된다 — 보라
  그라데이션도, 글래스모피즘도, **3개 균등 카드**도 쓰지 않는다.
*/

export default function Landing() {
  return (
    <div className="bg-rose-50 text-ink-900">
      <Header />
      <Hero />
      <How />
      <ForWhom />
      <Trust />
      <Closing />
      <Footer />
    </div>
  );
}

function Header() {
  return (
    <header className="mx-auto flex max-w-5xl items-center justify-between px-5 py-5">
      <span className="flex items-center gap-2 text-lg font-bold">
        <Mascot size={30} />
        claw-hub
      </span>
      <nav className="flex items-center gap-1 text-sm">
        <Link href="/admin/login" className="rounded-full px-4 py-2 font-medium hover:bg-rose-100">
          로그인
        </Link>
        <Link
          href="/admin/signup"
          className="rounded-full bg-ink-900 px-4 py-2 font-semibold text-rose-50"
        >
          매장 만들기
        </Link>
      </nav>
    </header>
  );
}

function Hero() {
  return (
    <section className="relative overflow-hidden">
      {/*
        배경 면. 그라데이션 대신 **단색 덩어리**를 겹친다. 안티-디폴트의
        "보라 그라데이션" 과 같은 이유이고, 평면이라야 어느 화면에서나
        같은 색으로 나온다.
      */}
      <div
        aria-hidden
        className="pointer-events-none absolute -right-32 -top-40 size-[34rem] rounded-full bg-rose-100"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute -left-40 top-56 size-[26rem] rounded-full bg-rose-100/60"
      />

      <div className="relative mx-auto max-w-5xl px-5 pb-20 pt-6 sm:pt-14">
        <div className="grid items-center gap-12 sm:grid-cols-[1.1fr_1fr]">
          <div className="grid gap-7">
            <p className="w-fit rounded-full border border-rose-200 bg-white/70 px-3.5 py-1.5 text-sm font-semibold text-rose-700">
              무인매장 환불 접수
            </p>

            {/* 헤드라인은 제품이 아니라 사라지는 고통을 말한다.
                "환불 관리 솔루션" 은 사장님의 언어가 아니다. */}
            <h1 className="text-5xl font-bold leading-[1.12] tracking-[-0.02em] sm:text-6xl">
              <span className="text-ink-500">&ldquo;돈만 먹었어요&rdquo;</span>
              <br />
              전화를
              <br className="sm:hidden" /> 안 받아도
              <br />
              됩니다
            </h1>

            <p className="max-w-sm text-pretty text-lg leading-relaxed text-ink-600">
              기계마다 QR 한 장. 손님이 직접 접수하고, 사장님은 알림으로 확인하고
              승인만 하면 됩니다.
            </p>

            <div className="flex flex-wrap items-center gap-2">
              <Link
                href="/admin/signup"
                className="group flex items-center gap-2 rounded-full bg-accent-600 py-3.5 pl-7 pr-5 font-semibold text-white transition-colors hover:bg-accent-700"
              >
                무료로 시작하기
                <ArrowRight size={18} weight="bold" className="transition-transform group-hover:translate-x-0.5" />
              </Link>
              <Link
                href="/admin/login"
                className="rounded-full px-6 py-3.5 font-semibold text-ink-700 hover:bg-rose-100"
              >
                이미 쓰고 있어요
              </Link>
            </div>

            <ul className="flex flex-wrap gap-x-5 gap-y-1.5 text-sm text-ink-500">
              <li>설치 없음</li>
              <li>손님은 앱을 깔지 않습니다</li>
              <li>카드 결제는 매장에서 취소</li>
            </ul>
          </div>

          <HeroArt />
        </div>
      </div>
    </section>
  );
}

/*
  히어로 그림. 스크린샷을 넣지 않는 이유: 보여줄 실적이 아직 없고 가짜
  대시보드 이미지는 금방 들킨다. 대신 사장님이 받게 될 알림 한 장을
  그대로 그린다 — 추상적인 일러스트보다 이게 더 설명이 된다.
*/
function HeroArt() {
  return (
    <div className="relative mx-auto w-full max-w-[19rem]">
      <div className="rounded-[2rem] border border-rose-200 bg-white p-7 shadow-[0_1px_2px_rgba(0,0,0,.04),0_12px_32px_-12px_rgba(157,83,93,.25)]">
        <div className="grid justify-items-center gap-5">
          <Mascot mood="worried" size={132} claw title="집게에 집히지 못한 인형" />

          <div className="grid w-full gap-1.5 text-center">
            <p className="font-bold">7번 기계 · 돈만 빠짐</p>
            <p className="text-sm text-ink-500">1,000원 · 방금</p>
          </div>

          <div className="flex w-full gap-2">
            <span className="flex-1 rounded-xl bg-accent-600 py-2.5 text-center text-sm font-semibold text-white">
              승인
            </span>
            <span className="flex-1 rounded-xl bg-ink-100 py-2.5 text-center text-sm font-semibold text-ink-600">
              보류
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

const STEPS = [
  {
    icon: QrCode,
    title: "손님이 QR을 찍습니다",
    body: "기계에 붙인 스티커를 찍으면 바로 접수 화면이 열립니다. 앱을 깔 필요도, 회원가입도 없습니다.",
  },
  {
    icon: BellRinging,
    title: "사장님에게 알림이 갑니다",
    body: "어느 기계에서 얼마가 빠졌는지 사진과 함께 옵니다. 같은 기계에 민원이 몰리면 고장을 따로 알려줍니다.",
  },
  {
    icon: CheckCircle,
    title: "승인하면 끝납니다",
    body: "현금이면 계좌로, 카드면 매장에서 결제를 취소합니다. 반복 신고와 계좌 돌려쓰기는 자동으로 걸러냅니다.",
  },
];

/*
  3개를 나란히 둔 카드로 만들지 않는다 (안티-디폴트). 이건 **순서가 있는
  흐름**이라 균등한 카드 셋으로 놓으면 "셋 중 아무거나" 처럼 보인다.
  세로로 쌓고 선으로 이어 순서를 눈에 보이게 한다.
*/
function How() {
  return (
    <section className="bg-white">
      <div className="mx-auto max-w-3xl px-5 py-20">
        <h2 className="text-3xl font-bold tracking-[-0.01em]">세 단계로 끝납니다</h2>

        <ol className="mt-12 grid gap-10">
          {STEPS.map((s, i) => (
            <li key={s.title} className="relative grid grid-cols-[auto_1fr] gap-x-5">
              {/* 연결선. 마지막 항목에는 긋지 않는다. */}
              {i < STEPS.length - 1 && (
                <span
                  aria-hidden
                  className="absolute left-[1.4rem] top-14 h-[calc(100%+1rem)] w-px bg-rose-200"
                />
              )}
              <span className="relative z-10 flex size-11 items-center justify-center rounded-full bg-rose-100 text-rose-700">
                <s.icon size={22} weight="duotone" />
              </span>
              <div className="grid gap-2 pt-1.5">
                <h3 className="text-lg font-bold">
                  <span className="mr-2 text-rose-400">{i + 1}</span>
                  {s.title}
                </h3>
                <p className="text-pretty leading-relaxed text-ink-600">{s.body}</p>
              </div>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

const SHOPS = ["인형뽑기", "코인노래방", "무인세탁", "무인사진관", "스터디카페", "무인카페"];

function ForWhom() {
  return (
    <section className="mx-auto max-w-5xl px-5 py-20">
      <div className="grid gap-10 sm:grid-cols-[1fr_1fr] sm:items-center">
        <div className="grid gap-4">
          <h2 className="text-3xl font-bold leading-snug tracking-[-0.01em]">
            사람이 없는 매장은
            <br />
            민원 받을 사람도 없습니다
          </h2>
          <p className="text-pretty leading-relaxed text-ink-600">
            무인으로 돌리는 순간 생기는 문제입니다. 손님은 하소연할 곳이 없고,
            사장님은 퇴근 후에 전화를 받습니다. claw-hub 은 그 사이에 섭니다.
          </p>
        </div>

        <ul className="flex flex-wrap gap-2">
          {SHOPS.map((s) => (
            <li
              key={s}
              className="rounded-full border border-rose-200 bg-white px-4 py-2 font-medium"
            >
              {s}
            </li>
          ))}
        </ul>
      </div>
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
    body: "기계별 민원과 인형 매입가를 함께 보면, 어느 기계가 실제로 남기고 있는지 알 수 있습니다.",
  },
];

function Trust() {
  return (
    <section className="bg-white">
      <div className="mx-auto grid max-w-5xl gap-10 px-5 py-20 sm:grid-cols-2">
        {TRUST.map((t) => (
          <div key={t.title} className="grid gap-3">
            <span className="flex size-11 items-center justify-center rounded-full bg-rose-100 text-rose-700">
              <t.icon size={22} weight="duotone" />
            </span>
            <h3 className="text-lg font-bold">{t.title}</h3>
            <p className="text-pretty leading-relaxed text-ink-600">{t.body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function Closing() {
  return (
    <section className="mx-auto max-w-5xl px-5 py-20">
      <div className="relative grid justify-items-center gap-6 overflow-hidden rounded-[2.5rem] bg-ink-900 px-6 py-16 text-center text-rose-50">
        <div
          aria-hidden
          className="pointer-events-none absolute -bottom-24 -right-16 size-72 rounded-full bg-white/[0.04]"
        />
        <Mascot mood="happy" size={92} />
        <h2 className="relative text-3xl font-bold leading-snug tracking-[-0.01em]">
          오늘 QR 한 장 붙이고
          <br />
          시작하세요
        </h2>
        <p className="relative max-w-sm text-pretty text-ink-300">
          매장을 만들고 기계를 등록하면 스티커가 바로 인쇄됩니다.
        </p>
        <Link
          href="/admin/signup"
          className="relative rounded-full bg-white px-7 py-3.5 font-semibold text-ink-900 transition-transform hover:scale-[1.02]"
        >
          매장 만들기
        </Link>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-5 pb-12 text-sm text-ink-500">
      <span className="flex items-center gap-2">
        <Mascot size={22} />
        claw-hub
      </span>
      <Link href="/privacy" className="hover:underline">
        개인정보 처리방침
      </Link>
    </footer>
  );
}
