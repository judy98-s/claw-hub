/**
 * API 호출 래퍼.
 *
 * 같은 오리진(`/api/...`)으로만 부른다. next.config.ts 의 rewrites 가 Go
 * 서버로 넘긴다. 세션 쿠키가 SameSite=Lax 라 크로스 오리진으로 부르면
 * 쿠키가 실리지 않는다.
 */

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Json = Record<string, unknown> | unknown[];

async function parse(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, { credentials: "same-origin", ...init });
  } catch {
    // 네트워크 자체가 실패했다. 지하 매장에서 흔하다.
    throw new ApiError(0, "network", "인터넷 연결을 확인해주세요.");
  }

  const body = (await parse(res)) as { error?: string; message?: string } | null;

  if (!res.ok) {
    throw new ApiError(
      res.status,
      body?.error ?? "unknown",
      // 서버가 보낸 한국어 메시지를 그대로 쓴다. 클라이언트가 문구를
      // 따로 만들면 서버 검증과 어긋난다.
      body?.message ?? "문제가 생겼습니다. 잠시 후 다시 시도해주세요.",
    );
  }
  return body as T;
}

export function get<T>(path: string): Promise<T> {
  return request<T>(path);
}

/**
 * claimToken 은 Slack 링크로 들어온 사람의 접근 토큰이다.
 *
 * 헤더로 보낸다. 쿼리 문자열은 프록시 접근 로그와 Referer 헤더에 남기
 * 쉬운데, 이 토큰 하나로 손님 계좌가 열린다. 첫 진입(Slack 링크 클릭)은
 * 어쩔 수 없이 URL 이지만, 그 이후 호출까지 URL 에 실을 이유는 없다.
 */
function withToken(token?: string): HeadersInit | undefined {
  return token ? { "X-Claim-Token": token } : undefined;
}

export function getWith<T>(path: string, token?: string): Promise<T> {
  return request<T>(path, { headers: withToken(token) });
}

export function postWith<T>(path: string, body: Json, token?: string): Promise<T> {
  return request<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(withToken(token) ?? {}) },
    body: JSON.stringify(body),
  });
}

export function post<T>(path: string, body?: Json): Promise<T> {
  return request<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body ?? {}),
  });
}

export function patch<T>(path: string, body: Json): Promise<T> {
  return request<T>(path, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** postForm 은 multipart 접수 전용이다. */
export function postForm<T>(path: string, form: FormData, idempotencyKey: string): Promise<T> {
  return request<T>(path, {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: form,
  });
}
