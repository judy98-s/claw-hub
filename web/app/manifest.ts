import type { MetadataRoute } from "next";

/**
 * 사장님 대시보드를 홈화면에 설치할 수 있게 한다.
 *
 * 손님 폼은 QR 로 한 번 들어왔다 나가는 화면이라 설치할 이유가 없다.
 * 사장님은 매일 여는 화면이므로 아이콘 하나가 주소 입력보다 낫다.
 *
 * 설치해도 푸시 알림은 오지 않는다. 알림은 Slack 이 담당한다.
 */
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "claw-hub 접수함",
    short_name: "claw-hub",
    description: "인형뽑기 기계 고장·환불 문의 접수",
    start_url: "/admin",
    display: "standalone",
    background_color: "#faf9f7",
    theme_color: "#0f7167",
    lang: "ko",
    icons: [
      { src: "/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "any" },
    ],
  };
}
