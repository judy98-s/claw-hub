/**
 * Pretendard 동적 서브셋을 node_modules 에서 public/fonts 로 복사한다.
 *
 * 폰트 파일 92개(3MB)를 저장소에 커밋하지 않기 위해서다. npm 패키지가
 * 단일 진실 공급원이고, 여기서는 경로만 public 기준으로 고쳐 쓴다.
 *
 * 동적 서브셋을 쓰는 이유: 전체 가변 폰트는 2MB라 지하 매장 LTE에서
 * 손님 폼이 먼저 죽는다. 서브셋이면 브라우저가 unicode-range 를 보고
 * 실제로 쓰인 글리프 범위만 받는다.
 */
import { cp, mkdir, readFile, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const src = resolve(root, "node_modules/pretendard/dist/web/variable");
const outDir = resolve(root, "public/fonts");

if (!existsSync(src)) {
  // 어디서 실행하라는지 말해주지 않으면 저장소 루트에서 npm install 을
  // 돌리게 되고, package.json 이 web/ 에 있으므로 ENOENT 가 난다.
  console.error(
    [
      "pretendard 패키지를 찾을 수 없습니다.",
      "",
      "  저장소 루트에서:   make web        (의존성을 자동으로 설치합니다)",
      "  직접 설치하려면:   cd web && npm install",
      "",
      "  package.json 은 web/ 안에 있습니다. 저장소 루트에서 npm install 을",
      "  실행하면 ENOENT 가 납니다.",
    ].join("\n"),
  );
  process.exit(1);
}

await mkdir(outDir, { recursive: true });
await cp(resolve(src, "woff2-dynamic-subset"), resolve(outDir, "pretendard"), {
  recursive: true,
});

const css = await readFile(
  resolve(src, "pretendardvariable-dynamic-subset.css"),
  "utf8",
);
await writeFile(
  resolve(outDir, "pretendard.css"),
  css.replaceAll("./woff2-dynamic-subset/", "/fonts/pretendard/"),
);

console.log("Pretendard 서브셋을 public/fonts 에 배치했습니다.");
