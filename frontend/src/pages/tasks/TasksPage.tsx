import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Faz 4 — Kanban görev panosu: HTML5 sürükle-bırak (ek bağımlılık yok),
// görev detayı çekmecesi, atama, yorum + @mention.

type Assignee = { user_id: string; full_name: string; username: string };
type Task = {
  id: string; title: string; description?: string; status: string; priority: string;
  due_date?: string; created_by: string; kanban_order: number; row_version: number;
  created_at: string; assignees: Assignee[];
};
type Comment = {
  id: string; author_id: string; author_name: string; body: string; created_at: string;
};

const STATUS_LABELS: Record<string, string> = {
  Backlog: "Birikim", Todo: "Yapılacak", InProgress: "Devam Ediyor",
  Review: "Kontrolde", Done: "Tamamlandı",
};
const PRIORITY_LABELS: Record<string, string> = {
  Low: "Düşük", Normal: "Normal", High: "Yüksek", Urgent: "Acil",
};
const PRIORITY_CLASS: Record<string, string> = {
  Low: "text-beton-400 border-beton-700",
  Normal: "text-beton-200 border-beton-600",
  High: "text-amber-400 border-amber-600",
  Urgent: "text-red-400 border-red-600",
};
// Sütun statüsü rengiyle anında ayırt edilsin — Satınalma akış panosuyla
// (ProcurementBoardPage) aynı fikir: iş ilerledikçe nötr → mavi → amber →
// mor → yeşile geçer.
const STATUS_HEAD_CLASS: Record<string, string> = {
  Backlog: "bg-beton-800/60 text-beton-200 border-beton-700",
  Todo: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  InProgress: "bg-amber-500/15 text-amber-400 border-amber-500/40",
  Review: "bg-violet-500/15 text-violet-300 border-violet-500/40",
  Done: "bg-green-500/15 text-green-300 border-green-500/40",
};

export default function TasksPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [statusOrder, setStatusOrder] = useState<string[]>([
    "Backlog", "Todo", "InProgress", "Review", "Done",
  ]);
  const [members, setMembers] = useState<Assignee[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [openTask, setOpenTask] = useState<Task | null>(null);
  const [creating, setCreating] = useState(false);
  const [dragId, setDragId] = useState<string | null>(null);
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ tasks: Task[]; status_order: string[] }>(
        `/projects/${pid}/tasks`, { projectId: pid });
      setTasks(r.tasks);
      if (r.status_order?.length) setStatusOrder(r.status_order);
      const m = await api<{ users: Assignee[] }>(
        `/projects/${pid}/assignable-users`, { projectId: pid });
      setMembers(m.users);
    } catch {
      setErr("Görevler yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const byStatus = useMemo(() => {
    const m: Record<string, Task[]> = {};
    for (const s of statusOrder) m[s] = [];
    for (const t of tasks) (m[t.status] ??= []).push(t);
    for (const s of Object.keys(m)) m[s].sort((a, b) => a.kanban_order - b.kanban_order);
    return m;
  }, [tasks, statusOrder]);

  async function moveTo(task: Task, status: string, order: number) {
    // İyimser güncelleme: pano anında yeniden düzenlenir, hata olursa geri yüklenir.
    const prev = tasks;
    setTasks((ts) => ts.map((t) => (t.id === task.id ? { ...t, status, kanban_order: order } : t)));
    try {
      const r = await api<{ task: Task }>(
        `/projects/${pid}/tasks/${task.id}/move`,
        { method: "POST", projectId: pid, body: { status, kanban_order: order, row_version: task.row_version } });
      setTasks((ts) => ts.map((t) => (t.id === task.id ? r.task : t)));
    } catch (e: any) {
      setTasks(prev);
      setErr(e?.api?.message ?? "Görev taşınamadı.");
      if (e?.status === 409) load();
    }
  }

  // Bırakılan kolonun sonuna ekleme sırası: son kartın order'ı + 1.
  function dropOrder(status: string): number {
    const col = byStatus[status] ?? [];
    return col.length ? col[col.length - 1].kanban_order + 1 : 1;
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div>
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="font-display text-2xl font-extrabold text-white">Görevler</h1>
          <p className="text-sm text-beton-400 mt-1">{current.name} — Kanban panosu. Kartları sürükleyerek taşıyın.</p>
        </div>
        {can("tasks.create") && (
          <button
            onClick={() => setCreating(true)}
            className="rounded-md bg-emniyet-500 px-3 py-1.5 text-sm font-semibold text-beton-950 hover:brightness-110 transition"
          >
            + Yeni Görev
          </button>
        )}
      </div>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-5 grid gap-3" style={{ gridTemplateColumns: `repeat(${statusOrder.length}, minmax(0, 1fr))` }}>
        {statusOrder.map((s) => (
          <div
            key={s}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              const t = tasks.find((x) => x.id === dragId);
              setDragId(null);
              if (t && t.status !== s) moveTo(t, s, dropOrder(s));
            }}
            className="rounded-lg bg-beton-900/60 border border-beton-800 min-h-[280px] flex flex-col overflow-hidden"
          >
            <div className={`flex items-center justify-between px-3 py-2 border-b ${STATUS_HEAD_CLASS[s] ?? STATUS_HEAD_CLASS.Backlog}`}>
              <span className="text-xs font-semibold uppercase tracking-wide">
                {STATUS_LABELS[s] ?? s}
              </span>
              <span className="text-xs opacity-70">{byStatus[s]?.length ?? 0}</span>
            </div>
            <div className="flex-1 space-y-2 p-2">
              {(byStatus[s] ?? []).map((t) => (
                <div
                  key={t.id}
                  draggable
                  onDragStart={() => setDragId(t.id)}
                  onDragEnd={() => setDragId(null)}
                  onClick={() => setOpenTask(t)}
                  className={`rounded-md border bg-beton-950 p-2.5 cursor-pointer hover:border-emniyet-500 transition
                    ${dragId === t.id ? "opacity-50" : ""} border-beton-800`}
                >
                  <p className="text-sm text-beton-100 leading-snug">{t.title}</p>
                  <div className="mt-2 flex items-center gap-2 flex-wrap">
                    <span className={`rounded border px-1.5 py-0.5 text-[10px] ${PRIORITY_CLASS[t.priority]}`}>
                      {PRIORITY_LABELS[t.priority] ?? t.priority}
                    </span>
                    {t.due_date && (
                      <span className={`text-[10px] ${isOverdue(t) ? "text-red-400" : "text-beton-400"}`}>
                        ⏱ {fmtDate(t.due_date)}
                      </span>
                    )}
                    {t.assignees.length > 0 && (
                      <span className="ml-auto text-[10px] text-beton-400" title={t.assignees.map((a) => a.full_name).join(", ")}>
                        {t.assignees.map((a) => initials(a.full_name)).join(" ")}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      {creating && (
        <TaskModal
          projectId={pid!} members={members} canAssign={can("tasks.assign")}
          onClose={() => setCreating(false)}
          onSaved={() => { setCreating(false); load(); }}
        />
      )}
      {openTask && (
        <TaskDrawer
          projectId={pid!} taskId={openTask.id} members={members}
          canAssign={can("tasks.assign")}
          onClose={() => setOpenTask(null)}
          onChanged={load}
        />
      )}
    </div>
  );
}

function isOverdue(t: Task): boolean {
  if (!t.due_date || t.status === "Done") return false;
  return new Date(t.due_date).getTime() < new Date(new Date().toDateString()).getTime();
}
function fmtDate(d: string): string {
  return new Date(d).toLocaleDateString("tr-TR");
}
function initials(name: string): string {
  return name.split(/\s+/).map((p) => p[0]?.toUpperCase() ?? "").slice(0, 2).join("");
}

// ---------------------------------------------------------------------------
// Yeni görev modalı
// ---------------------------------------------------------------------------
function TaskModal({ projectId, members, canAssign, onClose, onSaved }: {
  projectId: string; members: Assignee[]; canAssign: boolean;
  onClose: () => void; onSaved: () => void;
}) {
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [priority, setPriority] = useState("Normal");
  const [due, setDue] = useState("");
  const [assignees, setAssignees] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function save() {
    setErr(null);
    setBusy(true);
    try {
      await api(`/projects/${projectId}/tasks`, {
        method: "POST", projectId,
        body: {
          title,
          description: desc || null,
          priority,
          due_date: due || null,
          assignee_ids: canAssign && assignees.length ? assignees : undefined,
        },
      });
      onSaved();
    } catch (e: any) {
      setErr(e?.api?.message ?? "Görev oluşturulamadı.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Overlay onClose={onClose} title="Yeni Görev">
      <label className="block text-xs text-beton-400">Başlık *</label>
      <input value={title} onChange={(e) => setTitle(e.target.value)} className={inputCls} autoFocus />
      <label className="mt-3 block text-xs text-beton-400">Açıklama</label>
      <textarea value={desc} onChange={(e) => setDesc(e.target.value)} rows={3} className={inputCls} />
      <div className="mt-3 grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-beton-400">Öncelik</label>
          <select value={priority} onChange={(e) => setPriority(e.target.value)} className={inputCls}>
            {Object.entries(PRIORITY_LABELS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400">Termin</label>
          <input type="date" value={due} onChange={(e) => setDue(e.target.value)} className={inputCls} />
        </div>
      </div>
      {canAssign && (
        <>
          <label className="mt-3 block text-xs text-beton-400">Atananlar</label>
          <AssigneePicker members={members} value={assignees} onChange={setAssignees} />
        </>
      )}
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button onClick={onClose} className={btnGhost}>Vazgeç</button>
        <button onClick={save} disabled={busy || !title.trim()} className={btnPrimary}>
          {busy ? "Kaydediliyor…" : "Oluştur"}
        </button>
      </div>
    </Overlay>
  );
}

// ---------------------------------------------------------------------------
// Görev detay çekmecesi (düzenleme + yorumlar)
// ---------------------------------------------------------------------------
function TaskDrawer({ projectId, taskId, members, canAssign, onClose, onChanged }: {
  projectId: string; taskId: string; members: Assignee[]; canAssign: boolean;
  onClose: () => void; onChanged: () => void;
}) {
  const [task, setTask] = useState<Task | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [body, setBody] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [edit, setEdit] = useState(false);

  const load = useCallback(async () => {
    try {
      const t = await api<{ task: Task }>(`/projects/${projectId}/tasks/${taskId}`, { projectId });
      setTask(t.task);
      const c = await api<{ comments: Comment[] }>(
        `/projects/${projectId}/tasks/${taskId}/comments`, { projectId });
      setComments(c.comments);
    } catch {
      setErr("Görev yüklenemedi.");
    }
  }, [projectId, taskId]);

  useEffect(() => { load(); }, [load]);

  async function addComment() {
    if (!body.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      await api(`/projects/${projectId}/tasks/${taskId}/comments`, {
        method: "POST", projectId, body: { body },
      });
      setBody("");
      load();
    } catch (e: any) {
      setErr(e?.api?.message ?? "Yorum eklenemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!task || !confirm("Görev silinsin mi? (soft delete — denetim izinde kalır)")) return;
    try {
      await api(`/projects/${projectId}/tasks/${taskId}`, { method: "DELETE", projectId });
      onChanged();
      onClose();
    } catch (e: any) {
      setErr(e?.api?.message ?? "Görev silinemedi.");
    }
  }

  return (
    <Overlay onClose={onClose} title={task?.title ?? "Görev"}>
      {err && <p className="text-sm text-red-400">{err}</p>}
      {task && !edit && (
        <>
          <div className="flex items-center gap-2 flex-wrap text-xs">
            <span className={`rounded border px-1.5 py-0.5 ${PRIORITY_CLASS[task.priority]}`}>
              {PRIORITY_LABELS[task.priority]}
            </span>
            <span className="text-beton-400">{STATUS_LABELS[task.status] ?? task.status}</span>
            {task.due_date && (
              <span className={isOverdue(task) ? "text-red-400" : "text-beton-400"}>
                Termin: {fmtDate(task.due_date)}
              </span>
            )}
          </div>
          {task.description && (
            <p className="mt-3 text-sm text-beton-200 whitespace-pre-wrap">{task.description}</p>
          )}
          <p className="mt-3 text-xs text-beton-400">
            Atananlar: {task.assignees.length ? task.assignees.map((a) => a.full_name).join(", ") : "—"}
          </p>
          <div className="mt-3 flex gap-2">
            <button onClick={() => setEdit(true)} className={btnGhost}>Düzenle</button>
            <button onClick={remove} className="rounded-md border border-red-800 px-3 py-1 text-sm text-red-400 hover:border-red-500 transition">
              Sil
            </button>
          </div>
        </>
      )}
      {task && edit && (
        <TaskEditForm
          projectId={projectId} task={task} members={members} canAssign={canAssign}
          onCancel={() => setEdit(false)}
          onSaved={() => { setEdit(false); load(); onChanged(); }}
        />
      )}

      <h3 className="mt-6 text-sm font-semibold text-beton-200">Yorumlar</h3>
      <div className="mt-2 space-y-2 max-h-64 overflow-y-auto pr-1">
        {comments.length === 0 && <p className="text-xs text-beton-500">Henüz yorum yok.</p>}
        {comments.map((c) => (
          <div key={c.id} className="rounded-md border border-beton-800 bg-beton-950 p-2">
            <div className="flex items-center justify-between text-[11px] text-beton-400">
              <span>{c.author_name}</span>
              <span>{new Date(c.created_at).toLocaleString("tr-TR")}</span>
            </div>
            <p className="mt-1 text-sm text-beton-100 whitespace-pre-wrap">{renderMentions(c.body)}</p>
          </div>
        ))}
      </div>
      <div className="mt-3">
        <textarea
          value={body} onChange={(e) => setBody(e.target.value)} rows={2}
          placeholder="Yorum yazın… (@kullanıcıadı ile bahsedin)"
          className={inputCls}
        />
        <div className="mt-1 flex items-center justify-between">
          <MentionHint members={members} body={body} onPick={(u) => setBody((b) => appendMention(b, u))} />
          <button onClick={addComment} disabled={busy || !body.trim()} className={btnPrimary}>
            Gönder
          </button>
        </div>
      </div>
    </Overlay>
  );
}

function TaskEditForm({ projectId, task, members, canAssign, onCancel, onSaved }: {
  projectId: string; task: Task; members: Assignee[]; canAssign: boolean;
  onCancel: () => void; onSaved: () => void;
}) {
  const [title, setTitle] = useState(task.title);
  const [desc, setDesc] = useState(task.description ?? "");
  const [priority, setPriority] = useState(task.priority);
  const [due, setDue] = useState(task.due_date ? task.due_date.slice(0, 10) : "");
  const [assignees, setAssignees] = useState<string[]>(task.assignees.map((a) => a.user_id));
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true);
    setErr(null);
    try {
      await api(`/projects/${projectId}/tasks/${task.id}`, {
        method: "PATCH", projectId,
        body: {
          title, description: desc, priority,
          due_date: due, // "" = termini temizle
          assignee_ids: canAssign ? assignees : undefined,
          row_version: task.row_version,
        },
      });
      onSaved();
    } catch (e: any) {
      setErr(e?.api?.message ?? "Kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <label className="block text-xs text-beton-400">Başlık *</label>
      <input value={title} onChange={(e) => setTitle(e.target.value)} className={inputCls} />
      <label className="mt-3 block text-xs text-beton-400">Açıklama</label>
      <textarea value={desc} onChange={(e) => setDesc(e.target.value)} rows={3} className={inputCls} />
      <div className="mt-3 grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-beton-400">Öncelik</label>
          <select value={priority} onChange={(e) => setPriority(e.target.value)} className={inputCls}>
            {Object.entries(PRIORITY_LABELS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400">Termin</label>
          <input type="date" value={due} onChange={(e) => setDue(e.target.value)} className={inputCls} />
        </div>
      </div>
      {canAssign && (
        <>
          <label className="mt-3 block text-xs text-beton-400">Atananlar</label>
          <AssigneePicker members={members} value={assignees} onChange={setAssignees} />
        </>
      )}
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button onClick={onCancel} className={btnGhost}>Vazgeç</button>
        <button onClick={save} disabled={busy || !title.trim()} className={btnPrimary}>
          {busy ? "Kaydediliyor…" : "Kaydet"}
        </button>
      </div>
    </div>
  );
}

function AssigneePicker({ members, value, onChange }: {
  members: Assignee[]; value: string[]; onChange: (v: string[]) => void;
}) {
  return (
    <div className="mt-1 flex flex-wrap gap-1.5">
      {members.map((m) => {
        const on = value.includes(m.user_id);
        return (
          <button
            key={m.user_id}
            type="button"
            onClick={() => onChange(on ? value.filter((v) => v !== m.user_id) : [...value, m.user_id])}
            className={`rounded-full border px-2.5 py-0.5 text-xs transition ${
              on ? "border-emniyet-500 text-emniyet-500" : "border-beton-700 text-beton-300 hover:border-beton-500"
            }`}
          >
            {m.full_name}
          </button>
        );
      })}
      {members.length === 0 && <span className="text-xs text-beton-500">Proje üyesi yok.</span>}
    </div>
  );
}

// @mention yardımcıları — yorum kutusunda "@" yazılınca üye önerisi gösterilir.
function MentionHint({ members, body, onPick }: {
  members: Assignee[]; body: string; onPick: (u: Assignee) => void;
}) {
  const m = body.match(/(^|\s)@([a-zA-Z0-9._-]*)$/);
  if (!m) return <span />;
  const q = m[2].toLowerCase();
  const hits = members.filter((u) => u.username.toLowerCase().startsWith(q)).slice(0, 5);
  if (!hits.length) return <span />;
  return (
    <div className="flex flex-wrap gap-1">
      {hits.map((u) => (
        <button key={u.user_id} type="button" onClick={() => onPick(u)}
          className="rounded border border-beton-700 px-1.5 py-0.5 text-[11px] text-beton-300 hover:border-emniyet-500">
          @{u.username}
        </button>
      ))}
    </div>
  );
}
function appendMention(body: string, u: Assignee): string {
  return body.replace(/@([a-zA-Z0-9._-]*)$/, `@${u.username} `);
}
function renderMentions(body: string) {
  const parts = body.split(/(@[a-zA-Z0-9._-]+)/g);
  return parts.map((p, i) =>
    p.startsWith("@")
      ? <span key={i} className="text-emniyet-500">{p}</span>
      : <span key={i}>{p}</span>
  );
}

// ---------------------------------------------------------------------------
// Ortak küçük parçalar
// ---------------------------------------------------------------------------
function Overlay({ title, children, onClose }: {
  title: string; children: ReactNode; onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center bg-black/60 p-4 overflow-y-auto" onClick={onClose}>
      <div
        className="mt-10 w-full max-w-lg rounded-lg border border-beton-800 bg-beton-900 p-4 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h2 className="font-display text-lg font-bold text-white">{title}</h2>
          <button onClick={onClose} className="text-beton-400 hover:text-white">✕</button>
        </div>
        <div className="mt-3">{children}</div>
      </div>
    </div>
  );
}

const inputCls =
  "mt-1 w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";
const btnPrimary =
  "rounded-md bg-emniyet-500 px-3 py-1.5 text-sm font-semibold text-beton-950 hover:brightness-110 transition disabled:opacity-50";
const btnGhost =
  "rounded-md border border-beton-700 px-3 py-1.5 text-sm text-beton-200 hover:border-beton-500 transition";
