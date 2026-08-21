import type { ReactNode } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api, apiFetchBlob, apiUpload, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects, type Project } from "../ProjectContext";

const STATUS_LABEL: Record<string, string> = {
  Planning: "Planlama",
  Active: "Aktif",
  OnHold: "Beklemede",
  Closed: "Kapandı",
  Archived: "Arşiv",
};

// Künye çeşitlendirmesi — "Proje Türü" katalog + serbest giriş (bkz.
// ekipmanKatalog.ts'teki "Ad" seçici deseniyle aynı: seç veya elle yaz).
const PROJE_TURU_KATALOG = [
  "Konut", "Ticari / AVM", "Hastane / Sağlık",
  "Yol / Köprü / Altyapı", "Endüstriyel / Veri Merkezi", "Eğitim",
];
const PROJE_TURU_DIGER = "__diger__";

type Milestone = {
  id: string;
  name: string;
  planned_date?: string;
  actual_date?: string;
  weight_pct?: number;
  status: string;
  sort_order: number;
  row_version: number;
};

const MS_STATUS: Record<string, string> = {
  Planned: "Planlandı",
  InProgress: "Devam",
  Completed: "Tamamlandı",
  Delayed: "Gecikti",
};

export default function ProjectDetailPage() {
  const { id = "" } = useParams();
  const { can } = useAuth();
  const { reload: reloadProjects, select } = useProjects();
  const [project, setProject] = useState<Project | null>(null);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const canEdit = can("projects.edit");

  const load = useCallback(async () => {
    setErr(null);
    try {
      const p = await api<{ project: Project }>(`/projects/${id}`, { projectId: id });
      setProject(p.project);
      const m = await api<{ milestones: Milestone[] }>(`/projects/${id}/milestones`, { projectId: id });
      setMilestones(m.milestones);
    } catch {
      setErr("Proje yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [id]);

  useEffect(() => {
    select(id);
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  if (err) return <p className="text-sm text-red-400">{err} <Link to="/projects" className="text-emniyet-500 hover:underline">← Projeler</Link></p>;
  if (!project) return <p className="text-beton-400">Yükleniyor…</p>;

  return (
    <div>
      <div className="flex items-center gap-3">
        <Link to="/projects" className="text-beton-400 hover:text-beton-200 text-sm">← Projeler</Link>
      </div>
      <h1 className="mt-1 font-display text-2xl font-extrabold text-white">
        <span className="font-mono text-emniyet-500 text-lg mr-2">{project.code}</span>
        {project.name}
      </h1>

      <Kunye project={project} canEdit={canEdit} onSaved={async (p) => { setProject(p); await reloadProjects(); }} />

      <div className="mt-8 flex items-center gap-3">
        <h2 className="font-display text-lg font-bold text-white">Milestone'lar</h2>
      </div>
      <MilestoneList
        projectId={id}
        milestones={milestones}
        canEdit={canEdit}
        onChange={load}
      />
    </div>
  );
}

function Kunye({ project, canEdit, onSaved }: { project: Project; canEdit: boolean; onSaved: (p: Project) => void }) {
  const [edit, setEdit] = useState(false);
  const [f, setF] = useState(project);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [fieldErr, setFieldErr] = useState<Record<string, string>>({});
  const [projeTuruCustom, setProjeTuruCustom] = useState(
    !!project.proje_turu && !PROJE_TURU_KATALOG.includes(project.proje_turu)
  );

  const isActive = f.status === "Active";

  function validateActiveExtra(): boolean {
    if (!isActive) return true;
    const fe: Record<string, string> = {};
    if (!f.site_handover_date) fe.site_handover_date = "proje Aktif iken zorunlu";
    if (!f.client_rep_name?.trim()) fe.client_rep_name = "proje Aktif iken zorunlu";
    if (!f.site_manager_name?.trim()) fe.site_manager_name = "proje Aktif iken zorunlu";
    setFieldErr(fe);
    return Object.keys(fe).length === 0;
  }

  async function save() {
    setErr(null);
    if (!validateActiveExtra()) return;
    setBusy(true);
    try {
      const res = await api<{ project: Project }>(`/projects/${project.id}`, {
        method: "PATCH",
        projectId: project.id,
        body: {
          name: f.name,
          client_name: f.client_name,
          location: f.location,
          currency: f.currency,
          status: f.status,
          budget_total: f.budget_total,
          contract_amount: f.contract_amount,
          accent_color: f.accent_color ?? "",
          site_handover_date: f.site_handover_date ?? "",
          client_rep_name: f.client_rep_name ?? "",
          site_manager_name: f.site_manager_name ?? "",
          proje_turu: f.proje_turu ?? "",
          toplam_insaat_alani_m2: f.toplam_insaat_alani_m2 ?? null,
          kat_blok_bilgisi: f.kat_blok_bilgisi ?? "",
          row_version: project.row_version,
        },
      });
      onSaved(res.project);
      setF(res.project);
      setEdit(false);
      setFieldErr({});
    } catch (e) {
      if (e instanceof RequestError && e.api?.details) {
        setFieldErr(e.api.details as Record<string, string>);
      } else {
        setErr("Kaydedilemedi (sürüm çakışması olabilir).");
      }
    } finally {
      setBusy(false);
    }
  }

  const rows: [string, ReactNode][] = [
    ["Proje Türü", project.proje_turu || "—"],
    ["İşveren", project.client_name || "—"],
    ["Lokasyon", project.location || "—"],
    ["Toplam İnşaat Alanı", project.toplam_insaat_alani_m2 != null ? `${project.toplam_insaat_alani_m2.toLocaleString("tr-TR")} m²` : "—"],
    ["Kat / Blok Bilgisi", project.kat_blok_bilgisi || "—"],
    ["Para birimi", project.currency],
    ["Sözleşme Bedeli", project.contract_amount != null ? project.contract_amount.toLocaleString("tr-TR") : "—"],
    ["Yapım Bütçesi", project.budget_total != null ? project.budget_total.toLocaleString("tr-TR") : "—"],
    ["Statü", STATUS_LABEL[project.status] ?? project.status],
    ...(project.status === "Active" ? [
      ["Yer Teslim / İş Başı Tarihi", project.site_handover_date ? fmtDateTR(project.site_handover_date) : "—"] as [string, ReactNode],
      ["İşveren Proje Sorumlusu", project.client_rep_name || "—"] as [string, ReactNode],
      ["Şantiye Şefi", project.site_manager_name || "—"] as [string, ReactNode],
    ] : []),
    ["Vurgu Rengi", project.accent_color
      ? <span className="inline-flex items-center gap-1.5">
          <span className="w-3 h-3 rounded-full border border-beton-700 inline-block" style={{ background: project.accent_color }} />
          <span className="font-mono">{project.accent_color}</span>
        </span>
      : "Varsayılan"],
  ];

  if (!edit) {
    return (
      <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4">
        <div className="grid sm:grid-cols-2 gap-3 mb-4">
          <KunyeGorselKutusu
            projectId={project.id}
            label="Proje Görseli"
            category="ProjeGorseli"
            canUpload={false}
            linkHint={<>Henüz eklenmedi — <Link to="/" className="text-emniyet-500 hover:underline">Panel</Link>'den yükleyin.</>}
          />
          <KunyeGorselKutusu
            projectId={project.id}
            label="Konum / Vaziyet Planı Görseli"
            category="KonumGorseli"
            canUpload={canEdit}
          />
        </div>
        <div className="grid sm:grid-cols-2 gap-x-8 gap-y-2 text-sm">
          {rows.map(([k, v]) => (
            <div key={k} className="flex justify-between border-b border-beton-800/60 py-1">
              <span className="text-beton-400">{k}</span>
              <span className="text-beton-200">{v}</span>
            </div>
          ))}
        </div>
        <div className="mt-3 flex gap-2 flex-wrap">
          {canEdit && (
            <button
              onClick={() => { setF(project); setEdit(true); }}
              className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500"
            >
              Künyeyi düzenle
            </button>
          )}
          <Link
            to="/proje/ana-sozlesme"
            className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500"
          >
            Ana Sözleşme Ekle / Düzenle
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 sm:grid-cols-2">
      <Field label="Proje adı"><input className={inp} value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} /></Field>
      <Field label="Proje Türü">
        <select
          className={inp}
          value={projeTuruCustom ? PROJE_TURU_DIGER : (f.proje_turu ?? "")}
          onChange={(e) => {
            const v = e.target.value;
            if (v === PROJE_TURU_DIGER) { setProjeTuruCustom(true); setF({ ...f, proje_turu: "" }); return; }
            setProjeTuruCustom(false);
            setF({ ...f, proje_turu: v });
          }}
        >
          <option value="">— Seçin —</option>
          {PROJE_TURU_KATALOG.map((t) => <option key={t} value={t}>{t}</option>)}
          <option value={PROJE_TURU_DIGER}>Diğer (elle yaz)</option>
        </select>
        {projeTuruCustom && (
          <input
            placeholder="Örn: Karma Kullanım"
            className={`${inp} mt-1`}
            value={f.proje_turu ?? ""}
            onChange={(e) => setF({ ...f, proje_turu: e.target.value })}
          />
        )}
      </Field>
      <Field label="İşveren"><input className={inp} value={f.client_name || ""} onChange={(e) => setF({ ...f, client_name: e.target.value })} /></Field>
      <Field label="Lokasyon"><input className={inp} value={f.location || ""} onChange={(e) => setF({ ...f, location: e.target.value })} /></Field>
      <Field label="Toplam İnşaat Alanı (m²)">
        <input type="number" min={0} className={inp} value={f.toplam_insaat_alani_m2 ?? ""}
          onChange={(e) => setF({ ...f, toplam_insaat_alani_m2: e.target.value === "" ? undefined : Number(e.target.value) })} />
      </Field>
      <Field label="Kat / Blok Bilgisi">
        <input placeholder="Örn: B+12 Kat, 4 Blok" className={inp} value={f.kat_blok_bilgisi ?? ""}
          onChange={(e) => setF({ ...f, kat_blok_bilgisi: e.target.value })} />
      </Field>
      <Field label="Para birimi"><input className={inp} value={f.currency} onChange={(e) => setF({ ...f, currency: e.target.value })} /></Field>
      <Field label="Sözleşme Bedeli">
        <input type="number" className={inp} value={f.contract_amount ?? ""} onChange={(e) => setF({ ...f, contract_amount: e.target.value === "" ? undefined : Number(e.target.value) })} />
      </Field>
      <Field label="Yapım Bütçesi">
        <input type="number" className={inp} value={f.budget_total ?? ""} onChange={(e) => setF({ ...f, budget_total: e.target.value === "" ? undefined : Number(e.target.value) })} />
      </Field>
      <Field label="Statü">
        <select className={inp} value={f.status} onChange={(e) => setF({ ...f, status: e.target.value })}>
          {["Planning", "Active", "OnHold", "Closed", "Archived"].map((s) => <option key={s} value={s}>{STATUS_LABEL[s]}</option>)}
        </select>
      </Field>

      {isActive && (
        <>
          <Field label="Yer Teslim / İş Başı Tarihi *" error={fieldErr.site_handover_date}>
            <input type="date" className={inp} value={f.site_handover_date ?? ""} onChange={(e) => setF({ ...f, site_handover_date: e.target.value })} />
          </Field>
          <Field label="İşveren Proje Sorumlusu *" error={fieldErr.client_rep_name}>
            <input className={inp} value={f.client_rep_name ?? ""} onChange={(e) => setF({ ...f, client_rep_name: e.target.value })} />
          </Field>
          <Field label="Şantiye Şefi *" error={fieldErr.site_manager_name}>
            <input className={inp} value={f.site_manager_name ?? ""} onChange={(e) => setF({ ...f, site_manager_name: e.target.value })} />
          </Field>
        </>
      )}

      <Field label="Vurgu Rengi">
        <div className="flex items-center gap-2">
          <input
            type="color"
            value={f.accent_color || "#2f6fed"}
            onChange={(e) => setF({ ...f, accent_color: e.target.value })}
            className="h-9 w-12 rounded border border-beton-800 bg-beton-950 cursor-pointer"
          />
          <span className="text-xs text-beton-400 font-mono">{f.accent_color || "varsayılan"}</span>
          {f.accent_color && (
            <button type="button" onClick={() => setF({ ...f, accent_color: undefined })}
              className="text-xs text-beton-500 hover:text-beton-300 ml-auto">
              Sıfırla
            </button>
          )}
        </div>
      </Field>
      {err && <p className="sm:col-span-2 text-sm text-red-400">{err}</p>}
      <div className="sm:col-span-2 flex gap-2">
        <button onClick={save} disabled={busy} className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm">
          {busy ? "Kaydediliyor…" : "Kaydet"}
        </button>
        <button onClick={() => { setEdit(false); setFieldErr({}); }} className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500">Vazgeç</button>
      </div>
    </div>
  );
}

function MilestoneList({ projectId, milestones, canEdit, onChange }: {
  projectId: string; milestones: Milestone[]; canEdit: boolean; onChange: () => void;
}) {
  const [adding, setAdding] = useState(false);
  const [nf, setNf] = useState({ name: "", planned_date: "", weight_pct: "", status: "Planned" });

  async function add() {
    await api(`/projects/${projectId}/milestones`, {
      method: "POST",
      projectId,
      body: {
        name: nf.name,
        planned_date: nf.planned_date || null,
        weight_pct: nf.weight_pct === "" ? null : Number(nf.weight_pct),
        status: nf.status,
      },
    });
    setNf({ name: "", planned_date: "", weight_pct: "", status: "Planned" });
    setAdding(false);
    onChange();
  }

  async function setStatus(m: Milestone, status: string) {
    await api(`/projects/${projectId}/milestones/${m.id}`, {
      method: "PATCH", projectId, body: { status, row_version: m.row_version },
    });
    onChange();
  }

  async function remove(m: Milestone) {
    await api(`/projects/${projectId}/milestones/${m.id}`, { method: "DELETE", projectId });
    onChange();
  }

  return (
    <div className="mt-3 border border-beton-800 rounded-lg overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-beton-900 text-beton-400">
          <tr>
            <th className="text-left font-medium px-4 py-2">Ad</th>
            <th className="text-left font-medium px-4 py-2">Plan tarihi</th>
            <th className="text-left font-medium px-4 py-2">Ağırlık %</th>
            <th className="text-left font-medium px-4 py-2">Statü</th>
            {canEdit && <th className="px-4 py-2" />}
          </tr>
        </thead>
        <tbody>
          {milestones.length === 0 && !adding ? (
            <tr><td colSpan={5} className="px-4 py-6 text-center text-beton-400">Milestone yok.</td></tr>
          ) : (
            milestones.map((m) => (
              <tr key={m.id} className="border-t border-beton-800">
                <td className="px-4 py-2 text-beton-200">{m.name}</td>
                <td className="px-4 py-2 font-mono text-xs">{m.planned_date?.slice(0, 10) || "—"}</td>
                <td className="px-4 py-2">{m.weight_pct != null ? `${m.weight_pct}` : "—"}</td>
                <td className="px-4 py-2">
                  {canEdit ? (
                    <select
                      value={m.status}
                      onChange={(e) => setStatus(m, e.target.value)}
                      className="bg-beton-950 border border-beton-800 rounded px-2 py-0.5 text-xs text-beton-200"
                    >
                      {Object.keys(MS_STATUS).map((s) => <option key={s} value={s}>{MS_STATUS[s]}</option>)}
                    </select>
                  ) : (
                    <span className="font-mono text-xs">{MS_STATUS[m.status] ?? m.status}</span>
                  )}
                </td>
                {canEdit && (
                  <td className="px-4 py-2 text-right">
                    <button onClick={() => remove(m)} className="text-beton-400 hover:text-red-400 text-xs">sil</button>
                  </td>
                )}
              </tr>
            ))
          )}
          {adding && (
            <tr className="border-t border-beton-800 bg-beton-900/40">
              <td className="px-4 py-2"><input className={inpSm} placeholder="Ad" value={nf.name} onChange={(e) => setNf({ ...nf, name: e.target.value })} /></td>
              <td className="px-4 py-2"><input type="date" className={inpSm} value={nf.planned_date} onChange={(e) => setNf({ ...nf, planned_date: e.target.value })} /></td>
              <td className="px-4 py-2"><input type="number" className={inpSm} value={nf.weight_pct} onChange={(e) => setNf({ ...nf, weight_pct: e.target.value })} /></td>
              <td className="px-4 py-2">
                <select className={inpSm} value={nf.status} onChange={(e) => setNf({ ...nf, status: e.target.value })}>
                  {Object.keys(MS_STATUS).map((s) => <option key={s} value={s}>{MS_STATUS[s]}</option>)}
                </select>
              </td>
              <td className="px-4 py-2 text-right">
                <button onClick={add} className="text-emniyet-500 hover:underline text-xs mr-2">ekle</button>
                <button onClick={() => setAdding(false)} className="text-beton-400 text-xs">iptal</button>
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {canEdit && !adding && (
        <div className="p-2 border-t border-beton-800">
          <button onClick={() => setAdding(true)} className="text-sm text-emniyet-500 hover:underline">+ Milestone ekle</button>
        </div>
      )}
    </div>
  );
}

const inp = "w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500";
const inpSm = "w-full rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-200 outline-none focus:border-emniyet-500";

function fmtDateTR(iso: string): string {
  const [y, m, d] = iso.slice(0, 10).split("-");
  if (!y || !m || !d) return iso;
  return `${d}.${m}.${y}`;
}

// KunyeGorselKutusu — künyedeki görsel önizleme kutuları. Polimorfik documents
// motorunu (entity_type="project") doğrudan sorgular; Proje Görseli için
// Panel'deki (Dashboard.tsx CoverImageCard) YÜKLEME akışı tekrarlanmaz, aynı
// fotoğraf salt-okunur önizlenir (tek yükleme yeri kafa karıştırmasın diye) —
// Konum/Vaziyet Planı Görseli künyeye özel, burada yüklenir/değiştirilir.
function KunyeGorselKutusu({ projectId, label, category, canUpload, linkHint }: {
  projectId: string; label: string; category: string; canUpload: boolean; linkHint?: ReactNode;
}) {
  const [url, setUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const d = await api<{ documents: { id: string; latest_version?: number }[] }>(
        `/projects/${projectId}/documents?entity_type=project&entity_id=${projectId}&category=${category}`,
        { projectId });
      const doc = (d.documents ?? []).find((x) => x.latest_version);
      if (doc && doc.latest_version) {
        setUrl(await apiFetchBlob(`/projects/${projectId}/documents/${doc.id}/versions/${doc.latest_version}/download`));
      } else {
        setUrl(null);
      }
    } catch {
      setUrl(null);
    } finally {
      setLoading(false);
    }
  }, [projectId, category]);

  useEffect(() => { load(); }, [load]);

  async function upload(files: FileList | null) {
    const file = files?.[0];
    if (!file || !canUpload) return;
    setBusy(true);
    try {
      const doc = await api<{ document: { id: string } }>(`/projects/${projectId}/documents`, {
        method: "POST", projectId,
        body: { title: label, doc_category: category, entity_type: "project", entity_id: projectId },
      });
      const fd = new FormData();
      fd.append("file", file);
      await apiUpload(`/projects/${projectId}/documents/${doc.document.id}/versions`, fd);
      await load();
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-950 overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-beton-800">
        <span className="text-xs font-medium text-beton-400">{label}</span>
        {canUpload && (
          <>
            <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={(e) => upload(e.target.files)} />
            <button type="button" onClick={() => fileRef.current?.click()} disabled={busy}
              className="text-xs text-emniyet-500 hover:underline disabled:opacity-50">
              {busy ? "Yükleniyor…" : url ? "Değiştir" : "Yükle"}
            </button>
          </>
        )}
      </div>
      <div className="h-32 flex items-center justify-center">
        {loading ? (
          <span className="text-xs text-beton-500">Yükleniyor…</span>
        ) : url ? (
          <img src={url} alt={label} className="w-full h-full object-cover" />
        ) : (
          <span className="text-xs text-beton-500 px-3 text-center">{linkHint ?? "Henüz görsel eklenmedi."}</span>
        )}
      </div>
    </div>
  );
}

function Field({ label, error, children }: { label: string; error?: string; children: ReactNode }) {
  return (
    <div>
      <label className="block text-xs text-beton-400 mb-1">{label}</label>
      {children}
      {error && <p className="mt-1 text-xs text-red-400">{error}</p>}
    </div>
  );
}
