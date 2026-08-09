import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";

// Faz 11 — Sözleşme paneli.
//
// Backend'de sözleşme CRUD'u baştan beri vardı ama arayüzü yoktu: avans tutarı,
// teminat oranı, sözleşme süresi ve yıllara sari işareti yalnızca veritabanından
// girilebiliyordu. Oysa bu alanlar hakediş hesabını DOĞRUDAN belirler:
//   · advance_amount / advance_rate_pct → avans mahsubu
//   · retention_pct                     → teminat kesintisi
//   · is_multi_year + withholding_pct   → stopaj (GVK 42-44)
//   · start_date / end_date / revised_end_date → hakediş dönem kontrolleri
//
// Süre uzatımı verildiğinde revised_end_date doldurulur; dönem kontrolleri
// uzatılmış tarihi esas alır.

type Contract = {
  id: string;
  contract_no: string;
  type: string;
  amount: number;
  advance_amount?: number;
  retention_pct: number;
  advance_rate_pct: number;
  sign_date?: string | null;
  start_date?: string | null;
  end_date?: string | null;
  revised_end_date?: string | null;
  is_multi_year: boolean;
  withholding_pct: number;
  row_version: number;
};

const empty = {
  contract_no: "",
  type: "Sub",
  amount: "",
  advance_amount: "",
  retention_pct: "5",
  advance_rate_pct: "20",
  sign_date: "",
  start_date: "",
  end_date: "",
  revised_end_date: "",
  is_multi_year: false,
  withholding_pct: "5",
};

export default function ContractPanel({
  projectId,
  subId,
  canManage,
  canFin,
}: {
  projectId: string;
  subId: string;
  canManage: boolean;
  canFin: boolean;
}) {
  const [list, setList] = useState<Contract[]>([]);
  const [editing, setEditing] = useState<string | null>(null);
  const [f, setF] = useState({ ...empty });
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await api<{ contracts: Contract[] }>(
        `/projects/${projectId}/contracts?subcontractor_id=${subId}`,
        { projectId }
      );
      setList((r.contracts ?? []).filter((c) => true));
    } catch {
      setErr("Sözleşmeler yüklenemedi.");
    }
  }, [projectId, subId]);

  useEffect(() => {
    load();
    setEditing(null);
    setF({ ...empty });
  }, [load]);

  function startEdit(c: Contract) {
    setEditing(c.id);
    setF({
      contract_no: c.contract_no,
      type: c.type,
      amount: String(c.amount ?? ""),
      advance_amount: String(c.advance_amount ?? ""),
      retention_pct: String(c.retention_pct ?? ""),
      advance_rate_pct: String(c.advance_rate_pct ?? ""),
      sign_date: c.sign_date?.slice(0, 10) ?? "",
      start_date: c.start_date?.slice(0, 10) ?? "",
      end_date: c.end_date?.slice(0, 10) ?? "",
      revised_end_date: c.revised_end_date?.slice(0, 10) ?? "",
      is_multi_year: c.is_multi_year,
      withholding_pct: String(c.withholding_pct ?? "5"),
    });
  }

  async function save() {
    setBusy(true);
    setErr(null);
    const body: Record<string, unknown> = {
      subcontractor_id: subId,
      contract_no: f.contract_no.trim(),
      type: f.type,
      amount: Number(f.amount) || 0,
      advance_amount: Number(f.advance_amount) || 0,
      retention_pct: Number(f.retention_pct) || 0,
      advance_rate_pct: Number(f.advance_rate_pct) || 0,
      sign_date: f.sign_date || null,
      start_date: f.start_date || null,
      end_date: f.end_date || null,
      revised_end_date: f.revised_end_date || null,
      is_multi_year: f.is_multi_year,
      withholding_pct: Number(f.withholding_pct) || 0,
    };
    try {
      if (editing) {
        const cur = list.find((c) => c.id === editing);
        body.row_version = cur?.row_version ?? 0;
        await api(`/projects/${projectId}/contracts/${editing}`, {
          method: "PATCH",
          body,
          projectId,
        });
      } else {
        await api(`/projects/${projectId}/contracts`, { method: "POST", body, projectId });
      }
      setEditing(null);
      setF({ ...empty });
      await load();
    } catch (e: any) {
      setErr(e?.message ?? "Sözleşme kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  const inputCls =
    "w-full rounded-md bg-beton-950 border border-beton-800 px-2.5 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";
  const labelCls = "block text-[11px] text-beton-400 mb-1";

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
      <h2 className="font-display font-medium text-beton-100 text-sm uppercase tracking-wide">
        Sözleşmeler
      </h2>
      <p className="text-xs text-beton-500 mt-1">
        Avans, teminat ve stopaj ayarları hakediş hesabını doğrudan belirler.
      </p>

      {list.length === 0 ? (
        <p className="mt-3 text-xs text-beton-500 italic">Bu taşeron için kayıtlı sözleşme yok.</p>
      ) : (
        <table className="mt-3 w-full text-sm">
          <thead className="text-beton-400 text-xs">
            <tr className="text-left">
              <th className="py-1 pr-2">Sözleşme No</th>
              {canFin && <th className="py-1 pr-2 text-right">Bedel</th>}
              <th className="py-1 pr-2 text-right">Avans %</th>
              <th className="py-1 pr-2 text-right">Teminat %</th>
              <th className="py-1 pr-2">Süre</th>
              <th className="py-1 pr-2">Stopaj</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {list.map((c) => {
              const bitis = c.revised_end_date ?? c.end_date;
              return (
                <tr key={c.id} className="border-t border-beton-800">
                  <td className="py-1.5 pr-2 text-beton-200">{c.contract_no}</td>
                  {canFin && (
                    <td className="py-1.5 pr-2 text-right tabular-nums">
                      {c.amount?.toLocaleString("tr-TR")}
                    </td>
                  )}
                  <td className="py-1.5 pr-2 text-right tabular-nums">{c.advance_rate_pct}</td>
                  <td className="py-1.5 pr-2 text-right tabular-nums">{c.retention_pct}</td>
                  <td className="py-1.5 pr-2 text-xs text-beton-400">
                    {c.start_date ? c.start_date.slice(0, 10) : "—"} /{" "}
                    {bitis ? bitis.slice(0, 10) : "—"}
                    {c.revised_end_date && (
                      <span className="ml-1 text-amber-400" title="Süre uzatımı uygulandı">
                        ↻
                      </span>
                    )}
                  </td>
                  <td className="py-1.5 pr-2 text-xs">
                    {c.is_multi_year ? (
                      <span className="text-emniyet-500">Yıllara sari · %{c.withholding_pct}</span>
                    ) : (
                      <span className="text-beton-500">Yok</span>
                    )}
                  </td>
                  <td className="py-1.5 text-right">
                    {canManage && (
                      <button
                        onClick={() => startEdit(c)}
                        className="text-xs text-emniyet-500 hover:underline"
                      >
                        Düzenle
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {canManage && (
        <div className="mt-4 border-t border-beton-800 pt-3">
          <p className="text-xs text-beton-300 mb-2">
            {editing ? "Sözleşmeyi düzenle" : "Yeni sözleşme"}
          </p>
          <div className="grid gap-2 sm:grid-cols-3">
            <div>
              <label className={labelCls}>Sözleşme No</label>
              <input
                className={inputCls}
                value={f.contract_no}
                onChange={(e) => setF({ ...f, contract_no: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>Tür</label>
              <select
                className={inputCls}
                value={f.type}
                onChange={(e) => setF({ ...f, type: e.target.value })}
              >
                <option value="Sub">Taşeron sözleşmesi</option>
                <option value="Main">Ana sözleşme</option>
                <option value="Addendum">Zeyilname</option>
              </select>
            </div>
            <div>
              <label className={labelCls}>Sözleşme bedeli</label>
              <input
                className={inputCls}
                inputMode="decimal"
                value={f.amount}
                onChange={(e) => setF({ ...f, amount: e.target.value })}
              />
            </div>

            <div>
              <label className={labelCls}>Avans tutarı</label>
              <input
                className={inputCls}
                inputMode="decimal"
                value={f.advance_amount}
                onChange={(e) => setF({ ...f, advance_amount: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>Avans mahsup oranı %</label>
              <input
                className={inputCls}
                inputMode="decimal"
                value={f.advance_rate_pct}
                onChange={(e) => setF({ ...f, advance_rate_pct: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>Teminat kesinti oranı %</label>
              <input
                className={inputCls}
                inputMode="decimal"
                value={f.retention_pct}
                onChange={(e) => setF({ ...f, retention_pct: e.target.value })}
              />
            </div>

            <div>
              <label className={labelCls}>İmza tarihi</label>
              <input
                type="date"
                className={inputCls}
                value={f.sign_date}
                onChange={(e) => setF({ ...f, sign_date: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>İşe başlama</label>
              <input
                type="date"
                className={inputCls}
                value={f.start_date}
                onChange={(e) => setF({ ...f, start_date: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>Sözleşme bitişi</label>
              <input
                type="date"
                className={inputCls}
                value={f.end_date}
                onChange={(e) => setF({ ...f, end_date: e.target.value })}
              />
            </div>

            <div>
              <label className={labelCls}>Uzatılmış bitiş (süre uzatımı)</label>
              <input
                type="date"
                className={inputCls}
                value={f.revised_end_date}
                onChange={(e) => setF({ ...f, revised_end_date: e.target.value })}
              />
            </div>
            <div>
              <label className={labelCls}>Stopaj oranı %</label>
              <input
                className={inputCls}
                inputMode="decimal"
                value={f.withholding_pct}
                onChange={(e) => setF({ ...f, withholding_pct: e.target.value })}
              />
            </div>
            <label className="flex items-start gap-2 sm:col-span-1 mt-5">
              <input
                type="checkbox"
                className="mt-0.5 accent-emniyet-500"
                checked={f.is_multi_year}
                onChange={(e) => setF({ ...f, is_multi_year: e.target.checked })}
              />
              <span className="text-xs">
                <span className="text-beton-200">Yıllara sari iş</span>
                <span className="block text-beton-500">
                  Sonraki yıla sarkan işlerde stopaj kesilir (GVK 42-44).
                </span>
              </span>
            </label>
          </div>

          <p className="mt-2 text-[11px] text-beton-500">
            Dönem kontrolleri, süre uzatımı varsa uzatılmış bitiş tarihini esas alır.
          </p>

          <div className="mt-3 flex gap-2">
            <button
              disabled={busy || !f.contract_no.trim()}
              onClick={save}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 text-sm font-medium px-4 py-2"
            >
              {editing ? "Güncelle" : "Sözleşme Ekle"}
            </button>
            {editing && (
              <button
                onClick={() => {
                  setEditing(null);
                  setF({ ...empty });
                }}
                className="rounded-md border border-beton-700 text-beton-300 text-sm px-4 py-2"
              >
                Vazgeç
              </button>
            )}
          </div>
        </div>
      )}

      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
    </div>
  );
}
