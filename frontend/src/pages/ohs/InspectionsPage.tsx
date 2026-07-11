import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import type { TemplateItem } from "./ChecklistTemplatesPage";

// Faz 8 — İSG denetim listesi + detay (sonuçlar salt okunur: gönderilmiş
// denetim kanıttır, DB kilidiyle değiştirilemez).

type Inspection = {
  id: string; template_id: string; template_name: string;
  inspector_id: string; inspector_name: string; inspected_at: string;
  location_text?: string; gps_lat?: number; gps_lng?: number;
  score?: number; fail_count: number; created_at: string;
  results?: { no: number; answer: "ok" | "fail" | "na"; note?: string }[];
};

const ANSWER_LABEL: Record<string, string> = { ok: "Uygun", fail: "Uygunsuz", na: "Uygulanamaz" };
const ANSWER_STYLE: Record<string, string> = {
  ok: "text-green-300", fail: "text-red-300", na: "text-beton-400",
};

export default function InspectionsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [list, setList] = useState<Inspection[]>([]);
  const [detail, setDetail] = useState<Inspection | null>(null);
  const [tmplItems, setTmplItems] = useState<Record<number, string>>({});
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ inspections: Inspection[] }>(
        `/projects/${pid}/ohs/inspections`, { projectId: pid });
      setList(r.inspections);
    } catch { setErr("Denetimler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function open(id: string) {
    try {
      const r = await api<{ inspection: Inspection }>(
        `/projects/${pid}/ohs/inspections/${id}`, { projectId: pid });
      setDetail(r.inspection);
      // Madde metinleri şablondan (geçmiş denetimlerde şablon değişmiş olabilir;
      // metin bulunamazsa madde no gösterilir).
      const t = await api<{ templates: { id: string; items: TemplateItem[] }[] }>(
        `/ohs/checklist-templates`, { projectId: pid });
      const tmpl = t.templates.find((x) => x.id === r.inspection.template_id);
      const m: Record<number, string> = {};
      tmpl?.items.forEach((it) => { m[it.no] = it.text; });
      setTmplItems(m);
    } catch { setErr("Denetim detayı yüklenemedi."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-display font-bold text-white">İSG Denetimleri</h1>
        <Link to="/isg" className="text-xs text-beton-400 hover:text-beton-200">Bulgular</Link>
        <Link to="/isg/cezalar" className="text-xs text-beton-400 hover:text-beton-200">Cezalar</Link>
        {can("ohs.perform_inspection") && (
          <Link to="/isg/denetimler/yeni"
            className="ml-auto rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            Yeni Denetim
          </Link>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 divide-y divide-beton-800">
        {list.map((i) => (
          <button key={i.id} onClick={() => open(i.id)}
            className="w-full flex items-center gap-3 px-3 py-2 text-sm text-left hover:bg-beton-900">
            <div>
              <span className="text-beton-100">{i.template_name}</span>
              <span className="ml-2 text-xs text-beton-400">
                {new Date(i.inspected_at).toLocaleString("tr-TR")} · {i.inspector_name}
                {i.location_text ? ` · ${i.location_text}` : ""}
              </span>
            </div>
            <div className="ml-auto flex items-center gap-2">
              {i.fail_count > 0 && (
                <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
                  {i.fail_count} uygunsuz
                </span>
              )}
              {i.score != null && (
                <span className={`rounded border px-1.5 py-0.5 text-xs ${
                  i.score >= 90 ? "border-green-500/40 bg-green-500/10 text-green-300"
                  : i.score >= 70 ? "border-emniyet-500/40 bg-emniyet-500/10 text-emniyet-500"
                  : "border-red-500/40 bg-red-500/10 text-red-300"}`}>
                  %{i.score}
                </span>
              )}
            </div>
          </button>
        ))}
        {!list.length && <p className="px-3 py-4 text-sm text-beton-500">Henüz denetim yok.</p>}
      </div>

      {detail && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-2">
          <div className="flex items-center gap-3">
            <h2 className="text-sm font-semibold text-white">{detail.template_name}</h2>
            {detail.gps_lat != null && detail.gps_lng != null && (
              <span className="text-xs text-beton-400">
                GPS: {detail.gps_lat.toFixed(5)}, {detail.gps_lng.toFixed(5)}
              </span>
            )}
            <button onClick={() => setDetail(null)}
              className="ml-auto text-xs text-beton-400 hover:text-beton-200">Kapat</button>
          </div>
          <ul className="text-sm divide-y divide-beton-800">
            {(detail.results ?? []).map((r) => (
              <li key={r.no} className="py-1.5 flex items-start gap-2">
                <span className="text-xs text-beton-500 w-5 pt-0.5">{r.no}.</span>
                <span className="flex-1 text-beton-200">{tmplItems[r.no] ?? `Madde ${r.no}`}</span>
                <span className={`text-xs ${ANSWER_STYLE[r.answer]}`}>{ANSWER_LABEL[r.answer]}</span>
                {r.note && <span className="text-xs text-beton-400">— {r.note}</span>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
