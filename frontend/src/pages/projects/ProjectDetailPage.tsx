import type { ReactNode } from "react";
import { useCallback, useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects, type Project } from "../ProjectContext";

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

  async function save() {
    setBusy(true);
    setErr(null);
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
          row_version: project.row_version,
        },
      });
      onSaved(res.project);
      setF(res.project);
      setEdit(false);
    } catch {
      setErr("Kaydedilemedi (sürüm çakışması olabilir).");
    } finally {
      setBusy(false);
    }
  }

  const rows: [string, ReactNode][] = [
    ["İşveren", project.client_name || "—"],
    ["Lokasyon", project.location || "—"],
    ["Para birimi", project.currency],
    ["Bütçe", project.budget_total != null ? project.budget_total.toLocaleString("tr-TR") : "—"],
    ["Statü", project.status],
  ];

  if (!edit) {
    return (
      <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4">
        <div className="grid sm:grid-cols-2 gap-x-8 gap-y-2 text-sm">
          {rows.map(([k, v]) => (
            <div key={k} className="flex justify-between border-b border-beton-800/60 py-1">
              <span className="text-beton-400">{k}</span>
              <span className="text-beton-200">{v}</span>
            </div>
          ))}
        </div>
        {canEdit && (
          <button
            onClick={() => { setF(project); setEdit(true); }}
            className="mt-3 rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500"
          >
            Künyeyi düzenle
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 sm:grid-cols-2">
      <Field label="Proje adı"><input className={inp} value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} /></Field>
      <Field label="İşveren"><input className={inp} value={f.client_name || ""} onChange={(e) => setF({ ...f, client_name: e.target.value })} /></Field>
      <Field label="Lokasyon"><input className={inp} value={f.location || ""} onChange={(e) => setF({ ...f, location: e.target.value })} /></Field>
      <Field label="Para birimi"><input className={inp} value={f.currency} onChange={(e) => setF({ ...f, currency: e.target.value })} /></Field>
      <Field label="Bütçe">
        <input type="number" className={inp} value={f.budget_total ?? ""} onChange={(e) => setF({ ...f, budget_total: e.target.value === "" ? undefined : Number(e.target.value) })} />
      </Field>
      <Field label="Statü">
        <select className={inp} value={f.status} onChange={(e) => setF({ ...f, status: e.target.value })}>
          {["Planning", "Active", "OnHold", "Closed", "Archived"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
      </Field>
      {err && <p className="sm:col-span-2 text-sm text-red-400">{err}</p>}
      <div className="sm:col-span-2 flex gap-2">
        <button onClick={save} disabled={busy} className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm">
          {busy ? "Kaydediliyor…" : "Kaydet"}
        </button>
        <button onClick={() => setEdit(false)} className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500">Vazgeç</button>
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

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <label className="block text-xs text-beton-400 mb-1">{label}</label>
      {children}
    </div>
  );
}
