import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api } from "../../api/client";
import type { User } from "../../api/client";

type MatrixRow = {
  code: string;
  module: string;
  action: string;
  description: string;
  role_default: boolean;
  override: "GRANT" | "DENY" | null;
  effective: boolean;
};

export default function PermissionMatrixPage() {
  const [params, setParams] = useSearchParams();
  const userId = params.get("user") || "";
  const [projectId, setProjectId] = useState("");
  const [users, setUsers] = useState<User[]>([]);
  const [rows, setRows] = useState<MatrixRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Kullanıcı seçici (best-effort — manage_users izni yoksa sessizce boş kalır).
  useEffect(() => {
    api<{ users: User[] }>("/admin/users").then((r) => setUsers(r.users)).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    setErr(null);
    try {
      const qs = projectId ? `?project_id=${encodeURIComponent(projectId)}` : "";
      const res = await api<{ matrix: MatrixRow[] }>(`/admin/users/${userId}/permissions${qs}`);
      setRows(res.matrix);
    } catch {
      setErr("Matris yüklenemedi.");
    } finally {
      setLoading(false);
    }
  }, [userId, projectId]);

  useEffect(() => {
    load();
  }, [load]);

  async function setOverride(code: string, effect: "GRANT" | "DENY" | null) {
    await api(`/admin/users/${userId}/permissions/${code}`, {
      method: "PUT",
      body: { project_id: projectId || null, effect },
    });
    // Sunucudan tekrar oku: nihai (effective) karar API tarafında hesaplanır —
    // "override'ın etkisi anında API'de doğrulanıyor" (Faz 1 kabul kriteri).
    load();
  }

  const grouped = useMemo(() => {
    const m: Record<string, MatrixRow[]> = {};
    for (const r of rows) {
      if (!m[r.module]) m[r.module] = [];
      m[r.module].push(r);
    }
    return m;
  }, [rows]);

  return (
    <div>
      <h1 className="font-display text-2xl font-extrabold text-white">Kullanıcı × İzin Matrisi</h1>
      <p className="mt-1 text-beton-400 text-sm">
        Karar önceliği: <span className="font-mono text-beton-200">DENY &gt; GRANT &gt; rol varsayılanı</span>
      </p>

      <div className="mt-5 flex flex-wrap gap-3 items-end">
        <div>
          <label className="block text-xs text-beton-400 mb-1">Kullanıcı</label>
          <select
            value={userId}
            onChange={(e) => setParams(e.target.value ? { user: e.target.value } : {})}
            className="rounded-md bg-beton-900 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500 min-w-[16rem]"
          >
            <option value="">— seçin —</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>
                {u.full_name} ({u.email})
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400 mb-1">Proje kapsamı (opsiyonel)</label>
          <input
            value={projectId}
            onChange={(e) => setProjectId(e.target.value.trim())}
            placeholder="boş = global · Faz 2'de proje seçici gelir"
            className="w-80 rounded-md bg-beton-900 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          />
        </div>
      </div>

      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      {!userId ? (
        <p className="mt-8 text-beton-400 text-sm">Matrisi görmek için bir kullanıcı seçin.</p>
      ) : loading ? (
        <p className="mt-8 text-beton-400 text-sm">Yükleniyor…</p>
      ) : (
        <div className="mt-6 space-y-6">
          {Object.entries(grouped).map(([mod, list]) => (
            <div key={mod} className="border border-beton-800 rounded-lg overflow-hidden">
              <div className="bg-beton-900 px-4 py-2 font-mono text-xs tracking-wider text-emniyet-500 uppercase">
                {mod}
              </div>
              <table className="w-full text-sm">
                <tbody>
                  {list.map((r) => (
                    <tr key={r.code} className="border-t border-beton-800">
                      <td className="px-4 py-2">
                        <div className="text-beton-200">{r.action}</div>
                        <div className="text-xs text-beton-400">{r.description}</div>
                      </td>
                      <td className="px-3 py-2 text-center w-28">
                        <span className={"font-mono text-[11px] " + (r.role_default ? "text-beton-200" : "text-beton-500")}>
                          rol: {r.role_default ? "✓" : "—"}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-center w-28">
                        <EffBadge on={r.effective} />
                      </td>
                      <td className="px-4 py-2 w-72">
                        <TriState
                          value={r.override}
                          onChange={(eff) => setOverride(r.code, eff)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function EffBadge({ on }: { on: boolean }) {
  return (
    <span
      className={
        "font-mono text-[11px] px-2 py-0.5 rounded " +
        (on ? "bg-emniyet-500/15 text-emniyet-500" : "bg-beton-800 text-beton-400")
      }
    >
      {on ? "etkin" : "kapalı"}
    </span>
  );
}

function TriState({
  value,
  onChange,
}: {
  value: "GRANT" | "DENY" | null;
  onChange: (v: "GRANT" | "DENY" | null) => void;
}) {
  const opts: { key: "GRANT" | "DENY" | null; label: string; cls: string }[] = [
    { key: null, label: "Varsayılan", cls: "text-beton-300" },
    { key: "GRANT", label: "İzin ver", cls: "text-emniyet-500" },
    { key: "DENY", label: "Reddet", cls: "text-red-400" },
  ];
  return (
    <div className="inline-flex rounded-md border border-beton-800 overflow-hidden">
      {opts.map((o) => {
        const active = value === o.key;
        return (
          <button
            key={String(o.key)}
            onClick={() => onChange(o.key)}
            className={
              "px-3 py-1 text-xs transition " +
              (active ? "bg-beton-800 " + o.cls + " font-semibold" : "text-beton-400 hover:text-beton-200")
            }
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
