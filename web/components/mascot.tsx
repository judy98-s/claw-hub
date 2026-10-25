/**
 * 뽑이 — claw-hub 마스코트.
 *
 * 왜 인형인가: 이 서비스에서 손님이 잃은 것은 돈이고, 못 받은 것은 인형이다.
 * 집게도 기계도 아니라 **집히지 못한 인형**이 주인공이다.
 *
 * 왜 이 톤인가: 손님은 이미 화가 나 있고, 사장님은 돈을 내보내는 화면을
 * 본다. 신나는 캐릭터는 양쪽 다 거스른다. 뽑이는 웃기지 않고 차분하게
 * 쳐다본다 — "봤어요, 처리할게요" 가 이 캐릭터가 할 말 전부다.
 *
 * 그려 놓고 실제로 렌더해서 고친 것들:
 *   - 처음엔 얼굴 면이 몸 전체를 덮어 '동물 머리' 로 보였다. 머리와 몸통을
 *     분리하고 팔다리를 붙여야 인형으로 읽힌다.
 *   - 라벨 태그가 결정적이다. 이게 없으면 그냥 곰이고, 있으면 봉제인형이다.
 *   - worried 의 눈썹을 두 번 틀렸다. SVG 는 y 가 아래로 증가해서, 안쪽
 *     끝의 y 를 **작게** 해야 올라간 눈썹(걱정)이 된다. 반대로 하면 화난
 *     얼굴이 되는데, 민원 화면에 화난 마스코트를 띄우는 건 최악이다.
 *
 * 구현 메모:
 *   - 그라데이션을 쓰지 않는다. globals.css 의 안티-디폴트와 같은 이유이고,
 *     단색 평면이라야 24px 로 줄여도 형태가 살아남는다.
 *   - currentColor 를 쓰지 않는다. 몸·얼굴·눈이 각각 달라야 하는데 하나로는
 *     표현이 안 되고, 상속으로 색이 바뀌면 눈이 사라지는 사고가 난다.
 *   - 귀 크기를 일부러 다르게 뒀다. 완전 대칭은 로고처럼 보인다.
 */

export type MascotMood = "calm" | "happy" | "worried" | "sleep";

const EYE = "var(--color-ink-900, #1a1917)";
const BODY = "var(--color-accent-500, #14897c)";
const DARK = "var(--color-accent-700, #0d5c54)";
const FACE = "var(--color-ink-50, #faf9f7)";
const METAL = "var(--color-ink-400, #a19a8e)";
const TAG = "var(--color-ink-200, #e5e2dc)";

const STROKE = { strokeWidth: 2.6, strokeLinecap: "round" as const, fill: "none", stroke: EYE };

export function Mascot({
  mood = "calm",
  size = 120,
  claw = false,
  className = "",
  title,
}: {
  mood?: MascotMood;
  size?: number;
  /** 집게를 머리 위에 씌운다. 히어로처럼 큰 자리에서만 쓴다. */
  claw?: boolean;
  className?: string;
  /** 장식이면 비워 둔다 — 읽어 줄 것이 없으면 스크린리더가 건너뛴다. */
  title?: string;
}) {
  return (
    <svg
      viewBox="0 0 120 150"
      width={size}
      height={(size * 150) / 120}
      className={className}
      role={title ? "img" : "presentation"}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      {claw && <Claw />}

      {/* 팔과 발은 몸보다 먼저 그려 뒤로 보낸다. */}
      <ellipse cx="27" cy="100" rx="9" ry="13" fill={DARK} transform="rotate(-14 27 100)" />
      <ellipse cx="93" cy="100" rx="9" ry="13" fill={DARK} transform="rotate(14 93 100)" />
      <ellipse cx="45" cy="134" rx="12" ry="8" fill={DARK} />
      <ellipse cx="75" cy="134" rx="12" ry="8" fill={DARK} />

      <path
        d="M60 76c-17 0-26 12-26 29 0 16 11 24 26 24s26-8 26-24c0-17-9-29-26-29Z"
        fill={BODY}
      />

      {/* 봉제인형 라벨. '동물'이 아니라 '인형'이라고 말해주는 디테일. */}
      <rect x="80" y="92" width="13" height="9" rx="2" fill={TAG} />
      <path d="M83 94.5h7M83 97.5h4" stroke={METAL} strokeWidth="1.4" strokeLinecap="round" />

      <circle cx="33" cy="33" r="12.5" fill={DARK} />
      <circle cx="87" cy="31" r="13.5" fill={DARK} />
      <circle cx="60" cy="48" r="30" fill={BODY} />
      <ellipse cx="60" cy="53" rx="21" ry="18" fill={FACE} />

      <Face mood={mood} />
    </svg>
  );
}

function Face({ mood }: { mood: MascotMood }) {
  if (mood === "sleep") {
    return (
      <>
        {/* 감은 눈은 선으로 그린다. 점을 작게 만들면 졸린 게 아니라 멀리 있어 보인다. */}
        <path d="M47 50q4.5 4.5 9 0" {...STROKE} />
        <path d="M64 50q4.5 4.5 9 0" {...STROKE} />
        <path d="M55.5 62q4.5 3 9 0" {...STROKE} />
      </>
    );
  }

  return (
    <>
      {mood === "worried" && (
        <>
          {/* 안쪽 끝을 올린다(y 를 작게). 내리면 화난 얼굴이 된다. */}
          <path d="M45 44.5l9 -4" {...STROKE} />
          <path d="M75 44.5l-9 -4" {...STROKE} />
        </>
      )}

      <circle cx="51" cy="51" r="4" fill={EYE} />
      <circle cx="69" cy="51" r="4" fill={EYE} />
      {/* 하이라이트가 없으면 눈이 구멍처럼 보인다. */}
      <circle cx="52.3" cy="49.7" r="1.35" fill={FACE} />
      <circle cx="70.3" cy="49.7" r="1.35" fill={FACE} />

      {mood === "happy" ? (
        <path d="M53 60q7 6.5 14 0" {...STROKE} />
      ) : mood === "worried" ? (
        <path d="M54 64q6 -4.5 12 0" {...STROKE} />
      ) : (
        <path d="M55.5 61q4.5 3 9 0" {...STROKE} />
      )}
    </>
  );
}

/** 집게. 인형을 집으려다 놓친 그 집게다. */
function Claw() {
  const s = { stroke: METAL, strokeLinecap: "round" as const, fill: "none" };
  return (
    <g>
      <path d="M60 0v10" {...s} strokeWidth="3.5" />
      <rect x="50" y="8" width="20" height="8" rx="4" fill={METAL} />
      {/*
        팔 끝은 x 24 / 96 까지 내보낸다. 머리가 cx60 r30 이라 x 30~90 을
        덮는데, 그 안에서 끝내면 팔이 머리 뒤에 완전히 가려 집게로 보이지
        않고 머리에 박힌 금속 조각처럼 보인다.
      */}
      <path d="M53 15q-22 12 -29 37" {...s} strokeWidth="4" />
      <path d="M67 15q22 12 29 37" {...s} strokeWidth="4" />
    </g>
  );
}
