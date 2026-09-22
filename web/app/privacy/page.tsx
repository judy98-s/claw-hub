import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "개인정보처리방침 · claw-hub",
  description: "claw-hub 개인정보 수집·이용·보관에 관한 안내",
};

/**
 * 개인정보처리방침.
 *
 * 플레이스토어 등록에 URL 이 필수이고, 이 시스템은 계좌번호·전화번호·사진을
 * 다루므로 형식이 아니라 실제로 필요한 문서다.
 *
 * 내용은 이 소프트웨어가 실제로 하는 일과 일치시켰다. 하지 않는 일을
 * 약속해두면 그게 더 큰 문제가 된다. 대괄호로 표시된 곳은 매장이 직접
 * 채워야 하는 값이다.
 */

const RETENTION_PHOTO_DAYS = 90;

export default function PrivacyPage() {
  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <article className="grid gap-8">
        <header className="grid gap-2">
          <h1 className="text-2xl font-bold">개인정보처리방침</h1>
          <p className="text-sm text-[var(--muted)]">최종 개정일: 2026년 9월 22일</p>
        </header>

        <Callout>
          이 문서의 <b>대괄호 [ ] 부분은 서비스를 운영하는 매장이 직접 채워야</b>{" "}
          합니다. 채우지 않은 채로 공개하면 안 됩니다.
        </Callout>

        <Section title="1. 처리하는 개인정보와 목적">
          <p>
            claw-hub(이하 &ldquo;서비스&rdquo;)는 인형뽑기 기계의 고장·환불 문의를 접수하고
            처리하기 위해 아래 정보를 수집합니다.
          </p>
          <Table
            head={["항목", "수집 목적", "필수 여부"]}
            rows={[
              ["휴대폰 번호", "환불 확인 연락, 반복 신고 판단", "필수"],
              ["은행명·계좌번호·예금주명", "환불금 송금", "필수"],
              ["기계 번호, 증상, 금액, 설명", "고장 확인 및 처리", "필수"],
              ["증상 사진", "고장 사실 확인", `${"10,000"}원 이상 요청 시 필수, 그 외 선택`],
              ["접속 IP 주소", "과도한 반복 제출 차단", "자동 수집"],
            ]}
          />
        </Section>

        <Section title="2. 보관 기간과 파기">
          <ul className="grid list-disc gap-2 pl-5">
            <li>
              <b>사진</b>은 업로드로부터 <b>{RETENTION_PHOTO_DAYS}일</b>이 지나면 자동으로
              삭제됩니다.
            </li>
            <li>
              <b>신고 기록(전화번호·계좌번호 포함)</b>은 환불 분쟁 대응과 반복 신고 판단을
              위해 <b>[보관 기간: 예) 3년]</b> 동안 보관한 뒤 파기합니다.
            </li>
            <li>
              이용자가 삭제를 요청하면 관련 법령상 보존 의무가 있는 경우를 제외하고 지체 없이
              파기합니다.
            </li>
          </ul>
        </Section>

        <Section title="3. 안전성 확보 조치">
          <ul className="grid list-disc gap-2 pl-5">
            <li>
              계좌번호·전화번호·예금주명은 <b>암호화(AES-256-GCM)해서 저장</b>합니다.
              데이터베이스에 평문으로 남지 않습니다.
            </li>
            <li>
              반복 신고를 판단할 때는 원본 대신 <b>복원할 수 없는 값(HMAC-SHA256)</b>으로
              대조합니다.
            </li>
            <li>
              계좌·연락처를 열람한 기록은 <b>누가 언제 열었는지 별도로 남깁니다.</b>
            </li>
            <li>관리자 화면은 로그인한 사람만 접근할 수 있습니다.</li>
          </ul>
        </Section>

        <Section title="4. 제3자 제공과 처리 위탁">
          <p>
            서비스는 수집한 개인정보를 제3자에게 제공하지 않습니다. 다만 아래 업무를 위해
            외부 서비스를 이용합니다.
          </p>
          <Table
            head={["대상", "위탁 내용", "전달되는 정보"]}
            rows={[
              [
                "Slack Technologies",
                "매장 담당자에게 신규 접수 알림 발송",
                "기계 번호, 증상, 금액 (전화번호·계좌번호는 전달하지 않음)",
              ],
              ["[서버 호스팅 사업자명]", "서버 운영 및 데이터 보관", "수집한 정보 전체"],
            ]}
          />
        </Section>

        <Section title="5. 이용자의 권리">
          <p>
            이용자는 자신의 개인정보에 대해 열람·정정·삭제·처리정지를 요구할 수 있습니다.
            아래 연락처로 요청하시면 확인 후 처리합니다.
          </p>
        </Section>

        <Section title="6. 문의처">
          <div className="grid gap-1 rounded-lg bg-[var(--surface-sunken)] p-4 text-sm">
            <p>매장명: [매장 이름]</p>
            <p>담당자: [담당자 이름]</p>
            <p>연락처: [전화번호]</p>
            <p>이메일: [이메일 주소]</p>
          </div>
          <p className="text-sm text-[var(--muted)]">
            개인정보 침해에 대한 신고나 상담이 필요하시면 개인정보침해신고센터(국번 없이
            118), 대검찰청 사이버수사과(1301), 경찰청 사이버수사국(182)으로 문의하실 수
            있습니다.
          </p>
        </Section>
      </article>
    </main>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-3">
      <h2 className="text-lg font-bold">{title}</h2>
      <div className="grid gap-3 text-[15px] leading-relaxed">{children}</div>
    </section>
  );
}

function Callout({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-lg bg-[var(--tone-warn-bg)] p-4 text-sm text-[var(--tone-warn-fg)]">
      {children}
    </p>
  );
}

function Table({ head, rows }: { head: string[]; rows: string[][] }) {
  return (
    <div className="-mx-4 overflow-x-auto px-4">
      <table className="w-full min-w-md border-collapse text-sm">
        <thead>
          <tr className="border-b border-[var(--line)]">
            {head.map((h) => (
              <th key={h} className="py-2 pr-4 text-left font-semibold">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="border-b border-[var(--line)]">
              {r.map((c, j) => (
                <td key={j} className="py-2 pr-4 align-top text-[var(--muted)]">
                  {c}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
