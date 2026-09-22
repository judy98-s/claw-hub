/**
 * Digital Asset Links.
 *
 * TWA(플레이스토어에 올리는 껍데기 앱)가 이 주소를 읽어서 "이 앱과 이
 * 웹사이트는 같은 주인"임을 확인한다. 확인되지 않으면 앱 안에 브라우저
 * 주소창이 남아서, 앱이 아니라 크롬 창처럼 보인다.
 *
 * 값은 환경변수로 넣는다. 서명 키 지문은 keystore 를 만들어야 나오는데,
 * 그건 배포하는 사람의 비밀이고 저장소에 들어가면 안 된다.
 *
 *   ANDROID_PACKAGE_NAME=com.example.clawhub
 *   ANDROID_SHA256_FINGERPRINTS=AA:BB:...:FF          (쉼표로 여러 개 가능)
 *
 * 지문이 두 개 필요한 경우가 흔하다 — 내 keystore 지문과, Play 앱 서명을
 * 쓸 때 구글이 다시 서명한 지문. 둘 다 넣지 않으면 스토어에서 받은 앱만
 * 주소창이 뜬다.
 */
export const dynamic = "force-dynamic";

export function GET() {
  const pkg = process.env.ANDROID_PACKAGE_NAME ?? "";
  const fingerprints = (process.env.ANDROID_SHA256_FINGERPRINTS ?? "")
    .split(",")
    .map((s) => s.trim().toUpperCase())
    .filter(Boolean);

  // 설정 전에는 빈 배열을 내보낸다. 형식이 잘못된 JSON 을 내보내면
  // 검증 도구가 "파일 없음"과 구분되지 않는 오류를 준다.
  const body =
    pkg && fingerprints.length > 0
      ? [
          {
            relation: ["delegate_permission/common.handle_all_urls"],
            target: {
              namespace: "android_app",
              package_name: pkg,
              sha256_cert_fingerprints: fingerprints,
            },
          },
        ]
      : [];

  return new Response(JSON.stringify(body, null, 2), {
    headers: {
      "Content-Type": "application/json",
      // 지문을 바꿨을 때 캐시 때문에 몇 시간 헤매지 않도록.
      "Cache-Control": "public, max-age=300",
    },
  });
}
