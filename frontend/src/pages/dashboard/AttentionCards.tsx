import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";

// Panel dikkat sayaçları (E11): yanıtı geciken yazışma ve teslim tarihi geçen
// tedarik kalemi. Her sayaç yalnız kendi izni varsa istenir; hata olursa gizlenir.

export default function AttentionCards({ projectId, canRfi, canPlan, canTasks }: {
  projectId: string; canRfi: boolean; canPlan: boolean; canTasks: boolean;
}) {
  const [rfi, setRfi] = useState<number | null>(null);
  const [plan, setPlan] = useState<number | null>(null);
  const [overdueTasks, setOverdueTasks] = useState<number | null>(null);
  const [week, setWeek] = useState<{ tasks: number; correspondences: number; orders: number; payments: number } | null>(null);

  useEffect(() => {
    let alive = true;
    setRfi(null); setPlan(null); setOverdueTasks(null); setWeek(null);
    api<{ tasks: number; correspondences: number; orders: number; payments: number }>(`/projects/${projectId}/attention/this-week`, { projectId })
      .then((r) => alive && setWeek(r)).catch(() => {});
    if (canTasks) {
      api<{ count: number }>(`/projects/${projectId}/attention/tasks-overdue`, { projectId })
        .then((r) => alive && setOverdueTasks(r.count)).catch(() => {});
    }
    if (canRfi) {
      api<{ count: number }>(`/projects/${projectId}/attention/rfi-overdue`, { projectId })
        .then((r) => alive && setRfi(r.count)).catch(() => {});
    }
    if (canPlan) {
      api<{ count: number }>(`/projects/${projectId}/attention/plan-late`, { projectId })
        .then((r) => alive && setPlan(r.count)).catch(() => {});
    }
    return () => { alive = false; };
  }, [projectId, canRfi, canPlan, canTasks]);

  const cards: { label: string; value: number | null; to: string; hint: string }[] = [];
  if (canRfi) cards.push({ label: "Yanıtı geciken yazışma", value: rfi, to: "/yazismalar/gelen", hint: "Cevap tarihi geçmiş, hâlâ açık" });
  if (canPlan) cards.push({ label: "Teslim tarihi geçen kalem", value: plan, to: "/satinalma/tedarik-plani", hint: "Teslim alınmamış tedarik kalemi" });
  if (canTasks) cards.push({ label: "Vadesi geçen görev", value: overdueTasks, to: "/gorevler", hint: "Tamamlanmamış, vadesi geçmiş görev" });
  if (cards.length === 0 && !week) return null;

  return (
    <div className="grid gap-3 sm:grid-cols-2 p-4">
      {week && (
        <div className="rounded-lg border border-beton-800 bg-beton-900/60 p-3 sm:col-span-2">
          <div className="text-[11px] uppercase tracking-wider text-beton-500">Bu hafta (7 gün)</div>
          <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-2 text-sm">
            <div><span className="font-mono text-lg text-beton-100">{week.tasks}</span> <span className="text-beton-400">görev</span></div>
            <div><span className="font-mono text-lg text-beton-100">{week.correspondences}</span> <span className="text-beton-400">yanıt tarihi</span></div>
            <div><span className="font-mono text-lg text-beton-100">{week.orders}</span> <span className="text-beton-400">teslim</span></div>
            <div><span className="font-mono text-lg text-beton-100">{week.payments}</span> <span className="text-beton-400">hakediş dönemi</span></div>
          </div>
        </div>
      )}
      {cards.map((c) => (
        <Link key={c.label} to={c.to}
          className="rounded-lg border border-beton-800 bg-beton-900/60 p-3 hover:border-emniyet-500 transition">
          <div className="text-[11px] uppercase tracking-wider text-beton-500">{c.label}</div>
          <div className={`mt-1 font-mono text-2xl ${c.value && c.value > 0 ? "text-red-300" : "text-beton-100"}`}>
            {c.value ?? "—"}
          </div>
          <div className="text-xs text-beton-500 mt-0.5">{c.hint}</div>
        </Link>
      ))}
    </div>
  );
}
