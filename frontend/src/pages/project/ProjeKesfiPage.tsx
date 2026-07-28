import { useCallback, useEffect, useMemo, useState } from "react";
import { api, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

const KATEGORILER = [
  "Mimari",
  "Betonarme",
  "Cephe",
  "Çatı",
  "Mekanik",
  "Elektrik",
  "Peyzaj",
  "Diğer",
];

const inpBase =
  "rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

type Item = {
  id?: string;
  kategori: string;
  poz_no: string;
  tanim: string;
  birim: string;
  miktar: number;
  birim_fiyat: number;
  para_birimi: string;
  aciklama: string;
  sira: number;
  // editing state
  _dirty?: boolean;
  _new?: boolean;
};

function fmt(n: number): string {
  if (!n) return "—";
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function parseFmt(s: string): number {
  const clean = s.replace(/\./g, "").replace(",", ".");
  return parseFloat(clean) || 0;
}

function newItem(kategori: string, sira: number): Item {
  return {
    kategori, poz_no: "", tanim: "", birim: "m²",
    miktar: 0, birim_fiyat: 0, para_birimi: "TRY",
    aciklama: "", sira, _new: true, _dirty: true,
  };
}

const PARA_BIRIMLERI = ["TRY", "USD", "EUR"];

export default function ProjeKesfiPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canEdit = can("projects.edit");

  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState<Set<string>>(new Set());
  const [openKats, setOpenKats] = useState<Set<string>>(new Set(KATEGORILER));
  const [editingId, setEditingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setLoading(true);
    setErr(null);
    try {
      const res = await api<{ items: Item[] }>(`/projects/${pid}/survey-items`, { projectId: pid });
      setItems(res.items ?? []);
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 404)) {
        setErr("Keşif kalemleri yüklenemedi.");
      }
    } finally {
      setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  // Kategori bazlı gruplama
  const grouped = useMemo(() => {
    const map = new Map<string, Item[]>();
    for (const kat of KATEGORILER) map.set(kat, []);
    for (const it of items) {
      const k = it.kategori;
      if (!map.has(k)) map.set(k, []);
      map.get(k)!.push(it);
    }
    return map;
  }, [items]);

  // Kategori toplamları
  const katTotal = useMemo(() => {
    const t = new Map<string, number>();
    for (const [kat, its] of grouped) {
      t.set(kat, its.reduce((s, i) => s + i.miktar * i.birim_fiyat, 0));
    }
    return t;
  }, [grouped]);

  const grandTotal = useMemo(
    () => Array.from(katTotal.values()).reduce((s, v) => s + v, 0),
    [katTotal]
  );

  function toggleKat(kat: string) {
    setOpenKats(prev => {
      const next = new Set(prev);
      if (next.has(kat)) next.delete(kat); else next.add(kat);
      return next;
    });
  }

  function addItem(kat: string) {
    const katItems = grouped.get(kat) ?? [];
    const sira = katItems.length;
    const it = newItem(kat, sira);
    const tempId = `_new_${Date.now()}`;
    const withId = { ...it, id: tempId };
    setItems(prev => [...prev, withId]);
    setEditingId(tempId);
    if (!openKats.has(kat)) setOpenKats(prev => new Set([...prev, kat]));
  }

  function updateLocal(id: string, field: string, value: string | number) {
    setItems(prev =>
      prev.map(it => it.id === id ? { ...it, [field]: value, _dirty: true } : it)
    );
  }

  async function saveItem(it: Item) {
    if (!pid || !it.id) return;
    const tempId = it.id;
    setSaving(prev => new Set([...prev, tempId]));
    try {
      if (it._new) {
        const res = await api<{ id: string }>(`/projects/${pid}/survey-items`, {
          method: "POST",
          body: {
            kategori: it.kategori,
            poz_no: it.poz_no,
            tanim: it.tanim,
            birim: it.birim,
            miktar: it.miktar,
            birim_fiyat: it.birim_fiyat,
            para_birimi: it.para_birimi,
            aciklama: it.aciklama,
            sira: it.sira,
          },
          projectId: pid,
        });
        setItems(prev => prev.map(x =>
          x.id === tempId ? { ...x, id: res.id, _new: false, _dirty: false } : x
        ));
      } else {
        await api(`/projects/${pid}/survey-items/${it.id}`, {
          method: "PATCH",
          body: {
            kategori: it.kategori,
            poz_no: it.poz_no,
            tanim: it.tanim,
            birim: it.birim,
            miktar: it.miktar,
            birim_fiyat: it.birim_fiyat,
            para_birimi: it.para_birimi,
            aciklama: it.aciklama,
            sira: it.sira,
          },
          projectId: pid,
        });
        setItems(prev => prev.map(x =>
          x.id === it.id ? { ...x, _dirty: false } : x
        ));
      }
      setEditingId(null);
    } catch {
      setErr("Kalem kaydedilemedi.");
    } finally {
      setSaving(prev => { const n = new Set(prev); n.delete(tempId); return n; });
    }
  }

  async function deleteItem(it: Item) {
    if (!pid || !it.id) return;
    if (it._new) {
      setItems(prev => prev.filter(x => x.id !== it.id));
      return;
    }
    if (!confirm(`"${it.tanim}" kalemi silinsin mi?`)) return;
    try {
      await api(`/projects/${pid}/survey-items/${it.id}`, { method: "DELETE", projectId: pid });
      setItems(prev => prev.filter(x => x.id !== it.id));
    } catch {
      setErr("Kalem silinemedi.");
    }
  }

  if (!current) return <p className="p-6 text-beton-400">Önce üst bardan bir proje seçin.</p>;
  if (loading) return (
    <div className="p-6 text-beton-400 flex items-center gap-2">
      <span className="animate-spin inline-block">⏳</span> Yükleniyor…
    </div>
  );

  return (
    <div className="max-w-6xl mx-auto p-6 space-y-6">
      {/* Başlık + genel toplam */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-xl font-semibold text-chrome-text">Proje Keşfi</h1>
          <p className="text-sm text-chrome-text-2 mt-0.5">{current.name}</p>
        </div>
        <div className="text-right">
          <p className="text-xs text-chrome-text-3">Toplam Keşif Bedeli</p>
          <p className="text-2xl font-semibold text-chrome-text tabular-nums font-mono">
            {fmt(grandTotal)}
            <span className="text-sm text-chrome-text-2 ml-1">TRY</span>
          </p>
        </div>
      </div>

      {err && (
        <div className="rounded-md bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {err}
        </div>
      )}

      {/* Kategoriler */}
      {KATEGORILER.map(kat => {
        const katItems = grouped.get(kat) ?? [];
        const total = katTotal.get(kat) ?? 0;
        const isOpen = openKats.has(kat);

        return (
          <div key={kat} className="rounded-lg border border-chrome-border bg-chrome-2/40 overflow-hidden">
            {/* Kategori başlığı */}
            <button
              onClick={() => toggleKat(kat)}
              className="w-full flex items-center justify-between px-5 py-3
                         border-b border-chrome-border bg-chrome-2/60
                         hover:bg-chrome-active/40 transition-colors"
            >
              <div className="flex items-center gap-3">
                <span className="text-chrome-text-2 text-xs transition-transform"
                      style={{ transform: isOpen ? "rotate(90deg)" : "rotate(0)" }}>▶</span>
                <span className="text-sm font-semibold text-chrome-text">{kat}</span>
                <span className="text-xs text-chrome-text-3">
                  {katItems.length} kalem
                </span>
              </div>
              <span className="text-sm font-mono tabular-nums text-chrome-text-2">
                {total > 0 ? fmt(total) + " TRY" : "—"}
              </span>
            </button>

            {/* Kalem listesi */}
            {isOpen && (
              <div>
                {katItems.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-chrome-text-3 text-xs border-b border-chrome-border/40">
                          <th className="py-2 px-3 text-left font-medium w-14">Poz No</th>
                          <th className="py-2 px-3 text-left font-medium">Tanım</th>
                          <th className="py-2 px-3 text-left font-medium w-16">Birim</th>
                          <th className="py-2 px-3 text-right font-medium w-24">Miktar</th>
                          <th className="py-2 px-3 text-right font-medium w-32">Birim Fiyat</th>
                          <th className="py-2 px-3 text-right font-medium w-36">Tutar</th>
                          {canEdit && <th className="py-2 px-3 w-16" />}
                        </tr>
                      </thead>
                      <tbody>
                        {katItems.map(it => (
                          <tr key={it.id}
                              className={`border-b border-chrome-border/20 group hover:bg-chrome-active/20 transition-colors
                                          ${it._dirty ? "bg-blue-500/5" : ""}`}>
                            {editingId === it.id ? (
                              <EditRow
                                it={it}
                                onUpdate={(f, v) => updateLocal(it.id!, f, v)}
                                onSave={() => saveItem(it)}
                                onCancel={() => {
                                  if (it._new) setItems(p => p.filter(x => x.id !== it.id));
                                  else setEditingId(null);
                                }}
                                saving={saving.has(it.id!)}
                                canEdit={canEdit}
                              />
                            ) : (
                              <ViewRow
                                it={it}
                                canEdit={canEdit}
                                onEdit={() => setEditingId(it.id!)}
                                onDelete={() => deleteItem(it)}
                              />
                            )}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {canEdit && (
                  <div className="px-5 py-3 border-t border-chrome-border/20">
                    <button
                      onClick={() => addItem(kat)}
                      className="text-xs text-accent hover:text-accent/80 transition-colors"
                    >
                      + {kat} kalemi ekle
                    </button>
                  </div>
                )}

                {katItems.length === 0 && !canEdit && (
                  <p className="px-5 py-4 text-sm text-chrome-text-3">
                    Bu kategoride henüz kalem yok.
                  </p>
                )}
              </div>
            )}
          </div>
        );
      })}

      {/* Özet tablo */}
      <div className="rounded-lg border border-chrome-border bg-chrome-2/40 overflow-hidden">
        <div className="px-5 py-3 border-b border-chrome-border bg-chrome-2/60">
          <h2 className="text-sm font-semibold text-chrome-text">Keşif Özeti</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <tbody>
              {KATEGORILER.map(kat => {
                const total = katTotal.get(kat) ?? 0;
                const pct = grandTotal > 0 ? (total / grandTotal * 100) : 0;
                if (total === 0) return null;
                return (
                  <tr key={kat} className="border-b border-chrome-border/20">
                    <td className="px-5 py-2 text-chrome-text">{kat}</td>
                    <td className="px-5 py-2 w-1/2">
                      <div className="flex items-center gap-2">
                        <div className="flex-1 bg-beton-800 rounded-full h-1.5 overflow-hidden">
                          <div
                            className="h-full bg-accent rounded-full transition-all"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                        <span className="text-chrome-text-3 text-xs w-10 text-right">
                          %{pct.toFixed(1)}
                        </span>
                      </div>
                    </td>
                    <td className="px-5 py-2 text-right font-mono tabular-nums text-chrome-text-2">
                      {fmt(total)}
                    </td>
                    <td className="px-5 py-2 text-chrome-text-3 w-12">TRY</td>
                  </tr>
                );
              })}
              <tr className="bg-chrome-active/20 font-semibold">
                <td className="px-5 py-3 text-chrome-text">Genel Toplam</td>
                <td />
                <td className="px-5 py-3 text-right font-mono tabular-nums text-chrome-text">
                  {fmt(grandTotal)}
                </td>
                <td className="px-5 py-3 text-chrome-text-2">TRY</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

// ── Görüntüleme satırı ────────────────────────────────────────────────────────

function ViewRow({ it, canEdit, onEdit, onDelete }: {
  it: Item; canEdit: boolean; onEdit: () => void; onDelete: () => void;
}) {
  return (
    <>
      <td className="py-2 px-3 text-chrome-text-3 text-xs">{it.poz_no || "—"}</td>
      <td className="py-2 px-3 text-chrome-text">{it.tanim}</td>
      <td className="py-2 px-3 text-chrome-text-2">{it.birim}</td>
      <td className="py-2 px-3 text-right tabular-nums text-chrome-text-2">
        {it.miktar.toLocaleString("tr-TR", { maximumFractionDigits: 3 })}
      </td>
      <td className="py-2 px-3 text-right tabular-nums font-mono text-chrome-text-2">
        {fmt(it.birim_fiyat)}
      </td>
      <td className="py-2 px-3 text-right tabular-nums font-mono text-chrome-text font-medium">
        {fmt(it.miktar * it.birim_fiyat)}
      </td>
      {canEdit && (
        <td className="py-2 px-3">
          <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity justify-end">
            <button onClick={onEdit}
                    className="text-chrome-text-3 hover:text-accent transition-colors text-xs px-1.5 py-0.5 rounded">
              Düzenle
            </button>
            <button onClick={onDelete}
                    className="text-chrome-text-3 hover:text-red-400 transition-colors text-xs px-1.5 py-0.5 rounded">
              Sil
            </button>
          </div>
        </td>
      )}
    </>
  );
}

// ── Düzenleme satırı ─────────────────────────────────────────────────────────

function EditRow({ it, onUpdate, onSave, onCancel, saving, canEdit }: {
  it: Item;
  onUpdate: (field: string, value: string | number) => void;
  onSave: () => void;
  onCancel: () => void;
  saving: boolean;
  canEdit: boolean;
}) {
  return (
    <>
      <td className="py-1 px-2">
        <input type="text" value={it.poz_no}
               onChange={e => onUpdate("poz_no", e.target.value)}
               className={`${inpBase} w-14`} placeholder="Poz" />
      </td>
      <td className="py-1 px-2">
        <input type="text" value={it.tanim}
               onChange={e => onUpdate("tanim", e.target.value)}
               className={`${inpBase} w-full`} placeholder="İmalat tanımı" autoFocus />
      </td>
      <td className="py-1 px-2">
        <input type="text" value={it.birim}
               onChange={e => onUpdate("birim", e.target.value)}
               className={`${inpBase} w-16`} placeholder="m²" />
      </td>
      <td className="py-1 px-2">
        <input type="number" value={it.miktar || ""}
               onChange={e => onUpdate("miktar", parseFloat(e.target.value) || 0)}
               className={`${inpBase} w-24 text-right tabular-nums`}
               placeholder="0" min={0} step="any" />
      </td>
      <td className="py-1 px-2">
        <input type="text" inputMode="numeric"
               defaultValue={it.birim_fiyat ? fmt(it.birim_fiyat) : ""}
               onBlur={e => onUpdate("birim_fiyat", parseFmt(e.target.value))}
               className={`${inpBase} w-32 text-right font-mono tabular-nums`}
               placeholder="0,00" />
      </td>
      <td className="py-1 px-2 text-right font-mono tabular-nums text-chrome-text-2 text-xs">
        {fmt(it.miktar * it.birim_fiyat)}
      </td>
      <td className="py-1 px-2">
        <div className="flex gap-1">
          <button onClick={onSave} disabled={saving || !it.tanim}
                  className="text-xs bg-accent text-white px-2 py-0.5 rounded
                             hover:bg-accent/90 disabled:opacity-50 transition-colors whitespace-nowrap">
            {saving ? "…" : "Kaydet"}
          </button>
          <button onClick={onCancel}
                  className="text-xs text-chrome-text-3 hover:text-chrome-text px-1.5 py-0.5 transition-colors">
            İptal
          </button>
        </div>
      </td>
    </>
  );
}
