import type { Metadata, Viewport } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "claw-hub",
  description: "인형뽑기 기계 고장·환불 문의 접수",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // maximumScale 을 잠그지 않는다. 사장님이 사진 속 증상을 확대해서 봐야 하고,
  // 확대를 막는 건 접근성 위반이다.
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#faf9f7" },
    { media: "(prefers-color-scheme: dark)", color: "#0f0e0d" },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ko">
      <head>
        {/*
          Pretendard 동적 서브셋. 브라우저가 unicode-range 를 보고 실제로
          쓰인 글리프 범위만 받는다 — 전체 가변 폰트는 2MB라 지하 매장
          LTE에서 손님 폼이 먼저 죽는다.

          자체 호스팅이다. Google Fonts 링크가 아니라 /public 의 정적
          파일이라, 외부 요청도 추적도 없다.
        */}
        <link rel="preconnect" href="/" />
        <link rel="stylesheet" href="/fonts/pretendard.css" />
      </head>
      <body className="min-h-[100dvh] antialiased">{children}</body>
    </html>
  );
}
