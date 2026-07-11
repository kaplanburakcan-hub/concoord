import type { ReactNode } from "react";
import { Fragment, useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";

type AuditEntry = {
  id: string;
  actor_name?: string;
  actor_id?: string;
  entity: string;
  entity_id?: string;
  action: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  ip?: string;
  request_id?: string;
  at: string;
};

export default function AuditLogPage() {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [entity, setEntity] = useState("");
  const [action, setAction] = useState("");
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    const qs = new URLSearchParams();
    if (entity) qs.set("entity", entity);
    if (action) qs.set("action", action);
    try {
      const res = await api<{ logs: AuditEntry[] }>(`/admin/audit-logs?${qs.toString()}`);
      setLogs(res.logs);
    } finally {
      setLoading(false);
    }
  }, [entity, action]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div>
      <h1 className="font-display text-2xl font-extrabold text-white">Denetim İzi</h1>
      <p className="mt-1 text-beton-400 text-sm">
        Her yazma işlemi kim, ne zaman, ne değiştirdi bilgisiyle kaydedilir.
      </p>

      <div className="mt-5 flex flex-wrap gap-2">
        <input
          value={entity}
          onChange={(e) => setEntity(e.target.value)}
          placeholder="varlık (örn. users)"
          className="w-52 rounded-md bg-beton-900 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
        />
        <select
          value={action}
          onChange={(e) => setAction(e.target.value)}
          className="rounded-md bg-beton-900 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
        >
          <option value="">tüm aksiyonlar</option>
          <option value="INSERT">INSERT</option>
          <option value="UPDATE">UPDATE</option>
          <option value="DELETE">DELETE</option>
        </select>
        <button onClick={load} className="rounded-md border border-beton-800 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500">
          Filtrele
        </button>
      </div>

      <div className="mt-4 border border-beton-800 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-beton-900 text-beton-400">
            <tr>
              <Th>Zaman</Th>
              <Th>Aktör</Th>
              <Th>Varlık</Th>
              <Th>Aksiyon</Th>
              <Th>IP</Th>
              <Th></Th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} className="px-4 py-6 text-center text-beton-400">Yükleniyor…</td></tr>
            ) : logs.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-6 text-center text-beton-400">Kayıt yok.</td></tr>
            ) : (
              logs.map((l) => (
                <Fragment key={l.id}>
                  <tr className="border-t border-beton-800">
                    <Td className="font-mono text-xs">{new Date(l.at).toLocaleString("tr-TR")}</Td>
                    <Td>{l.actor_name || <span className="text-beton-500">sistem</span>}</Td>
                    <Td className="font-mono text-xs">{l.entity}</Td>
                    <Td>
                      <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-beton-800 text-beton-200">{l.action}</span>
                    </Td>
                    <Td className="font-mono text-xs text-beton-400">{l.ip || "—"}</Td>
                    <Td>
                      {(l.before || l.after) && (
                        <button
                          onClick={() => setOpen(open === l.id ? null : l.id)}
                          className="text-emniyet-500 hover:underline text-xs"
                        >
                          {open === l.id ? "gizle" : "diff"}
                        </button>
                      )}
                    </Td>
                  </tr>
                  {open === l.id && (
                    <tr className="border-t border-beton-800 bg-beton-950">
                      <td colSpan={6} className="px-4 py-3">
                        <div className="grid sm:grid-cols-2 gap-4">
                          <Diff title="Önce" data={l.before} />
                          <Diff title="Sonra" data={l.after} />
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Diff({ title, data }: { title: string; data?: Record<string, unknown> }) {
  return (
    <div>
      <div className="text-xs text-beton-400 mb-1">{title}</div>
      <pre className="font-mono text-[11px] text-beton-200 whitespace-pre-wrap break-all">
        {data ? JSON.stringify(data, null, 2) : "—"}
      </pre>
    </div>
  );
}

function Th({ children }: { children?: ReactNode }) {
  return <th className="text-left font-medium px-4 py-2">{children}</th>;
}
function Td({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <td className={"px-4 py-2 " + className}>{children}</td>;
}
