import Link from "next/link";
import {
  QrCode,
  BellRinging,
  CheckCircle,
  ShieldCheck,
  Package,
  Storefront,
} from "@phosphor-icons/react/dist/ssr";

import { Mascot } from "@/components/mascot";

export const metadata = {
  title: "claw-hub — 무인매장 환불 접수",
  description:
    "QR 한 장으로 손님이 직접 접수하고, 사장님은 알림으로 확인하고 승인합니다. 인형뽑기·코인노래방·무인세탁.",
};

/*
  랜딩 페이지.

  읽는 사람: 무인매장 사장님. 기술에 관심이 없고, 지금 민원 전화에
  시달리고 있다. 그래서 기능을 나열하지 않고 **그 전화가 사라진다**는
  한 가지를 먼저 말한다.

  앱 안쪽(손님 폼·대시보드)보다 시각적 변주를 올린 유일한 화면이다.
  앱 안쪽은 돈과 계좌를 다루는 자리라 화려함이 신뢰를 깎지만, 여기는
  처음 보는 사람을 붙잡아야 한다. 그래도 보라 그라데이션과 글래스모피즘은
  쓰지 않는다 — globals.css 의 안티-디폴트는 여기에도 적용된다.
*/

export default function Landing() {
  return (
    <div className="bg-ink-50 text-ink-900">
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
      <span className="flex items-center gap-2 font-bold">
        <Mascot size={28} />
        claw-hub
      </span>
      <nav className="flex items-center gap-4 text-sm">
        <Link href="/admin/login" className="font-medium hover:underline">
          로그인
        </Link>
        <Link
          href="/admin/signup"
          className="rounded-full bg-ink-900 px-4 py-2 font-semibold text-ink-50"
        >
          매장 만들기
        </Link>
      </nav>
    </header>
  );
}

function Hero() {
  return (
    <section className="mx-auto max-w-5xl px-5 pb-16 pt-8 sm:pt-16">
      <div className="grid items-center gap-10 sm:grid-cols-[1.15fr_1fr]">
        <div className="grid gap-6">
          <p className="w-fit rounded-full bg-accent-100 px-3 py-1 text-sm font-semibold text-accent-700">
            무인매장 환불 접수
          </p>

          {/*
            헤드라인은 제품이 아니라 사라지는 고통을 말한다. "환불 관리
            솔루션" 은 사장님의 언어가 아니다.
          */}
          <h1 className="text-balance text-4xl font-bold leading-[1.15] sm:text-5xl">
            &ldquo;돈만 먹었어요&rdquo;
            <br />
            전화를 안 받아도 됩니다
          </h1>

          <p className="max-w-md text-pretty text-lg text-ink-600">
            기계마다 QR 한 장. 손님이 직접 접수하고, 사장님은 알림으로 확인하고
            승인만 하면 됩니다. 매장에 없어도 됩니다.
          </p>

          <div className="flex flex-wrap items-center gap-3">
            <Link
              href="/admin/signup"
              className="rounded-full bg-accent-600 px-6 py-3 font-semibold text-ink-50 transition-colors hover:bg-accent-700"
            >
              무료로 시작하기
            </Link>
            <Link
              href="/admin/login"
              className="rounded-full px-6 py-3 font-semibold text-ink-700 hover:bg-ink-100"
            >
              이미 쓰고 있어요
            </Link>
          </div>

          <p className="text-sm text-ink-500">
            설치 없음 · 손님은 앱을 깔지 않습니다 · 카드 결제는 매장에서 취소
          </p>
        </div>

        <HeroArt />
      </div>
    </section>
  );
}

/*
  히어로 그림. 스크린샷을 넣지 않는 이유: 아직 보여줄 실적이 없고, 가짜
  대시보드 이미지는 금방 들킨다. 대신 이 서비스가 하는 일 한 장면 —
  집게가 놓친 인형 — 을 그린다.
*/
function HeroArt() {
  return (
    <div className="relative mx-auto w-full max-w-xs">
      <div className="rounded-[2rem] border border-ink-200 bg-white p-8 shadow-sm">
        <div className="grid justify-items-center gap-5">
          <Mascot mood="worried" size={150} claw title="집게에 집히지 못한 인형" />
          <div className="grid w-full gap-2 text-center">
            <p className="text-sm font-semibold">7번 기계 · 돈만 빠짐</p>
            <p className="text-sm text-ink-500">1,000원 · 방금</p>
          </div>
          <div className="flex w-full gap-2">
            <span className="flex-1 rounded-lg bg-ok-50 py-2 text-center text-sm font-semibold text-ok-700">
              승인
            </span>
            <span className="flex-1 rounded-lg bg-ink-100 py-2 text-center text-sm font-semibold text-ink-600">
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
    body: "어느 기계에서 얼마가 빠졌는지, 사진과 함께 옵니다. 같은 기계에 민원이 몰리면 고장을 따로 알려줍니다.",
  },
  {
    icon: CheckCircle,
    title: "승인하면 끝납니다",
    body: "현금이면 계좌로, 카드면 매장에서 결제를 취소합니다. 반복 신고와 계좌 돌려쓰기는 자동으로 걸러냅니다.",
  },
];

function How() {
  return (
    <section className="border-y border-ink-200 bg-white">
      <div className="mx-auto max-w-5xl px-5 py-16">
        <h2 className="text-2xl font-bold sm:text-3xl">세 단계로 끝납니다</h2>
        <ol className="mt-10 grid gap-8 sm:grid-cols-3">
          {STEPS.map((s, i) => (
            <li key={s.title} className="grid gap-3">
              <span className="flex size-11 items-center justify-center rounded-xl bg-accent-50 text-accent-700">
                <s.icon size={24} weight="duotone" />
              </span>
              <h3 className="font-bold">
                <span className="mr-1.5 text-ink-400">{i + 1}.</span>
                {s.title}
              </h3>
              <p className="text-pretty text-ink-600">{s.body}</p>
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
    <section className="mx-auto max-w-5xl px-5 py-16">
      <div className="grid gap-8 sm:grid-cols-[1fr_1.1fr] sm:items-center">
        <div className="grid gap-4">
          <h2 className="text-balance text-2xl font-bold sm:text-3xl">
            사람이 없는 매장은
            <br />
            민원 받을 사람도 없습니다
          </h2>
          <p className="text-pretty text-ink-600">
            무인으로 돌리는 순간 생기는 문제입니다. 손님은 하소연할 곳이 없고,
            사장님은 퇴근 후에 전화를 받습니다. claw-hub 은 그 사이에 섭니다.
          </p>
        </div>

        <ul className="flex flex-wrap gap-2">
          {SHOPS.map((s) => (
            <li
              key={s}
              className="flex items-center gap-1.5 rounded-full border border-ink-200 bg-white px-4 py-2 font-medium"
            >
              <Storefront size={18} className="text-ink-400" />
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
    <section className="border-y border-ink-200 bg-white">
      <div className="mx-auto grid max-w-5xl gap-8 px-5 py-16 sm:grid-cols-2">
        {TRUST.map((t) => (
          <div key={t.title} className="grid gap-3">
            <span className="flex size-11 items-center justify-center rounded-xl bg-ink-100 text-ink-700">
              <t.icon size={24} weight="duotone" />
            </span>
            <h3 className="font-bold">{t.title}</h3>
            <p className="text-pretty text-ink-600">{t.body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function Closing() {
  return (
    <section className="mx-auto max-w-5xl px-5 py-20">
      <div className="grid justify-items-center gap-6 rounded-[2rem] bg-ink-900 px-6 py-14 text-center text-ink-50">
        <Mascot mood="happy" size={88} />
        <h2 className="text-balance text-2xl font-bold sm:text-3xl">
          오늘 QR 한 장 붙이고 시작하세요
        </h2>
        <p className="max-w-sm text-pretty text-ink-300">
          매장을 만들고 기계를 등록하면 스티커가 바로 인쇄됩니다.
        </p>
        <Link
          href="/admin/signup"
          className="rounded-full bg-ink-50 px-6 py-3 font-semibold text-ink-900 transition-colors hover:bg-white"
        >
          매장 만들기
        </Link>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-5 pb-10 text-sm text-ink-500">
      <span className="flex items-center gap-2">
        <Mascot size={20} />
        claw-hub
      </span>
      <Link href="/privacy" className="hover:underline">
        개인정보 처리방침
      </Link>
    </footer>
  );
}
