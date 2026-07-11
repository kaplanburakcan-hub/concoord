import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Faz 8 — İSG checklist şablonu yönetimi. Kontrol tanımları VERİDİR (Plan §7):
// yeni denetim türü eklemek kod değil, bu ekrandan konfigürasyon işidir.

export type TemplateItem = { no: number; text: string; critical?: boolean };
export type Template = {
  id: string; name: string; category: string; items: TemplateItem[];
  is_active: boolean; row_version: number; created_at: string;
};

export default function ChecklistTemplatesPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canManage = can("ohs.manage_checklists");

  const [templates, setTemplates] = useState<Template[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [editing, setEditing] = useState<Template | null>(null); // null=kapalı
  const [isNew, setIsNew] = useState(false);

  const load = useCallback(async () => {
    setErr(null);
    try {
      const r = await api<{ templates: Template[] }>(`/ohs/checklist-templates`, { projectId: pid });
      setTemplates(r.templates);
    } catch { setErr("Şablonlar yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  function startNew() {
    setIsNew(true);
    setEditing({
      id: "", name: "", category: "Genel", is_active: true, row_version: 0, created_at: "",
      items: [{ no: 1, text: "" }],
    });
  }

  async function save() {
    if (!editing) return;
    setErr(null);
    const body = {
      name: editing.name.trim(),
      category: editing.category.trim() || "Genel",
      items: editing.items
        .filter((it) => it.text.trim())
        .map((it, i) => ({ no: i + 1, text: it.text.trim(), critical: it.critical || undefined })),
      is_active: editing.is_active,
      row_version: editing.row_version,
    };
    try {
      if (isNew) {
        await api(`/ohs/checklist-templates`, { method: "POST", projectId: pid, body });
      } else {
        await api(`/ohs/checklist-templates/${editing.id}`, { method: "PATCH", projectId: pid, body });
      }
      setEditing(null);
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Şablon kaydedilemedi.");
    }
  }

  async function remove(t: Template) {
    if (!confirm(`"${t.name}" şablonu silinsin mi? (Geçmiş denetimler etkilenmez.)`)) return;
    try {
      await api(`/ohs/checklist-templates/${t.id}`, { method: "DELETE", projectId: pid });
      load();
    } catch { setErr("Şablon silinemedi."); }
  }

  function setItem(i: number, patch: Partial<TemplateItem>) {
    setEditing((t) => t && { ...t, items: t.items.map((x, j) => (j === i ? { ...x, ...patch } : x)) });
  }

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-display font-bold text-white">İSG Checklist Şablonları</h1>
        {canManage && !editing && (
          <button onClick={startNew}
            className="ml-auto rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            Yeni Şablon
          </button>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      {editing && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              Ad
              <input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="text-xs text-beton-300">
              Kategori
              <input value={editing.category} onChange={(e) => setEditing({ ...editing, category: e.target.value })}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="flex items-end gap-2 text-xs text-beton-300 pb-1">
              <input type="checkbox" checked={editing.is_active}
                onChange={(e) => setEditing({ ...editing, is_active: e.target.checked })} />
              Aktif
            </label>
          </div>
          <div className="space-y-2">
            {editing.items.map((it, i) => (
              <div key={i} className="flex items-center gap-2">
                <span className="text-xs text-beton-500 w-5">{i + 1}.</span>
                <input value={it.text} placeholder="Kontrol maddesi"
                  onChange={(e) => setItem(i, { text: e.target.value })}
                  className="flex-1 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <label className="flex items-center gap-1 text-xs text-beton-400">
                  <input type="checkbox" checked={!!it.critical}
                    onChange={(e) => setItem(i, { critical: e.target.checked })} />
                  kritik
                </label>
                <button onClick={() => setEditing((t) => t && { ...t, items: t.items.filter((_, j) => j !== i) })}
                  className="text-xs text-beton-500 hover:text-red-400">✕</button>
              </div>
            ))}
            <button onClick={() => setEditing((t) => t && { ...t, items: [...t.items, { no: t.items.length + 1, text: "" }] })}
              className="text-xs text-emniyet-500 hover:underline">+ Madde ekle</button>
          </div>
          <div className="flex gap-2">
            <button onClick={save} disabled={!editing.name.trim()}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
              Kaydet
            </button>
            <button onClick={() => setEditing(null)}
              className="rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-300 hover:bg-beton-800">
              Vazgeç
            </button>
          </div>
        </div>
      )}

      <div className="rounded-lg border border-beton-800 divide-y divide-beton-800">
        {templates.map((t) => (
          <div key={t.id} className="flex items-center gap-3 px-3 py-2 text-sm">
            <div>
              <span className="text-beton-100">{t.name}</span>
              <span className="ml-2 text-xs text-beton-400">{t.category} · {t.items.length} madde</span>
            </div>
            {!t.is_active && (
              <span className="rounded border border-beton-700 bg-beton-800 px-1.5 py-0.5 text-xs text-beton-400">pasif</span>
            )}
            {canManage && (
              <div className="ml-auto flex gap-3">
                <button onClick={() => { setIsNew(false); setEditing({ ...t, items: [...t.items] }); }}
                  className="text-xs text-emniyet-500 hover:underline">Düzenle</button>
                <button onClick={() => remove(t)} className="text-xs text-beton-500 hover:text-red-400">Sil</button>
              </div>
            )}
          </div>
        ))}
        {!templates.length && <p className="px-3 py-4 text-sm text-beton-500">Henüz şablon yok.</p>}
      </div>
    </div>
  );
}
