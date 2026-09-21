# syntax=docker/dockerfile:1
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Next.js 는 rewrites 를 빌드 시점에 routes-manifest.json 으로 굳힌다.
# next start 는 런타임 환경변수를 다시 읽지 않는다. 그래서 프로덕션에서는
# rewrites 에 의존하지 않고 Caddy 가 /api 를 Go 로 보낸다.
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000

# standalone 출력은 node_modules 없이 뜬다.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public

USER node
EXPOSE 3000
CMD ["node", "server.js"]
