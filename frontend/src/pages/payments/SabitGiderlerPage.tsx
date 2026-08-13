import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Nakit Akış Faz E — Sabit Giderler: araç kiraları, endirekt personel,
// mobilizasyon sarf vb. tekrarlayan aylık giderler. Bu kayıtlar cash_events'e
// YAZILMAZ — Render'da arka plan işçisi olmadığı için, nakit akış raporu
// (Faz F) bunları istenen tarih aralığı için ay ay sanal olarak genişletip
// gerçek cash_events satırlarıyla birleştirecek.

type FixedExpense = {
  id: string;
  label: string;
  amount: number;
  category: string;
  expense_day_of_month: number;
  start_date: string;
  end_date?: string | null;
  active: boolean;
  created_by_name: string;
  row_version: number;
};

const KATEGORILER = ["Araç Kirası", "Endirekt Personel", "Mobilizasyon", "Şantiye Genel Gider", "Diğer"];

const inpBase =
  "rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 " +
  "outline-none focus:border-emniyet-500";

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function bosForm() {
  return {
    label: "", amount: "", category: "Diğer", expense_day_of_month: "1",
    start_date: new Date().toISOString().slice(0, 10), end_date: "",
  };
}

export default function SabitGiderlerPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [liste, setListe] = useState<FixedExpense[]>([]);
  const [formAcik, setFormAcik] = useState(false);
  const [form, setForm] = useState(bosForm());
  const [olusturuluyor, setOlusturuluyor] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ fixed_expenses: FixedExpense[] }>(
        `/projects/${pid}/fixed-expenses`, { projectId: pid });
      setListe(r.fixed_expenses ?? []);
    } catch {
      setErr("Sabit giderler yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function olustur() {
    if (!pid || !form.label.trim() || !form.amount) return;
    setOlusturuluyor(true);
    setErr(null);
    try {
      await api(`/projects/${pid}/fixed-expenses`, {
        method: "POST", projectId: pid,
        body: {
          label: form.label.trim(),
          amount: Number(form.amount),
          category: form.category,
          expense_day_of_month: Number(form.expense_day_of_month),
          start_date: form.start_date,
          end_date: form.end_date || undefined,
        },
      });
      setForm(bosForm());
      setFormAcik(false);
      await load();
    } catch {
      setErr("Kayıt oluşturulamadı.");
    } finally {
      setOlusturuluyor(false);
    }
  }

  async function aktifDegistir(e: FixedExpense) {
    if (!pid) return;
    try {
      await api(`/projects/${pid}/fixed-expenses/${e.id}`, {
        method: "PATCH", projectId: pid,
        body: {
          label: e.label, amount: e.amount, category: e.category,
          expense_day_of_month: e.expense_day_of_month, start_date: e.start_date,
          end_date: e.end_date || undefined, active: !e.active, row_version: e.row_version,
        },
      });
      await load();
    } catch {
      setErr("Güncellenemedi (sayfa güncel olmayabilir).");
    }
  }

  async function sil(e: FixedExpense) {
    if (!pid || !confirm(`"${e.label}" silinsin mi?`)) return;
    try {
      await api(`/projects/${pid}/fixed-expenses/${e.id}`, { method: "DELETE", projectId: pid });
      await load();
    } catch {
      setErr("Silinemedi.");
    }
  }

  const aylikToplam = liste.filter((e) => e.active).reduce((s, e) => s + e.amount, 0);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-4xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">Sabit Giderler</h1>
          <p className="text-xs text-beton-400 mt-0.5">
            Araç kiraları, endirekt personel, mobilizasyon sarf vb. tekrarlayan aylık giderler —
            her ayın belirlenen gününde nakit akışına çıkış olarak yansır.
          </p>
        </div>
        <button onClick={() => setFormAcik(true)}
          className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110">
          + Sabit Gider Ekle
        </button>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
        <p className="text-xs text-beton-500 mb-1">Aktif Aylık Toplam</p>
        <p className="text-lg font-bold text-white">{fmt(aylikToplam)} TL / ay</p>
      </div>

      <div className="border border-beton-800 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-beton-900 border-b border-beton-800 text-left text-xs text-beton-500">
              <th className="py-2 px-3">Gider</th>
              <th className="py-2 px-3">Kategori</th>
              <th className="py-2 px-3 text-right">Tutar</th>
              <th className="py-2 px-3 text-center">Gün</th>
              <th className="py-2 px-3">Başlangıç</th>
              <th className="py-2 px-3">Bitiş</th>
              <th className="py-2 px-3">Durum</th>
              <th className="w-24" />
            </tr>
          </thead>
          <tbody>
            {liste.map((e) => (
              <tr key={e.id} className={`border-b border-beton-800/50 ${!e.active ? "opacity-50" : ""}`}>
                <td className="py-2 px-3 text-beton-100 font-medium">{e.label}</td>
                <td className="py-2 px-3 text-beton-400">{e.category}</td>
                <td className="py-2 px-3 text-right text-beton-200 font-mono">{fmt(e.amount)}</td>
                <td className="py-2 px-3 text-center text-beton-400">{e.expense_day_of_month}</td>
                <td className="py-2 px-3 text-beton-400 text-xs">{e.start_date}</td>
                <td className="py-2 px-3 text-beton-400 text-xs">{e.end_date || "—"}</td>
                <td className="py-2 px-3">
                  <button onClick={() => aktifDegistir(e)}
                    className={`text-xs px-2 py-0.5 rounded-full border ${
                      e.active ? "bg-green-500/15 text-green-300 border-green-500/40"
                      : "bg-beton-800 text-beton-500 border-beton-700"
                    }`}>
                    {e.active ? "Aktif" : "Pasif"}
                  </button>
                </td>
                <td className="py-2 px-3">
                  <button onClick={() => sil(e)} className="text-xs text-red-500 hover:text-red-400">Sil</button>
                </td>
              </tr>
            ))}
            {!liste.length && (
              <tr><td colSpan={8} className="py-6 text-center text-beton-500 text-sm">Henüz sabit gider tanımlanmadı.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Yeni Sabit Gider Formu */}
      {formAcik && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setFormAcik(false)}>
          <div className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-md mx-4 p-6 space-y-4"
            onClick={(e) => e.stopPropagation()}>
            <h2 className="font-display font-bold text-white text-lg">Yeni Sabit Gider</h2>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Gider Adı *</label>
              <input value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })}
                className={`${inpBase} w-full`} placeholder="Ör. Şantiye pikap kirası" />
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-beton-400 mb-1">Tutar (TL) *</label>
                <input value={form.amount} inputMode="decimal"
                  onChange={(e) => setForm({ ...form, amount: e.target.value })}
                  className={`${inpBase} w-full text-right`} placeholder="0" />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">Kategori</label>
                <select value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })}
                  className={`${inpBase} w-full`}>
                  {KATEGORILER.map((k) => <option key={k} value={k}>{k}</option>)}
                </select>
              </div>
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Masraflaşma Günü (ayın kaçı) *</label>
              <input value={form.expense_day_of_month} inputMode="numeric"
                onChange={(e) => setForm({ ...form, expense_day_of_month: e.target.value })}
                className={`${inpBase} w-full`} placeholder="1-28" />
              <p className="text-[10px] text-beton-500 mt-1">Her ayın bu gününde nakit akışına çıkış olarak yansır (1-28).</p>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-beton-400 mb-1">Başlangıç Tarihi *</label>
                <input type="date" value={form.start_date}
                  onChange={(e) => setForm({ ...form, start_date: e.target.value })}
                  className={`${inpBase} w-full`} />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">Bitiş Tarihi</label>
                <input type="date" value={form.end_date}
                  onChange={(e) => setForm({ ...form, end_date: e.target.value })}
                  className={`${inpBase} w-full`} />
                <p className="text-[10px] text-beton-500 mt-1">Boş bırakılırsa sınırsız devam eder.</p>
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button onClick={() => { setFormAcik(false); setForm(bosForm()); }}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
                İptal
              </button>
              <button onClick={olustur}
                disabled={!form.label.trim() || !form.amount || olusturuluyor}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
                {olusturuluyor ? "Oluşturuluyor…" : "Oluştur"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
