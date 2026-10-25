/**
 * 뽑이 — claw-hub 마스코트.
 *
 * 왜 인형인가: 손님이 잃은 것은 돈이고 못 받은 것은 인형이다. 집게도
 * 기계도 아니라 **집히지 못한 인형**이 주인공이다.
 *
 * 왜 이 비율인가 — 처음 그린 버전이 빈티 났던 이유를 뜯어보니 전부
 * 비율과 색의 문제였다. 요즘 통하는 캐릭터들의 문법은 이렇다:
 *
 *   머리가 전체의 60~65%   (어른 비율 1:1 은 캐릭터가 아니라 사람으로 읽힌다)
 *   눈은 멀고 아래에        (가깝고 위면 성인 얼굴이 된다)
 *   크림/흰 바탕 + 포인트색  (채도 높은 단색은 기업 로고처럼 보인다)
 *   볼터치                  (이것만으로 호감도가 크게 바뀐다)
 *   살짝 불규칙한 실루엣     (완벽한 원은 로고고, 눌린 곡선이라야 손그림이다)
 *   부드러운 외곽선          (면만 있으면 밋밋하다)
 *
 * 특정 IP 를 베끼지 않는다. 인기 캐릭터를 따라 그리면 저작권 분쟁이 되고,
 * 이건 상용 서비스에 들어간다. 위 **문법**만 가져왔다.
 *
 * 표정은 네 가지지만 톤은 하나다. 손님은 화가 나 있고 사장님은 돈을
 * 내보내는 화면을 본다. 신나는 캐릭터는 양쪽 다 거스른다.
 *
 * 구현 메모:
 *   - 그라데이션을 쓰지 않는다. 24px 로 줄여도 형태가 살아남아야 한다.
 *   - 집게를 켜면 viewBox 를 위로 넓힌다. 머리가 x 23~97 을 덮으므로
 *     팔이 그 안에서 끝나면 통째로 가려져 머리에 박힌 나사처럼 보인다.
 */

export type MascotMood = "calm" | "happy" | "worried" | "sleep";

const LINE = "#4a4038";
const EYE = "#2b2522";
const CREAM = "#fffdf9";
const EAR = "#f6d9d2";
const BLUSH = "#f8b3ac";
const METAL = "#cdc8bf";
const TEAL = "var(--color-accent-500, #14897c)";

const INK = { stroke: EYE, strokeWidth: 2.4, strokeLinecap: "round" as const, fill: "none" };

export function Mascot({
  mood = "calm",
  size = 120,
  claw = false,
  className = "",
  title,
}: {
  mood?: MascotMood;
  size?: number;
  /** 집게를 머리 위에 띄운다. 히어로처럼 큰 자리에서만 쓴다. */
  claw?: boolean;
  className?: string;
  /** 장식이면 비워 둔다 — 읽어 줄 것이 없으면 스크린리더가 건너뛴다. */
  title?: string;
}) {
  const top = claw ? -26 : 0;
  const vh = 124 - top;

  return (
    <svg
      viewBox={`0 ${top} 120 ${vh}`}
      width={size}
      height={(size * vh) / 120}
      className={className}
      role={title ? "img" : "presentation"}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      {claw && <Claw />}

      <g stroke={LINE} strokeWidth="2.6" strokeLinejoin="round">
        <ellipse cx="49" cy="113" rx="8.5" ry="5.5" fill={CREAM} />
        <ellipse cx="71" cy="113" rx="8.5" ry="5.5" fill={CREAM} />
        {/* 몸은 머리에 거의 가려지는 작은 덩어리다. */}
        <path d="M60 82c-13 0-20 7-20 17 0 8 9 12 20 12s20-4 20-12c0-10-7-17-20-17Z" fill={CREAM} />
        <path d="M41 93q-8.5 1.5 -10.5 7.5" fill="none" strokeLinecap="round" strokeWidth="3.4" />
        <path d="M79 93q8.5 1.5 10.5 7.5" fill="none" strokeLinecap="round" strokeWidth="3.4" />
        <path d="M32 30q-4 -14 7 -16 9 -2 10 11Z" fill={CREAM} />
        <path d="M88 28q5 -14 -6 -16 -9 -2 -11 11Z" fill={CREAM} />
        {/* 머리. 완벽한 원이 아니라 살짝 눌린 실루엣이라야 손그림으로 읽힌다. */}
        <path d="M60 18C36 18 23 33 23 53c0 21 15 35 37 35s37-14 37-35C97 33 84 18 60 18Z" fill={CREAM} />
      </g>

      <path d="M36 27q-2 -8 4 -9 5 -1 6 6Z" fill={EAR} />
      <path d="M84 25q3 -8 -3 -9 -5 -1 -7 6Z" fill={EAR} />

      <ellipse cx="33" cy="69" rx="7" ry="4.4" fill={BLUSH} opacity="0.85" />
      <ellipse cx="87" cy="69" rx="7" ry="4.4" fill={BLUSH} opacity="0.85" />

      <Face mood={mood} />

      {/* 라벨. 옆 솔기에서 삐져나온 천 조각이다 — 몸에 붙인 사각형은
          스티커로 보이고, 이 태그가 '곰' 과 '봉제인형' 을 가른다. */}
      <path
        d="M79 95l8 -2.5v9l-8 -2.5z"
        fill={CREAM}
        stroke={LINE}
        strokeWidth="2.2"
        strokeLinejoin="round"
      />
      <path d="M82 95.5h3" stroke={TEAL} strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

/** 눈은 x 46 / 74 로 멀고, y 62 로 낮다. 가깝고 높으면 성인 얼굴이 된다. */
function Face({ mood }: { mood: MascotMood }) {
  if (mood === "sleep") {
    return (
      <>
        <path d="M41 62q5 5 10 0" {...INK} strokeWidth="3" />
        <path d="M69 62q5 5 10 0" {...INK} strokeWidth="3" />
        <path d="M57 73q3 2.5 6 0" {...INK} />
      </>
    );
  }

  if (mood === "happy") {
    return (
      <>
        <path d="M40.5 64q5.5 -7 11 0" {...INK} strokeWidth="3.2" />
        <path d="M68.5 64q5.5 -7 11 0" {...INK} strokeWidth="3.2" />
        <path d="M55 71q5 5 10 0" {...INK} />
      </>
    );
  }

  return (
    <>
      {mood === "worried" && (
        <>
          {/* 안쪽 끝을 올린다(y 를 작게). 내리면 화난 얼굴이 된다. */}
          <path d="M39 52.5l8 -3" {...INK} />
          <path d="M81 52.5l-8 -3" {...INK} />
        </>
      )}
      <ellipse cx="46" cy="62" rx="4.6" ry="5.4" fill={EYE} />
      <ellipse cx="74" cy="62" rx="4.6" ry="5.4" fill={EYE} />
      {/* 하이라이트가 없으면 눈이 구멍처럼 보인다. */}
      <circle cx="47.6" cy="60.2" r="1.5" fill="#fff" />
      <circle cx="75.6" cy="60.2" r="1.5" fill="#fff" />
      {mood === "worried" ? (
        <path d="M56 74q4 -3.5 8 0" {...INK} />
      ) : (
        <path d="M56 72q4 3 8 0" {...INK} />
      )}
    </>
  );
}

/** 집게. 인형을 집으려다 놓친 그 집게다. */
function Claw() {
  const s = { stroke: LINE, strokeLinecap: "round" as const, strokeLinejoin: "round" as const, fill: "none" };
  return (
    <g>
      <path d="M60 -22v8" {...s} strokeWidth="3.4" />
      <rect x="50" y="-15" width="20" height="8" rx="4" fill={METAL} stroke={LINE} strokeWidth="2.6" />
      <path d="M54 -7q-30 14 -40 36" {...s} strokeWidth="4" />
      <path d="M66 -7q30 14 40 36" {...s} strokeWidth="4" />
    </g>
  );
}
