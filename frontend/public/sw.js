// İPKS service worker — PWA v2 (Plan Faz 10 cilası; Faz 6 temeli).
//
// Görev sınırı bilinçli olarak dar: UYGULAMA KABUĞUNU çevrimdışı erişilebilir
// tutmak. API istekleri ASLA cache'lenmez — veri tazeliği ve yetki kontrolü
// her zaman sunucudadır; yazma işlemlerinin çevrimdışı kuyruğu uygulama
// katmanındadır (src/offline/queue.ts). Background Sync API bilinçli
// kullanılmadı: iOS Safari desteği yok, saha cihazlarının önemli bölümü iOS.
//
// Faz 10 eklemeleri:
//   - Sürümlü cache + eski sürümlerin activate'te temizliği (zaten vardı, korunur).
//   - Çevrimdışı yedek sayfası (/offline.html): kabuk da yoksa anlamlı ekran.
//   - Statik varlıklarda stale-while-revalidate; başarısız fetch'te cache'e düşüş.
//   - message → SKIP_WAITING: yeni sürüm anında devralabilsin.

const VERSION = "v2";
const SHELL_CACHE = `ipks-shell-${VERSION}`;
const OFFLINE_URL = "/offline.html";
const PRECACHE = ["/", "/offline.html", "/manifest.webmanifest"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((c) => c.addAll(PRECACHE)).catch(() => {})
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((k) => k !== SHELL_CACHE).map((k) => caches.delete(k)))
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("message", (event) => {
  if (event.data === "SKIP_WAITING") self.skipWaiting();
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return; // yazmalar uygulama kuyruğuna
  const url = new URL(req.url);

  // API ve sağlık uçları: her zaman ağ; hata uygulamaya düşer (offline kuyruk).
  if (
    url.pathname.startsWith("/api/") ||
    url.pathname === "/healthz" ||
    url.pathname === "/readyz"
  ) {
    return;
  }

  // Navigasyon: network-first → kabuk → çevrimdışı yedek sayfası.
  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req)
        .then((res) => {
          const copy = res.clone();
          caches.open(SHELL_CACHE).then((c) => c.put("/", copy)).catch(() => {});
          return res;
        })
        .catch(async () => (await caches.match("/")) || (await caches.match(OFFLINE_URL)))
    );
    return;
  }

  // Statik varlıklar: stale-while-revalidate.
  if (["script", "style", "font", "image"].includes(req.destination)) {
    event.respondWith(
      caches.match(req).then((hit) => {
        const network = fetch(req)
          .then((res) => {
            const copy = res.clone();
            caches.open(SHELL_CACHE).then((c) => c.put(req, copy)).catch(() => {});
            return res;
          })
          .catch(() => hit);
        return hit || network;
      })
    );
  }
});
