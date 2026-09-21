// Tailwind v4는 @tailwindcss/postcss 를 쓴다.
// v3처럼 tailwindcss 플러그인을 직접 넣으면 동작하지 않는다.
export default {
  plugins: { "@tailwindcss/postcss": {} },
};
