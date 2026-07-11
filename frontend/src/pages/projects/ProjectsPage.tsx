import type { ReactNode } from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects, type Project } from "../ProjectContext";

const STATUS_LABEL: Record<string, string> = {
  Planning: "Planlama",
  Active: "Aktif",
  OnHold: "Beklemede",
  Closed: "Kapandı",
  Archived: "Arşiv",
};

export default function ProjectsPage() {
  const { projects, loading, reload, select } = useProjects();
  const { can } = useAuth();
  const [showNew, setShowNew] = useState(false);
  const nav = useNavigate();

  function open(p: Project) {
    select(p.id);
    nav(`/projects/${p.id}`);
  }

  return (
    <div>
      <div className="flex items-center gap-3">
        <h1 className="font-display text-2xl font-extrabold text-white">Projeler</h1>
        {can("projects.create") && (
          <button
            onClick={() => setShowNew((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-sm transition"
          >
            {showNew ? "Kapat" : "Yeni proje"}
          </button>
        )}
      </div>

      {showNew && (
        <NewProjectForm
          onCreated={async (id) => {
            setShowNew(false);
            await reload();
            select(id);
            nav(`/projects/${id}`);
          }}
        />
      )}

      <div className="mt-4 border border-beton-800 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-beton-900 text-beton-400">
            <tr>
              <Th>Kod</Th>
              <Th>Proje</Th>
              <Th>İşveren</Th>
              <Th>Statü</Th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={4} className="px-4 py-6 text-center text-beton-400">Yükleniyor…</td></tr>
            ) : projects.length === 0 ? (
              <tr><td colSpan={4} className="px-4 py-6 text-center text-beton-400">Henüz proje yok.</td></tr>
            ) : (
              projects.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => open(p)}
                  className="border-t border-beton-800 cursor-pointer hover:bg-beton-900/60"
                >
                  <Td className="font-mono text-xs text-beton-200">{p.code}</Td>
                  <Td className="text-beton-200">{p.name}</Td>
                  <Td>{p.client_name || "—"}</Td>
                  <Td>
                    <span className="font-mono text-xs px-2 py-0.5 rounded bg-beton-800 text-beton-200">
                      {STATUS_LABEL[p.status] ?? p.status}
                    </span>
                  </Td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function NewProjectForm({ onCreated }: { onCreated: (id: string) => void }) {
  const [f, setF] = useState({ code: "", name: "", client_name: "", location: "", currency: "TRY" });
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setErr(null);
    setBusy(true);
    try {
      const res = await api<{ project: Project }>("/projects", { method: "POST", body: f });
      onCreated(res.project.id);
    } catch {
      setErr("Oluşturulamadı — kodun benzersiz olduğundan emin olun.");
    } finally {
      setBusy(false);
    }
  }

  const fields: [keyof typeof f, string][] = [
    ["code", "Proje kodu"],
    ["name", "Proje adı"],
    ["client_name", "İşveren"],
    ["location", "Lokasyon"],
    ["currency", "Para birimi"],
  ];

  return (
    <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 sm:grid-cols-2">
      {fields.map(([k, label]) => (
        <div key={k}>
          <label className="block text-xs text-beton-400 mb-1">{label}</label>
          <input
            value={f[k]}
            onChange={(e) => setF({ ...f, [k]: e.target.value })}
            className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          />
        </div>
      ))}
      {err && <p className="sm:col-span-2 text-sm text-red-400">{err}</p>}
      <div className="sm:col-span-2">
        <button
          onClick={submit}
          disabled={busy}
          className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm transition"
        >
          {busy ? "Kaydediliyor…" : "Oluştur"}
        </button>
      </div>
    </div>
  );
}

function Th({ children }: { children: ReactNode }) {
  return <th className="text-left font-medium px-4 py-2">{children}</th>;
}
function Td({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <td className={"px-4 py-2 " + className}>{children}</td>;
}
