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

// Satınalma taleplerindeki rozet deseniyle aynı (bkz. PR_STATUS_STYLE,
// PurchaseRequestsPage.tsx) — statü rengiyle anında ayırt edilsin.
const STATUS_STYLE: Record<string, string> = {
  Planning: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Active: "bg-green-500/15 text-green-300 border-green-500/40",
  OnHold: "bg-amber-500/15 text-amber-300 border-amber-500/40",
  Closed: "bg-beton-800 text-beton-200 border-beton-700",
  Archived: "bg-beton-800 text-beton-200 border-beton-700",
};

// Proje kendi vurgu rengini (accent_color) tanımlamamışsa, her proje yine de
// kendi çerçevesiyle ayırt edilsin diye sırayla dönen bir palet kullanılır
// (Proje Keşfi'ndeki disiplin renkleriyle aynı ruhta).
const FALLBACK_PALETTE = ["#3b82f6", "#f97316", "#06b6d4", "#f43f5e", "#10b981", "#eab308", "#22c55e", "#8b5cf6"];
function projectAccent(p: Project, idx: number): string {
  return p.accent_color || FALLBACK_PALETTE[idx % FALLBACK_PALETTE.length];
}

function fmtDate(s?: string) {
  if (!s) return "—";
  const [y, m, d] = s.slice(0, 10).split("-");
  if (!y || !m || !d) return "—";
  return `${d}.${m}.${y}`;
}

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

      <div className="mt-4">
        {/* Başlık satırı — sütun etiketleri, tablo semantiğini korur */}
        <div className="hidden sm:grid grid-cols-[70px_2fr_1.3fr_75px_75px_100px] gap-3 px-4 py-1.5 text-xs font-medium text-beton-500 uppercase tracking-wide">
          <span>Kod</span>
          <span>Proje</span>
          <span>İşveren</span>
          <span>Başlangıç</span>
          <span>Planlanan Bitiş</span>
          <span>Statü</span>
        </div>

        {loading ? (
          <p className="px-4 py-6 text-center text-beton-400 text-sm">Yükleniyor…</p>
        ) : projects.length === 0 ? (
          <p className="px-4 py-6 text-center text-beton-400 text-sm">Henüz proje yok.</p>
        ) : (
          <div className="flex flex-col gap-2.5">
            {projects.map((p, i) => {
              const accent = projectAccent(p, i);
              return (
                <div
                  key={p.id}
                  onClick={() => open(p)}
                  className="grid grid-cols-1 sm:grid-cols-[70px_2fr_1.3fr_75px_75px_100px] gap-1.5 sm:gap-3
                             items-center rounded-lg border-l-4 border cursor-pointer px-4 py-3
                             bg-beton-900 hover:brightness-110 transition"
                  style={{ borderLeftColor: accent, borderColor: `${accent}33` }}
                >
                  <span className="font-mono text-xs text-beton-300 truncate min-w-0">{p.code}</span>
                  <span className="text-beton-100 font-medium truncate min-w-0" title={p.name}>{p.name}</span>
                  <span className="text-beton-400 text-sm truncate min-w-0" title={p.client_name}>{p.client_name || "—"}</span>
                  <span className="text-beton-400 text-xs tabular-nums whitespace-nowrap">{fmtDate(p.start_date)}</span>
                  <span className="text-beton-400 text-xs tabular-nums whitespace-nowrap">{fmtDate(p.end_date)}</span>
                  <span>
                    <span className={`text-xs font-medium px-2 py-0.5 rounded-full border ${STATUS_STYLE[p.status] ?? STATUS_STYLE.Closed}`}>
                      {STATUS_LABEL[p.status] ?? p.status}
                    </span>
                  </span>
                </div>
              );
            })}
          </div>
        )}
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
