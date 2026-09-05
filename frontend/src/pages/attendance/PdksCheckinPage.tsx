import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api } from "../../api/client";

// PdksCheckinPage — rota /pdks. KİMLİK DOĞRULAMASI GEREKTİRMEZ: işçi bu
// sayfaya AppShell/oturum olmadan, yalnızca panodaki QR'ı okutarak ulaşır.
// Güvenlik oturumdan değil, URL'deki 60 saniyelik/tek kullanımlık token'dan
// gelir (sunucu tarafında doğrulanır). Konum yalnızca bu tek kayıt anında,
// tek atış olarak alınır — sürekli izleme yok. Tarayıcı sahte konumu
// (mock location) tespit edemez; bu yüzden burada ya da başka hiçbir yerde
// "GPS ile kesin doğrulama" gibi bir iddia YOKTUR — kayıt konum/zaman/cihaz
// kimliğiyle saklanır ve şantiye şefi onayına sunulur.

type PersonnelOption = { id: string; ad_soyad: string; gorev: string };

type QueuedEvent = {
  client_uuid: string;
  qr_token: string;
  person_id: string;
  event_type: "in" | "out";
  captured_at: string;
  lat?: number;
  lng?: number;
  accuracy_m?: number;
  device_id?: string;
};

type EventResult = {
  client_uuid: string;
  ok: boolean;
  duplicate?: boolean;
  error?: string;
  geofence_ok?: boolean;
  distance_m?: number;
};

const QUEUE_KEY = "ipks.pdks.queue";
const DEVICE_KEY = "ipks.pdks.device_id";

function loadQueue(): QueuedEvent[] {
  try {
    return JSON.parse(localStorage.getItem(QUEUE_KEY) ?? "[]");
  } catch {
    return [];
  }
}
function saveQueue(q: QueuedEvent[]) {
  localStorage.setItem(QUEUE_KEY, JSON.stringify(q));
}
function deviceId(): string {
  let id = localStorage.getItem(DEVICE_KEY);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(DEVICE_KEY, id);
  }
  return id;
}

// getPosition — TEK ATIŞ konum okuması (enableHighAccuracy, 15sn zaman
// aşımı). watchPosition KULLANILMAZ — arka plan/sürekli izleme yok.
// İzin verilmezse/başarısızsa null döner; kayıt yine de gönderilir
// (konumsuz), sunucu bunu geofence_ok=false olarak işaretler — reddetmez.
function getPosition(): Promise<GeolocationPosition | null> {
  return new Promise((resolve) => {
    if (!navigator.geolocation) {
      resolve(null);
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve(pos),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 15000 }
    );
  });
}

export default function PdksCheckinPage() {
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";

  const [personnel, setPersonnel] = useState<PersonnelOption[]>([]);
  const [personId, setPersonId] = useState("");
  const [eventType, setEventType] = useState<"in" | "out">("in");
  const [loadingList, setLoadingList] = useState(true);
  const [listError, setListError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; warn?: boolean; message: string } | null>(null);
  const [pendingCount, setPendingCount] = useState(() => loadQueue().length);

  useEffect(() => {
    if (!token) {
      setListError("Kod eksik. Panodaki QR'ı tekrar okutun.");
      setLoadingList(false);
      return;
    }
    api<{ personnel: PersonnelOption[] }>(`/attendance/personnel?token=${encodeURIComponent(token)}`)
      .then((res) => setPersonnel(res.personnel))
      .catch(() => setListError("Kod geçersiz veya süresi dolmuş. Panodaki QR'ı tekrar okutun."))
      .finally(() => setLoadingList(false));
  }, [token]);

  // Uçak modu / bağlantısızlık senaryosu: bağlantı geri gelince (ya da
  // sayfa yeniden açılınca) kuyruğa alınmış kayıtları toplu gönder.
  useEffect(() => {
    flushQueue();
    window.addEventListener("online", flushQueue);
    return () => window.removeEventListener("online", flushQueue);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function flushQueue() {
    const q = loadQueue();
    if (q.length === 0) return;
    try {
      const res = await api<{ results: EventResult[] }>("/attendance/events", { method: "POST", body: q });
      const stillNeedsToken = new Set(
        res.results.filter((r) => r.error === "gecersiz_kod" || r.error === "kod_suresi_gecersiz").map((r) => r.client_uuid)
      );
      // Başarılı (ok/duplicate) ya da kalıcı biçimde geçersiz (personel bulunamadı
      // vb.) kayıtlar kuyruktan düşer — yalnızca "token süresi/geçerliliği"
      // yüzünden başarısız olanlar kuyrukta kalır (yeni bir kod okutulunca tekrar denenir).
      const remaining = q.filter((e) => stillNeedsToken.has(e.client_uuid));
      saveQueue(remaining);
      setPendingCount(remaining.length);
    } catch {
      // Ağ hatası — kuyrukta bırak, sonraki 'online'/mount'ta tekrar denenir.
    }
  }

  async function submit() {
    if (!personId) {
      setResult({ ok: false, message: "Lütfen adınızı seçin." });
      return;
    }
    setSubmitting(true);
    setResult(null);

    const pos = await getPosition();
    const ev: QueuedEvent = {
      client_uuid: crypto.randomUUID(),
      qr_token: token,
      person_id: personId,
      event_type: eventType,
      captured_at: new Date().toISOString(),
      lat: pos?.coords.latitude,
      lng: pos?.coords.longitude,
      accuracy_m: pos?.coords.accuracy,
      device_id: deviceId(),
    };

    const queued = loadQueue();
    queued.push(ev);
    saveQueue(queued);
    setPendingCount(queued.length);

    try {
      const res = await api<{ results: EventResult[] }>("/attendance/events", { method: "POST", body: [ev] });
      const after = loadQueue().filter((e) => e.client_uuid !== ev.client_uuid);
      saveQueue(after);
      setPendingCount(after.length);

      const r = res.results[0];
      if (!r || !r.ok) {
        setResult({ ok: false, message: "Kayıt alınamadı. Panodaki QR'ı tekrar okutup deneyin." });
      } else if (r.geofence_ok === false) {
        setResult({
          ok: true, warn: true,
          message: "Kaydınız alındı. Şantiye sınırı dışında görünüyorsunuz — şefinizin onayına sunulacak.",
        });
      } else {
        setResult({ ok: true, message: eventType === "in" ? "Giriş kaydınız alındı." : "Çıkış kaydınız alındı." });
      }
    } catch {
      // Ağ yok — kayıt kuyrukta kaldı, bağlantı gelince otomatik gönderilecek.
      setResult({ ok: true, warn: true, message: "Bağlantı yok. Kaydınız cihazda saklandı, bağlantı gelince otomatik gönderilecek." });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-6" style={{ background: "#0d0f14" }}>
      <div className="w-full max-w-sm rounded-2xl border border-beton-800 bg-beton-900 p-7">
        <div className="text-center mb-5">
          <p className="text-[10px] font-bold uppercase tracking-[.15em]" style={{ color: "var(--group-accent)" }}>PDKS</p>
          <h1 className="font-display text-xl font-extrabold text-beton-100 mt-1">Giriş / Çıkış Kaydı</h1>
          {pendingCount > 0 && (
            <p className="mt-2 text-[11px] text-amber-400">{pendingCount} kayıt gönderilmeyi bekliyor…</p>
          )}
        </div>

        {loadingList ? (
          <p className="text-center text-beton-400 text-sm py-6">Yükleniyor…</p>
        ) : listError ? (
          <p className="text-center text-red-400 text-sm py-6">{listError}</p>
        ) : (
          <>
            <label className="block text-xs text-beton-400 mb-1">Adınız</label>
            <select
              value={personId}
              onChange={(e) => setPersonId(e.target.value)}
              className="w-full rounded-lg bg-beton-950 border border-beton-800 px-3 py-2.5 text-beton-100 outline-none focus:border-emniyet-500"
            >
              <option value="">— Seçin —</option>
              {personnel.map((p) => (
                <option key={p.id} value={p.id}>{p.ad_soyad} — {p.gorev}</option>
              ))}
            </select>

            <div className="mt-4 grid grid-cols-2 gap-2">
              <button
                onClick={() => setEventType("in")}
                className={`rounded-lg py-3 text-sm font-semibold transition ${
                  eventType === "in" ? "bg-emniyet-500 text-beton-950" : "bg-beton-950 border border-beton-800 text-beton-300"
                }`}
              >
                Giriş
              </button>
              <button
                onClick={() => setEventType("out")}
                className={`rounded-lg py-3 text-sm font-semibold transition ${
                  eventType === "out" ? "bg-emniyet-500 text-beton-950" : "bg-beton-950 border border-beton-800 text-beton-300"
                }`}
              >
                Çıkış
              </button>
            </div>

            <button
              onClick={submit}
              disabled={submitting || !personId}
              className="mt-5 w-full rounded-lg bg-[var(--group-accent)] hover:brightness-110 disabled:opacity-60 text-white-solid font-semibold py-2.5 transition"
            >
              {submitting ? "Kaydediliyor…" : "Kaydet"}
            </button>

            {result && (
              <p className={`mt-4 text-sm text-center ${result.ok ? (result.warn ? "text-amber-400" : "text-green-400") : "text-red-400"}`}>
                {result.message}
              </p>
            )}

            <p className="mt-5 text-[10.5px] text-beton-500 text-center leading-relaxed">
              Kaydınız konum, saat ve cihaz bilgisiyle saklanır ve şantiye şefi onayına sunulur.
            </p>
          </>
        )}
      </div>
    </main>
  );
}
