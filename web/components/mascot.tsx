/**
 * 뽑이 — claw-hub 마스코트.
 *
 * 왜 인형인가: 손님이 잃은 것은 돈이고 못 받은 것은 인형이다. 집게도
 * 기계도 아니라 **집히지 못한 인형**이 주인공이다.
 *
 * 왜 토끼인가: 처음 두 번은 곰으로 그렸는데 매번 "흔한 캐릭터" 로
 * 끝났다. 둥근 머리에 둥근 귀 두 개는 세상에 수천 개 있어서 기억에
 * 남지 않는다. 긴 귀는 실루엣만으로 구분되고, 무엇보다 **귀가 감정을
 * 말한다** — 얼굴이 뭉개지는 22px 에서도 처진 귀는 읽힌다.
 *
 * 왜 요소를 줄였나: 세련됨은 더하는 게 아니라 빼는 것이었다. 직전
 * 버전은 외곽선·귀 안쪽 분홍·볼터치·눈 하이라이트·태그가 전부 시선을
 * 나눠 가져서 정리가 안 된 인상을 줬다. 지금은 외곽선 / 눈 / 입 /
 * 볼터치 / 태그 다섯 가지뿐이다.
 *
 * 비율 규칙 (요즘 통하는 캐릭터들의 공통 문법):
 *   머리가 전체의 60~65%, 눈은 멀고(x 47/73) 아래로(y 64),
 *   크림 바탕 + 포인트색만, 완벽한 원이 아니라 살짝 눌린 곡선.
 *
 * 특정 IP 를 베끼지 않는다. 인기 캐릭터를 따라 그리면 저작권 분쟁이
 * 되고 이건 상용 서비스에 들어간다. 위 **문법**만 가져왔다.
 *
 * 구현 메모:
 *   - 그라데이션 없음. 22px 로 줄여도 형태가 살아남아야 한다.
 *   - 집게를 켜면 viewBox 를 위로 넓힌다. 머리가 x 26~94 를 덮어서
 *     팔이 그 안에서 끝나면 가려져 머리에 박힌 나사처럼 보인다.
 */

export type MascotMood = "calm" | "happy" | "worried" | "sleep";

/*
  선을 2.2 에서 1.7 로 줄였다. 굵은 선은 '귀여움' 쪽으로, 가는 선은
  '세련됨' 쪽으로 간다. 다만 1.4 아래로 내리면 22px 에서 외곽선이 사라져
  형체가 뭉개진다 — 세 굵기를 실제로 렌더해 보고 1.7 에서 멈췄다.

  색은 아주 연한 핑크 대신 **실제로 분홍으로 읽히는** 톤을 골랐다. 더
  연하면 흰색에 분홍 기미가 도는 정도라 작은 자리에서 흰 덩어리가 된다.

  로즈는 globals.css 에서 '장식의 색' 으로 정의했다. 누를 수 있는 것에는
  쓰지 않는다 — 그건 액센트(청록)의 몫이다.
*/
const LINE = "#c4707a";
const EYE = "#4a3230";
const BODY = "#ffe9ec";
const EAR = "#ffc3cd";
const BLUSH = "#f58ca0";
const METAL = "#efe3e0";

const INK = { stroke: EYE, strokeWidth: 2.2, strokeLinecap: "round" as const, fill: "none" };

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
  const top = claw ? -34 : -2;
  const vh = 120 - top;

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

      <g stroke={LINE} strokeWidth="1.7" strokeLinejoin="round">
        <ellipse cx="50" cy="110" rx="7.5" ry="5" fill={BODY} />
        <ellipse cx="70" cy="110" rx="7.5" ry="5" fill={BODY} />
        {/* 몸은 머리에 거의 가려지는 작은 덩어리다. */}
        <path d="M60 84c-12 0-18 6-18 15 0 7 8 11 18 11s18-4 18-11c0-9-6-15-18-15Z" fill={BODY} />
        <Ears mood={mood} />
        {/* 머리. 완벽한 원이 아니라 살짝 눌린 실루엣이라야 손그림으로 읽힌다. */}
        <path d="M60 22C38 22 26 36 26 55c0 20 14 33 34 33s34-13 34-33C94 36 82 22 60 22Z" fill={BODY} />
      </g>

      <InnerEars mood={mood} />

      <ellipse cx="36" cy="71" rx="6" ry="3.8" fill={BLUSH} opacity="0.75" />
      <ellipse cx="84" cy="71" rx="6" ry="3.8" fill={BLUSH} opacity="0.75" />

      <Face mood={mood} />

      {/* 라벨. 옆 솔기에서 삐져나온 천 조각이다 — 몸에 붙인 사각형은
          스티커로 보이고, 이 태그가 '동물' 과 '봉제인형' 을 가른다. */}
      <path
        d="M78 96l7 -2.2v8l-7 -2.2z"
        fill={BODY}
        stroke={LINE}
        strokeWidth="1.7"
        strokeLinejoin="round"
      />
      <path d="M80.5 96.5h2.5" stroke={BLUSH} strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

/**
 * 귀가 감정을 말한다.
 *
 * 얼굴보다 실루엣이 먼저 읽히기 때문에, 작게 줄였을 때 표정은 뭉개져도
 * 귀의 각도는 남는다. 기쁘면 곧게 서고, 걱정·졸림이면 옆으로 처진다.
 */
function Ears({ mood }: { mood: MascotMood }) {
  if (mood === "happy") {
    return (
      <>
        <path d="M46 26q-5 -26 2 -24 7 2 7 24" fill={BODY} />
        <path d="M74 26q5 -26 -2 -24 -7 2 -7 24" fill={BODY} />
      </>
    );
  }
  if (mood === "worried" || mood === "sleep") {
    return (
      <>
        <path d="M44 30q-16 -14 -10 -19 6 -5 16 13" fill={BODY} />
        <path d="M76 30q16 -14 10 -19 -6 -5 -16 13" fill={BODY} />
      </>
    );
  }
  return (
    <>
      <path d="M46 28q-7 -22 0 -22 7 0 8 21" fill={BODY} />
      <path d="M74 28q7 -22 0 -22 -7 0 -8 21" fill={BODY} />
    </>
  );
}

/**
 * 귀 안쪽. 처진 귀(worried·sleep)에는 넣지 않는다 — 접힌 귀의 안쪽은
 * 보이지 않는 게 맞고, 넣으면 귀가 접혔다는 사실이 흐려진다.
 */
function InnerEars({ mood }: { mood: MascotMood }) {
  if (mood === "worried" || mood === "sleep") return null;
  if (mood === "happy") {
    return (
      <>
        <path d="M48 24q-2.5 -17 1 -16 3.5 1 3.5 16Z" fill={EAR} />
        <path d="M72 24q2.5 -17 -1 -16 -3.5 1 -3.5 16Z" fill={EAR} />
      </>
    );
  }
  return (
    <>
      <path d="M48 26q-3.5 -15 0 -15 3.5 0 4 14Z" fill={EAR} />
      <path d="M72 26q3.5 -15 0 -15 -3.5 0 -4 14Z" fill={EAR} />
    </>
  );
}

/** 눈은 x 47 / 73 으로 멀고, y 64 로 낮다. 가깝고 높으면 성인 얼굴이 된다. */
function Face({ mood }: { mood: MascotMood }) {
  if (mood === "sleep") {
    return (
      <>
        <path d="M42 64q5 4.5 10 0" {...INK} strokeWidth="2.8" />
        <path d="M68 64q5 4.5 10 0" {...INK} strokeWidth="2.8" />
      </>
    );
  }

  if (mood === "happy") {
    return (
      <>
        <path d="M42 66q5 -6.5 10 0" {...INK} strokeWidth="3" />
        <path d="M68 66q5 -6.5 10 0" {...INK} strokeWidth="3" />
        <path d="M56 73q4 3.5 8 0" {...INK} />
      </>
    );
  }

  return (
    <>
      {mood === "worried" && (
        <>
          {/* 안쪽 끝을 올린다(y 를 작게). 내리면 화난 얼굴이 된다. */}
          <path d="M40 55l7 -2.5" {...INK} />
          <path d="M80 55l-7 -2.5" {...INK} />
        </>
      )}
      <ellipse cx="47" cy="64" rx="4" ry="5.2" fill={EYE} />
      <ellipse cx="73" cy="64" rx="4" ry="5.2" fill={EYE} />
      {mood === "worried" ? (
        <path d="M57 75q3 -3 6 0" {...INK} />
      ) : (
        <path d="M57 73.5q3 2.5 6 0" {...INK} />
      )}
    </>
  );
}

/** 집게. 인형을 집으려다 놓친 그 집게다. */
function Claw() {
  const s = {
    stroke: LINE,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    fill: "none",
  };
  return (
    <g>
      <path d="M60 -30v8" {...s} strokeWidth="2.3" />
      <rect x="51" y="-23" width="18" height="7" rx="3.5" fill={METAL} stroke={LINE} strokeWidth="1.7" />
      <path d="M55 -15q-28 13 -37 34" {...s} strokeWidth="2.5" />
      <path d="M65 -15q28 13 37 34" {...s} strokeWidth="2.5" />
    </g>
  );
}
