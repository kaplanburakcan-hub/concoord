// PWA offline kuyruk v1 (Plan Faz 6).
//
// Kapsam: GÜNLÜK RAPOR yazma işlemleri (oluştur / taslak güncelle / gönder).
// Cihaz çevrimdışıyken istekler localStorage'da sıraya alınır; bağlantı
// dönünce (online olayı, uygulama açılışı, ya da elle "senkronize et")
// SIRAYLA sunucuya oynatılır. Sıra korunur: bir öğe başarısız olursa
// arkasındakiler beklemede kalır (create → update → submit bağımlılığı).
//
// v1 sınırları (bilinçli sadelik; Faz 8 İSG kuyruğu bu modülü genişletir):
//  - Yalnızca JSON gövdeli istekler (fotoğraf ekleri Faz 8 kapsamında).
//  - Çakışma çözümü yok: sunucu 409 dönerse öğe "conflict" işaretlenir ve
//    kullanıcıya gösterilir; kuyruk kalanlarla devam etmez, kullanıcı karar verir.
//  - Kuyruk cihaz-yereldir; oturum düşerse token yenilendikten sonra oynatılır.
//
// Kabul kriteri karşılığı: "uçak modunda girilen rapor bağlantıda senkronize oluyor".

import { api, RequestError } from "../api/client";

export type QueuedRequest = {
  id: string;               // yerel benzersiz id
  createdAt: string;
  method: "POST" | "PUT";
  path: string;             // /projects/{pid}/daily-reports...
  projectId: string;
  body: unknown;
  label: string;            // kullanıcıya gösterilen açıklama
  status: "pending" | "conflict" | "failed";
  error?: string;
  // create edilen kaydın sunucu id'sine sonraki isteklerde ihtiyaç varsa,
  // path içinde {local:<id>} yer tutucusu kullanılır ve oynatma sırasında
  // gerçek id ile değiştirilir.
};

const KEY = "ipks.offline.queue.v1";
type Listener = (items: QueuedRequest[]) => void;
const listeners = new Set<Listener>();

function read(): QueuedRequest[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "[]") as QueuedRequest[];
  } catch {
    return [];
  }
}
function write(items: QueuedRequest[]) {
  localStorage.setItem(KEY, JSON.stringify(items));
  listeners.forEach((l) => l(items));
}

export function subscribe(l: Listener): () => void {
  listeners.add(l);
  l(read());
  return () => listeners.delete(l);
}

export function queued(): QueuedRequest[] {
  return read();
}

export function enqueue(req: Omit<QueuedRequest, "id" | "createdAt" | "status">): QueuedRequest {
  const item: QueuedRequest = {
    ...req,
    id: crypto.randomUUID(),
    createdAt: new Date().toISOString(),
    status: "pending",
  };
  write([...read(), item]);
  return item;
}

export function remove(id: string) {
  write(read().filter((q) => q.id !== id));
}

// apiWithOfflineFallback — çevrimiçi dene; ağ hatasında kuyruğa al.
// Dönen değer: {queued:true} ise istek yerelde bekliyor demektir.
export async function apiWithOfflineFallback<T>(
  req: Omit<QueuedRequest, "id" | "createdAt" | "status">
): Promise<{ queued: boolean; data?: T }> {
  if (!navigator.onLine) {
    enqueue(req);
    return { queued: true };
  }
  try {
    const data = await api<T>(req.path, {
      method: req.method,
      body: req.body,
      projectId: req.projectId,
    });
    return { queued: false, data };
  } catch (e) {
    // Yalnızca AĞ hatasında kuyruğa al (TypeError: fetch failed). Sunucunun
    // döndürdüğü 4xx/5xx iş hatasıdır; kuyruklamak veriyi maskeleyebilir.
    if (e instanceof RequestError) throw e;
    enqueue(req);
    return { queued: true };
  }
}

let syncing = false;

// sync — kuyruğu sırayla oynatır. create yanıtındaki id, sonraki öğelerin
// path'lerindeki {local:<localId>} yer tutucularına yazılır.
export async function sync(): Promise<{ done: number; remaining: number }> {
  if (syncing) return { done: 0, remaining: read().length };
  syncing = true;
  let done = 0;
  try {
    let items = read();
    const idMap: Record<string, string> = {};
    while (items.length > 0) {
      const item = items[0];
      if (item.status === "conflict" || item.status === "failed") break;
      if (!navigator.onLine) break;

      let path = item.path;
      for (const [localID, serverID] of Object.entries(idMap)) {
        path = path.replaceAll(`{local:${localID}}`, serverID);
      }
      if (path.includes("{local:")) {
        // Bağımlı olduğu create henüz oynatılamadı — sıradaki turda dener.
        break;
      }
      try {
        const res = await api<{ id?: string }>(path, {
          method: item.method,
          body: item.body,
          projectId: item.projectId,
        });
        if (item.method === "POST" && res && res.id) idMap[item.id] = res.id;
        items = items.slice(1);
        write(items);
        done++;
      } catch (e) {
        if (e instanceof RequestError && e.status === 409) {
          items[0] = { ...item, status: "conflict", error: e.message };
          write(items);
        } else if (e instanceof RequestError && e.status >= 400 && e.status < 500) {
          items[0] = { ...item, status: "failed", error: e.message };
          write(items);
        }
        // Ağ / 5xx: değişiklik yok, sonraki tetiklemede yeniden denenir.
        break;
      }
    }
  } finally {
    syncing = false;
  }
  return { done, remaining: read().length };
}

// initOfflineSync — uygulama açılışında çağrılır: bağlantı dönüşünde ve
// periyodik olarak kuyruğu oynatmayı dener.
export function initOfflineSync() {
  window.addEventListener("online", () => void sync());
  // Açılışta bekleyen varsa dene (oturum tazelendikten sonra da çalışır).
  window.setTimeout(() => void sync(), 3000);
  window.setInterval(() => {
    if (navigator.onLine && read().some((q) => q.status === "pending")) void sync();
  }, 60_000);
}
