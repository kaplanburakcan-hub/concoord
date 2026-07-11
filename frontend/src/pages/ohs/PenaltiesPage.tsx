import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiDownload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Faz 8 — İSG ceza tutanakları. Kesme isteği sunucuda anında PDF üretir ve
// bildirim yayar (kabul: 60 sn içinde). Para cezası, taşeronun sonraki taslak
// hakedişinde kesinti önerisi olarak otomatik görünür.

type Penalty = {
  id: string; penalty_no: string; subcontractor_id: string; subcontractor_name: string;
  finding_id?: string; violation_type: string; penalty_level: string; amount?: number;
  note?: string; issued_by_name: string; issued_at: string; status: string;
  applied_payment_id?: string; has_pdf: boolean; row_version: number; created_at: string;
};
type Sub = { id: string; company_name: string };

const LEVEL_LABEL: Record<string, string> = { Warning: "Uyarı", Fine: "Para Cezası" };
const ST_LABEL: Record<string, string> = {
  Issued: "Kesildi", Acknowledged: "Tebellüğ Edildi", AppliedToPayment: "Hakedişe Uygulandı",
};
const ST_STYLE: Record<string, string> = {
  Issued: "bg-red-500/15 text-red-300 border-red-500/40",
  Acknowledged: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  AppliedToPayment: "bg-green-500/15 text-green-300 border-green-500/40",
};

function fileToDataURL(f: File): Promise<string> {
  return new Promise((res, rej) => {
    const r = new FileReader();
    r.onload = () => res(r.result as string);
    r.onerror = () => rej(new Error("dosya okunamadı"));
    r.readAsDataURL(f);
  });
}

export default function PenaltiesPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canIssue = can("ohs.issue_penalty");

  const [list, setList] = useState<Penalty[]>([]);
  const [subs, setSubs] = useState<Sub[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  // form
  const [subID, setSubID] = useState("");
  const [violation, setViolation] = useState("");
  const [level, setLevel] = useState("Fine");
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ penalties: Penalty[] }>(`/projects/${pid}/ohs/penalties`, { projectId: pid });
      setList(r.penalties);
    } catch { setErr("Ceza tutanakları yüklenemedi ya da erişim yetkiniz yok."); }
    try {
      const s = await api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid });
      setSubs(s.subcontractors);
    } catch { /* taşeron temsilcisi listeyi göremeyebilir; form zaten kapalı */ }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function issue() {
    if (!pid || !subID || !violation.trim()) return;
    if (level === "Fine" && !(Number(amount) > 0)) { setErr("Para cezasında pozitif tutar zorunlu."); return; }
    setBusy(true); setErr(null); setMsg(null);
    try {
      let evidenceB64: string | undefined;
      let evidenceName: string | undefined;
      const f = fileRef.current?.files?.[0];
      if (f) {
        if (f.size > 8 * 1024 * 1024) { setErr("Kanıt fotoğrafı 8 MB sınırını aşıyor."); setBusy(false); return; }
        evidenceB64 = await fileToDataURL(f);
        evidenceName = f.name;
      }
      const r = await api<{ penalty_no: string }>(`/projects/${pid}/ohs/penalties`, {
        method: "POST", projectId: pid,
        body: {
          subcontractor_id: subID,
          violation_type: violation.trim(),
          penalty_level: level,
          amount: level === "Fine" ? Number(amount) : undefined,
          note: note.trim() || undefined,
          evidence_base64: evidenceB64, evidence_name: evidenceName,
        },
      });
      setMsg(`${r.penalty_no} kesildi — PDF hazır, bildirimler gönderildi.` +
        (level === "Fine" ? " Kesinti önerisi bir sonraki taslak hakedişte görünecek." : ""));
      setSubID(""); setViolation(""); setAmount(""); setNote(""); setShowForm(false);
      if (fileRef.current) fileRef.current.value = "";
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Tutanak kesilemedi.");
    } finally { setBusy(false); }
  }

  async function acknowledge(p: Penalty) {
    try {
      await api(`/projects/${pid}/ohs/penalties/${p.id}/acknowledge`, {
        method: "POST", projectId: pid, body: {},
      });
      load();
    } catch { setErr("Tebellüğ yapılamadı (yalnızca ilgili taşeron temsilcisi)."); }
  }

  async function downloadPdf(p: Penalty) {
    try {
      await apiDownload(`/projects/${pid}/ohs/penalties/${p.id}/pdf`, `${p.penalty_no}.pdf`);
    } catch { setErr("PDF indirilemedi."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-display font-bold text-white">İSG Ceza Tutanakları</h1>
        <Link to="/isg" className="text-xs text-beton-400 hover:text-beton-200">Bulgular</Link>
        <Link to="/isg/denetimler" className="text-xs text-beton-400 hover:text-beton-200">Denetimler</Link>
        {canIssue && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            Ceza Kes
          </button>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}
      {msg && <p className="text-sm text-emniyet-500">{msg}</p>}

      {showForm && canIssue && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              Taşeron
              <select value={subID} onChange={(e) => setSubID(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
                <option value="">Seçin…</option>
                {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
              </select>
            </label>
            <label className="text-xs text-beton-300">
              Yaptırım
              <select value={level} onChange={(e) => setLevel(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
                <option value="Fine">Para Cezası</option>
                <option value="Warning">Uyarı</option>
              </select>
            </label>
            {level === "Fine" && (
              <label className="text-xs text-beton-300">
                Tutar (TL)
                <input type="number" min="0" step="0.01" value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className="mt-1 block w-32 rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
              </label>
            )}
          </div>
          <label className="block text-xs text-beton-300">
            İhlal türü
            <input value={violation} onChange={(e) => setViolation(e.target.value)}
              placeholder="Baretsiz çalışma"
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </label>
          <label className="block text-xs text-beton-300">
            Açıklama
            <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2}
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </label>
          <div className="flex flex-wrap items-center gap-3">
            {/* capture: kanıt fotoğrafı sahada kameradan */}
            <input ref={fileRef} type="file" accept="image/*" capture="environment"
              className="text-xs text-beton-300 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
            <button onClick={issue} disabled={busy || !subID || !violation.trim()}
              className="rounded-md bg-red-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-red-400 disabled:opacity-40">
              {busy ? "Kesiliyor…" : "Tutanağı Kes (PDF + bildirim)"}
            </button>
          </div>
        </div>
      )}

      <div className="rounded-lg border border-beton-800 divide-y divide-beton-800">
        {list.map((p) => (
          <div key={p.id} className="px-3 py-2 text-sm space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-semibold text-beton-100">{p.penalty_no}</span>
              <span className={`rounded border px-1.5 py-0.5 text-xs ${ST_STYLE[p.status]}`}>
                {ST_LABEL[p.status]}
              </span>
              <span className="text-xs text-beton-400">{LEVEL_LABEL[p.penalty_level]}</span>
              {p.amount != null && (
                <span className="text-xs text-red-300">{p.amount.toLocaleString("tr-TR")} TL</span>
              )}
              <span className="ml-auto text-xs text-beton-400">
                {new Date(p.issued_at).toLocaleString("tr-TR")} · {p.issued_by_name}
              </span>
            </div>
            <p className="text-beton-100">{p.violation_type}
              <span className="ml-2 text-xs text-beton-400">— {p.subcontractor_name}</span>
            </p>
            <div className="flex items-center gap-3 text-xs">
              {p.has_pdf && (
                <button onClick={() => downloadPdf(p)} className="text-emniyet-500 hover:underline">
                  Tutanak PDF
                </button>
              )}
              {p.status === "Issued" && (
                <button onClick={() => acknowledge(p)} className="text-blue-300 hover:underline">
                  Tebellüğ Et
                </button>
              )}
              {p.status === "AppliedToPayment" && (
                <span className="text-green-300">✓ Hakedişe kesinti olarak uygulandı</span>
              )}
            </div>
          </div>
        ))}
        {!list.length && <p className="px-3 py-4 text-sm text-beton-500">Henüz ceza tutanağı yok.</p>}
      </div>
    </div>
  );
}
