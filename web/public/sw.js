/**
 * 사장님 대시보드용 서비스 워커.
 *
 * 하는 일은 푸시 수신과 클릭 처리 **뿐**이다. 캐싱은 하지 않는다 —
 * 민원 목록은 30초마다 새로 읽는 화면이고, 서비스 워커가 응답을 캐시하면
 * 사장님이 처리한 건이 계속 목록에 남는다. 돈이 오가는 화면에서 그건
 * 캐시 때문이라고 짐작하기 어려운 버그가 된다.
 *
 * 손님 폼은 대상이 아니다. QR 로 한 번 들어왔다 나가는 화면이다.
 */

self.addEventListener("install", () => {
  // 새 워커를 바로 올린다. 안 그러면 탭을 전부 닫을 때까지 옛 워커가 산다.
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  // 본문이 없거나 JSON 이 아닐 수 있다. 그래도 알림은 띄운다 —
  // 조용히 넘기면 사장님은 민원이 온 줄 모른다.
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = {};
  }

  const title = data.title || "새 민원";
  event.waitUntil(
    self.registration.showNotification(title, {
      body: data.body || "대시보드에서 확인해주세요.",
      icon: "/icons/icon-192.png",
      badge: "/icons/icon-192.png",
      // 같은 건으로 여러 번 오면 덮어쓴다. 알림이 쌓이면 사장님은
      // 알림을 통째로 끈다.
      tag: data.tag || "claw-hub",
      renotify: true,
      data: { url: data.url || "/admin/claims" },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = (event.notification.data && event.notification.data.url) || "/admin/claims";

  event.waitUntil(
    (async () => {
      const all = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      // 이미 열린 탭이 있으면 그 탭을 쓴다. 누를 때마다 새 탭이 열리면
      // 사장님 폰에 탭이 수십 개 쌓인다.
      for (const c of all) {
        if ("focus" in c) {
          await c.focus();
          if ("navigate" in c) await c.navigate(url);
          return;
        }
      }
      await self.clients.openWindow(url);
    })(),
  );
});
