import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";

// Faz 4 — bildirim kanal tercihleri. Varsayılan: uygulama içi + e-posta açık,
// SMS kapalı (sağlayıcı anlaşması sonrası kullanıcı açar).

const CHANNELS: { key: string; label: string; note: string }[] = [
  { key: "InApp", label: "Uygulama içi", note: "Üst bardaki zil — anlık bildirimler." },
  { key: "Email", label: "E-posta", note: "Görev atama, @mention ve termin hatırlatmaları e-posta ile gelir." },
  { key: "SMS", label: "SMS", note: "Kısa mesaj (sağlayıcı yapılandırması gerektirir; telefon numaranız kayıtlı olmalıdır)." },
];

export default function NotificationPrefsPage() {
  const [prefs, setPrefs] = useState<Record<string, boolean>>({});
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const r = await api<{ preferences: Record<string, boolean> }>("/notification-preferences");
      setPrefs(r.preferences);
    } catch {
      setErr("Tercihler yüklenemedi.");
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function toggle(channel: string) {
    const next = !prefs[channel];
    setSaving(channel);
    setErr(null);
    try {
      await api("/notification-preferences", {
        method: "PUT",
        body: { channel, enabled: next },
      });
      setPrefs((p) => ({ ...p, [channel]: next }));
    } catch {
      setErr("Tercih kaydedilemedi.");
    } finally {
      setSaving(null);
    }
  }

  return (
    <div className="max-w-lg">
      <h1 className="font-display text-2xl font-extrabold text-white">Bildirim Ayarları</h1>
      <p className="text-sm text-beton-400 mt-1">Bildirimleri hangi kanallardan almak istediğinizi seçin.</p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-5 space-y-3">
        {CHANNELS.map((c) => (
          <div key={c.key} className="flex items-start justify-between gap-4 rounded-lg border border-beton-800 bg-beton-900 p-3">
            <div>
              <p className="text-sm font-semibold text-beton-100">{c.label}</p>
              <p className="mt-0.5 text-xs text-beton-400">{c.note}</p>
            </div>
            <button
              onClick={() => toggle(c.key)}
              disabled={saving === c.key}
              className={`mt-0.5 h-6 w-11 shrink-0 rounded-full border transition ${
                prefs[c.key] ? "border-emniyet-500 bg-emniyet-500/30" : "border-beton-700 bg-beton-950"
              }`}
              title={prefs[c.key] ? "Açık" : "Kapalı"}
            >
              <span
                className={`block h-4 w-4 rounded-full transition ${
                  prefs[c.key] ? "ml-6 bg-emniyet-500" : "ml-1 bg-beton-500"
                }`}
              />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
