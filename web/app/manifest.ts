import type { MetadataRoute } from "next";

/**
 * 사장님 대시보드를 홈화면 앱으로 설치할 수 있게 한다.
 *
 * 이 매니페스트는 두 곳에서 쓰인다.
 *  1. 브라우저의 "홈 화면에 추가"
 *  2. Play 스토어에 올리는 TWA 껍데기 (Bubblewrap 이 여기서 값을 읽는다)
 *
 * 손님 폼은 QR 로 한 번 들어왔다 나가는 화면이라 설치 대상이 아니다.
 * 고장난 기계 앞에서 앱을 설치할 사람은 없다.
 */
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "claw-hub 접수함",
    short_name: "claw-hub",
    description: "인형뽑기 기계 고장·환불 문의 접수",
    start_url: "/admin",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    background_color: "#faf9f7",
    theme_color: "#0f7167",
    lang: "ko",
    categories: ["business", "productivity"],
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      // maskable: 기기마다 아이콘을 원형·사각형 등으로 잘라낸다.
      // any 아이콘을 그대로 쓰면 모서리가 잘려 집게가 사라진다.
      { src: "/icons/maskable-192.png", sizes: "192x192", type: "image/png", purpose: "maskable" },
      { src: "/icons/maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
    shortcuts: [
      { name: "접수함", url: "/admin" },
      { name: "기계", url: "/admin/machines" },
    ],
  };
}
