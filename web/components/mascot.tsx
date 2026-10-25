import { STORES, type StoreType } from "@/lib/stores";

/**
 * 매장 캐릭터.
 *
 * 그림은 `web/public/characters/` 의 SVG 42개(6종 × 7)다. 컴포넌트가
 * 그리지 않고 파일을 가리킨다 — 일러스트가 최종본으로 바뀌면 SVG 만
 * 갈아 끼우면 되고(파일명·뷰박스 유지), 42개를 번들에 인라인하면
 * 지하 LTE 에서 손님 폼이 먼저 죽는다.
 *
 * 표정은 **귀(또는 머리 위 요소)의 각도로 먼저 읽힌다.** 기쁨은 서고
 * 걱정·졸림은 처진다. 그래서 22px 로 줄여도 기분이 남는다.
 *
 * 쓰는 자리 (docs/DESIGN-V2.md §5.2)
 *   calm     헤더 로고, 기본
 *   happy    접수 완료, 환불 완료, 랜딩 마무리
 *   worried  손님 폼 1~3단계, 대시보드 알림 말풍선, QR 스티커
 *   sleep    대시보드 "처리할 것 없음", 푸터
 */

export type MascotMood = "calm" | "happy" | "worried" | "sleep";

/**
 * full  전신. 기본.
 * main  소품 포함 전신. 랜딩·캐릭터 소개처럼 큰 자리에만.
 * peek  판을 잡고 빼꼼. SVG 아래 가장자리가 판의 윗선이다.
 */
export type MascotMode = "full" | "main" | "peek";

const RATIO = { full: 212 / 200, main: 212 / 200, peek: 172 / 200 };

function fileName(character: string, mood: MascotMood, mode: MascotMode) {
  if (mode === "main") return `${character}_main`;
  if (mode === "peek") return mood === "worried" ? `${character}_peek_worried` : `${character}_peek`;
  return `${character}_${mood}`;
}

export function Mascot({
  store = "claw",
  mood = "calm",
  mode = "full",
  size = 120,
  className = "",
  title,
}: {
  store?: StoreType;
  mood?: MascotMood;
  mode?: MascotMode;
  /** 너비(px). 높이는 뷰박스 비율로 따라간다. */
  size?: number;
  className?: string;
  /** 장식이면 비워 둔다 — 읽어 줄 것이 없으면 스크린리더가 건너뛴다. */
  title?: string;
}) {
  const cfg = STORES[store] ?? STORES.claw;

  // 22~32px 자리에서는 소품이 뭉개진다. 소품 없는 파일로 내린다.
  const effective: MascotMode = mode === "main" && size < 48 ? "full" : mode;

  const src = `/characters/${cfg.character}/${fileName(cfg.character, mood, effective)}.svg`;

  return (
    // eslint-disable-next-line @next/next/no-img-element -- 정적 SVG라 최적화할 것이 없다
    <img
      src={src}
      width={size}
      height={Math.round(size * RATIO[effective])}
      alt={title ?? ""}
      aria-hidden={title ? undefined : true}
      className={className}
      draggable={false}
    />
  );
}
