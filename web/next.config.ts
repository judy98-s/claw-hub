import type { NextConfig } from "next";

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  // Docker 배포에서 node_modules 없이 뜨게 한다.
  output: "standalone",

  // 브라우저는 같은 오리진으로만 요청한다. 세션 쿠키가 SameSite=Lax 라
  // 크로스 오리진으로 부르면 쿠키가 안 실린다.
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${apiOrigin}/api/:path*` },
      { source: "/healthz", destination: `${apiOrigin}/healthz` },
    ];
  },

  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          // 손님이 QR로 들어오는 페이지다. 다른 사이트가 iframe으로
          // 감싸서 클릭을 가로채면 안 된다.
          { key: "X-Frame-Options", value: "DENY" },
        ],
      },
    ];
  },
};

export default nextConfig;
