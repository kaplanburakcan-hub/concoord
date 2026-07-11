import { useEffect, useState } from "react";
import { QueuedRequest, remove, subscribe, sync } from "../../offline/queue";

// Çevrimdışı kuyruk durumu (Faz 6 — PWA offline kuyruk v1).
// Bekleyen kayıt varsa kullanıcıya gösterir; bağlantı varken elle senkron
// tetiklenebilir. conflict/failed öğeler açıklamayla listelenir ve
// kullanıcı kararıyla kuyruktan çıkarılabilir (veri sessizce kaybolmaz).

export default function OfflineQueueBanner({ onSynced }: { onSynced?: () => void }) {
  const [items, setItems] = useState<QueuedRequest[]>([]);
  const [busy, setBusy] = useState(false);
  const [online, setOnline] = useState(navigator.onLine);

  useEffect(() => subscribe(setItems), []);
  useEffect(() => {
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, []);

  if (items.length === 0 && online) return null;

  async function doSync() {
    setBusy(true);
    try {
      const res = await sync();
      if (res.done > 0) onSynced?.();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-lg border border-emniyet-500/40 bg-emniyet-500/10 p-3 text-sm space-y-2">
      {!online && (
        <p className="text-emniyet-500 font-medium">
          Çevrimdışısınız — girilen raporlar cihazda bekletilir ve bağlantıda gönderilir.
        </p>
      )}
      {items.length > 0 && (
        <>
          <div className="flex items-center justify-between gap-2 flex-wrap">
            <span className="text-beton-200">
              Bekleyen çevrimdışı kayıt: <strong>{items.length}</strong>
            </span>
            {online && (
              <button
                onClick={doSync}
                disabled={busy}
                className="rounded-md bg-emniyet-500 px-3 py-1 text-xs font-medium text-beton-950 disabled:opacity-50"
              >
                {busy ? "Senkronize ediliyor…" : "Şimdi senkronize et"}
              </button>
            )}
          </div>
          <ul className="space-y-1">
            {items.map((q) => (
              <li key={q.id} className="flex items-center justify-between gap-2 text-xs">
                <span className={q.status === "pending" ? "text-beton-300" : "text-red-400"}>
                  {q.label}
                  {q.status !== "pending" && (
                    <> — {q.status === "conflict" ? "çakışma" : "hata"}: {q.error || "sunucu reddetti"}</>
                  )}
                </span>
                {q.status !== "pending" && (
                  <button
                    onClick={() => remove(q.id)}
                    className="shrink-0 rounded border border-beton-700 px-2 py-0.5 text-beton-300 hover:border-red-400"
                    title="Bu kaydı kuyruktan çıkar (veri gönderilmeyecek)"
                  >
                    Vazgeç
                  </button>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
