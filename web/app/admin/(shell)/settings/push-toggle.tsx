"use client";

import { useCallback, useEffect, useState } from "react";
import { BellSimple, BellSimpleSlash } from "@phosphor-icons/react";

import { get, post } from "@/lib/api";
import { Button } from "@/components/ui/button";

/**
 * 웹 푸시 켜기/끄기.
 *
 * 사장님 전용이다. 손님은 앱을 설치하지 않는다 (docs/DESIGN-V2.md §8).
 *
 * iOS 는 **홈 화면에 추가한 뒤에만** 푸시를 받는다. 사파리 탭에서는
 * 권한 요청 자체가 실패한다. 그래서 그 경우를 따로 안내한다 — "알림이
 * 안 켜져요" 문의의 대부분이 이것이다.
 */

type KeyInfo = { publicKey: string; enabled: boolean };

/** VAPID 공개키는 base64url 문자열이다. 구독 API 는 바이트 배열을 받는다. */
function urlBase64ToArrayBuffer(base64: string): ArrayBuffer {
  const padded = (base64 + "=".repeat((4 - (base64.length % 4)) % 4))
    .replace(/-/g, "+")
    .replace(/_/g, "/");
  const raw = atob(padded);
  const buf = new ArrayBuffer(raw.length);
  const view = new Uint8Array(buf);
  for (let i = 0; i < raw.length; i++) view[i] = raw.charCodeAt(i);
  return buf;
}

export function PushToggle() {
  const [info, setInfo] = useState<KeyInfo | null>(null);
  const [subscribed, setSubscribed] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  const supported =
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window;

  // 홈 화면에서 연 앱인지. iOS 는 여기서만 푸시가 된다.
  const standalone =
    typeof window !== "undefined" &&
    (window.matchMedia("(display-mode: standalone)").matches ||
      // iOS 사파리만 쓰는 옛 속성.
      (navigator as unknown as { standalone?: boolean }).standalone === true);

  const isIOS =
    typeof navigator !== "undefined" && /iPad|iPhone|iPod/.test(navigator.userAgent);

  useEffect(() => {
    get<KeyInfo>("/api/admin/push/key")
      .then(setInfo)
      // 서버에 키가 없으면 버튼을 숨긴다. 눌러봤자 실패하는 버튼을
      // 보여주는 것보다 없는 게 낫다.
      .catch(() => setInfo({ publicKey: "", enabled: false }));
  }, []);

  useEffect(() => {
    if (!supported) return;
    navigator.serviceWorker
      .getRegistration()
      .then((reg) => reg?.pushManager.getSubscription())
      .then((sub) => setSubscribed(Boolean(sub)))
      .catch(() => setSubscribed(false));
  }, [supported]);

  const enable = useCallback(async () => {
    if (!info?.publicKey) return;
    setBusy(true);
    setMsg("");
    try {
      const reg = await navigator.serviceWorker.register("/sw.js");
      await navigator.serviceWorker.ready;

      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        // 브라우저 설정에서 풀어야 한다. 다시 물어도 안 뜬다.
        setMsg("브라우저가 알림을 막고 있습니다. 주소창 왼쪽 자물쇠에서 알림을 허용해주세요.");
        return;
      }

      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToArrayBuffer(info.publicKey),
      });
      // toJSON() 을 그대로 보낸다. 서버가 같은 모양을 받는다 — 필드를
      // 옮겨 담으면 그 과정에서 오타가 난다.
      await post("/api/admin/push/subscribe", JSON.parse(JSON.stringify(sub)));
      setSubscribed(true);
      setMsg("알림을 켰습니다. 새 민원이 오면 바로 알려드립니다.");
    } catch {
      setMsg("알림을 켜지 못했습니다. 잠시 후 다시 시도해주세요.");
    } finally {
      setBusy(false);
    }
  }, [info]);

  const disable = useCallback(async () => {
    setBusy(true);
    setMsg("");
    try {
      const reg = await navigator.serviceWorker.getRegistration();
      const sub = await reg?.pushManager.getSubscription();
      if (sub) {
        // 서버에서 먼저 지운다. 브라우저에서만 지우면 서버는 죽은
        // 구독으로 계속 발송을 시도한다.
        await post("/api/admin/push/unsubscribe", { endpoint: sub.endpoint });
        await sub.unsubscribe();
      }
      setSubscribed(false);
      setMsg("알림을 껐습니다.");
    } catch {
      setMsg("알림을 끄지 못했습니다.");
    } finally {
      setBusy(false);
    }
  }, []);

  // 서버에 키가 없으면 이 기능 자체가 없는 것이다.
  if (!info || !info.enabled) return null;

  if (!supported) {
    return (
      <p className="text-sm text-[var(--muted)]">
        이 브라우저는 알림을 지원하지 않습니다. 크롬이나 사파리 최신 버전에서 열어주세요.
      </p>
    );
  }

  if (isIOS && !standalone) {
    return (
      <p className="text-sm text-[var(--muted)]">
        아이폰은 <b>홈 화면에 추가</b>한 뒤에만 알림을 받을 수 있습니다. 공유 버튼 →
        &ldquo;홈 화면에 추가&rdquo; 를 누르고, 거기서 열어 다시 시도해주세요.
      </p>
    );
  }

  return (
    <div className="grid gap-2">
      {subscribed ? (
        <Button variant="secondary" onClick={disable} disabled={busy}>
          <BellSimpleSlash size={18} weight="regular" /> 알림 끄기
        </Button>
      ) : (
        <Button onClick={enable} disabled={busy || !info.publicKey}>
          <BellSimple size={18} weight="regular" /> 새 민원 알림 켜기
        </Button>
      )}
      {msg && <p className="text-sm text-[var(--muted)]">{msg}</p>}
    </div>
  );
}
