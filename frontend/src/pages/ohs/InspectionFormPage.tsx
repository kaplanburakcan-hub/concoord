import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import { apiWithOfflineFallback } from "../../offline/queue";
import { useProjects } from "../ProjectContext";
import type { Template } from "./ChecklistTemplatesPage";

// Faz 8 — Mobil denetim ekranı. Uçak modunda doldurulan form offline kuyruğa
// (Faz 6 altyapısı) alınır; bağlantı dönünce senkronize olur. GPS opsiyoneldir
// (izin verilmezse yalnız metin lokasyonla devam edilir).

type Answer = "ok" | "fail" | "na";

export default function InspectionFormPage() {
  const { current } = useProjects();
  const nav = useNavigate();
  const pid = current?.id;

  const [templates, setTemplates] = useState<Template[]>([]);
  const [tmplID, setTmplID] = useState("");
  const [answers, setAnswers] = useState<Record<number, Answer>>({});
  const [notes, setNotes] = useState<Record<number, string>>({});
  const [location, setLocation] = useState("");
  const [gps, setGps] = useState<{ lat: number; lng: number } | null>(null);
  const [gpsMsg, setGpsMsg] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await api<{ templates: Template[] }>(
        `/ohs/checklist-templates?active=true`, { projectId: pid });
      setTemplates(r.templates);
      if (r.templates.length === 1) setTmplID(r.templates[0].id);
    } catch { setErr("Şablonlar yüklenemedi (çevrimdışıysanız daha önce açtığınız sayfadan devam edin)."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const tmpl = templates.find((t) => t.id === tmplID);

  function takeGps() {
    setGpsMsg("Konum alınıyor…");
    navigator.geolocation.getCurrentPosition(
      (p) => { setGps({ lat: p.coords.latitude, lng: p.coords.longitude }); setGpsMsg(null); },
      () => setGpsMsg("Konum alınamadı (izin verilmemiş olabilir)."),
      { enableHighAccuracy: true, timeout: 10_000 }
    );
  }

  const allAnswered = !!tmpl && tmpl.items.every((it) => answers[it.no]);

  async function submit() {
    if (!pid || !tmpl || !allAnswered) return;
    setBusy(true); setErr(null);
    const body = {
      template_id: tmpl.id,
      inspected_at: new Date().toISOString(), // offline'da cihaz saati korunur
      location_text: location.trim() || undefined,
      gps_lat: gps?.lat, gps_lng: gps?.lng,
      results: tmpl.items.map((it) => ({
        no: it.no, answer: answers[it.no], note: notes[it.no]?.trim() || undefined,
      })),
    };
    try {
      const res = await apiWithOfflineFallback({
        method: "POST", path: `/projects/${pid}/ohs/inspections`,
        projectId: pid, body, label: `İSG denetimi — ${tmpl.name}`,
      });
      if (res.queued) {
        alert("Bağlantı yok: denetim cihazda sıraya alındı; bağlantı dönünce gönderilecek.");
      }
      nav("/isg/denetimler");
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Denetim gönderilemedi.");
    } finally { setBusy(false); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4 max-w-xl">
      <div className="flex items-center gap-3">
        <Link to="/isg/denetimler" className="text-xs text-beton-400 hover:text-beton-200">← Denetimler</Link>
        <h1 className="text-lg font-display font-bold text-white">Yeni İSG Denetimi</h1>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
        <label className="block text-xs text-beton-300">
          Checklist şablonu
          <select value={tmplID} onChange={(e) => { setTmplID(e.target.value); setAnswers({}); setNotes({}); }}
            className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-2 text-sm text-beton-100">
            <option value="">Seçin…</option>
            {templates.map((t) => (
              <option key={t.id} value={t.id}>{t.category} — {t.name}</option>
            ))}
          </select>
        </label>
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex-1 min-w-[160px] text-xs text-beton-300">
            Lokasyon
            <input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="B blok 3. kat"
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-2 text-sm text-beton-100" />
          </label>
          <button onClick={takeGps}
            className="rounded-md border border-beton-700 px-3 py-2 text-xs text-beton-300 hover:bg-beton-800">
            {gps ? `GPS ✓ (${gps.lat.toFixed(4)}, ${gps.lng.toFixed(4)})` : "GPS al"}
          </button>
        </div>
        {gpsMsg && <p className="text-xs text-beton-400">{gpsMsg}</p>}
      </div>

      {tmpl && (
        <div className="space-y-2">
          {tmpl.items.map((it) => (
            <div key={it.no} className="rounded-lg border border-beton-800 bg-beton-900 p-3 space-y-2">
              <p className="text-sm text-beton-100">
                <span className="text-beton-500">{it.no}.</span> {it.text}
                {it.critical && (
                  <span className="ml-2 rounded border border-red-500/40 bg-red-500/10 px-1 py-0.5 text-xs text-red-300">
                    kritik
                  </span>
                )}
              </p>
              {/* Mobil: büyük dokunma hedefleri */}
              <div className="grid grid-cols-3 gap-2">
                {(["ok", "fail", "na"] as Answer[]).map((a) => (
                  <button key={a} onClick={() => setAnswers((x) => ({ ...x, [it.no]: a }))}
                    className={`rounded-md border py-2 text-xs font-semibold ${
                      answers[it.no] === a
                        ? a === "ok" ? "border-green-500 bg-green-500/20 text-green-300"
                          : a === "fail" ? "border-red-500 bg-red-500/20 text-red-300"
                          : "border-beton-500 bg-beton-700 text-beton-200"
                        : "border-beton-700 text-beton-400 hover:bg-beton-800"}`}>
                    {a === "ok" ? "Uygun" : a === "fail" ? "Uygunsuz" : "Uygulanamaz"}
                  </button>
                ))}
              </div>
              {answers[it.no] === "fail" && (
                <input value={notes[it.no] ?? ""} placeholder="Uygunsuzluk notu (bulguyu ayrıca kaydedin)"
                  onChange={(e) => setNotes((x) => ({ ...x, [it.no]: e.target.value }))}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-2 text-sm text-beton-100" />
              )}
            </div>
          ))}
          <button onClick={submit} disabled={!allAnswered || busy}
            className="w-full rounded-md bg-emniyet-500 py-2.5 text-sm font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
            {busy ? "Gönderiliyor…" : allAnswered ? "Denetimi Gönder" : "Tüm maddeleri yanıtlayın"}
          </button>
          <p className="text-xs text-beton-500">
            Uygunsuz maddeler için Bulgular ekranından foto + terminli bulgu kaydı açabilirsiniz.
          </p>
        </div>
      )}
    </div>
  );
}
