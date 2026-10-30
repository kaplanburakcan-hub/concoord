import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";

// Kişisel takvim bağlantısı (E12). Belirteç yalnız oluşturulduğu anda gösterilir;
// sonradan kopyalanamaz. Listede iptal edilebilir bağlantılar görünür.

type Feed = { id: string; created_at: string; expires_at: string };
type Created = { id: string; path: string; expires_at: string };

export default function CalendarFeedCard({ projectId }: { projectId: string }) {
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [fresh, setFresh] = useState<{ url: string; expires: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await api<{ feeds: Feed[] }>(`/projects/${projectId}/calendar-feeds`, { projectId });
      setFeeds(r.feeds ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Takvim bağlantıları"));
    }
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    setBusy(true);
    setErr(null);
    try {
      const r = await api<Created>(`/projects/${projectId}/calendar-feeds`, { method: "POST", projectId });
      setFresh({ url: window.location.origin + r.path, expires: r.expires_at });
      setCopied(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Takvim bağlantısı"));
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    try {
      await api(`/projects/${projectId}/calendar-feeds/${id}`, { method: "DELETE", projectId });
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "İptal"));
    }
  }

  async function copy() {
    if (!fresh) return;
    try {
      await navigator.clipboard.writeText(fresh.url);
      setCopied(true);
    } catch {
      setErr("Kopyalanamadı; bağlantıyı elle seçip kopyalayın.");
    }
  }

  return (
    <div className="space-y-3 text-sm">
      <p className="text-xs text-beton-500">
        Takvim uygulamanıza salt okunur abone olun: hakediş ve ödeme tarihleri, çek keside tarihleri
        ve milestone'lar görünür. Bağlantı 90 gün geçerlidir.
      </p>

      <button onClick={create} disabled={busy}
        className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 disabled:opacity-50">
        Yeni takvim bağlantısı oluştur
      </button>

      {fresh && (
        <div className="rounded-md border border-emniyet-500/40 bg-emniyet-500/10 p-3 space-y-2">
          <p className="text-xs text-beton-200">Bu bağlantı yalnızca şimdi gösterilir; kopyalayıp takvim uygulamanıza ekleyin.</p>
          <input readOnly value={fresh.url} onFocus={(e) => e.currentTarget.select()}
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs font-mono text-beton-100" />
          <div className="flex items-center gap-2 text-xs">
            <button onClick={copy} className="rounded border border-beton-700 px-2 py-1 text-beton-200 hover:border-emniyet-500">
              {copied ? "Kopyalandı" : "Kopyala"}
            </button>
            <span className="text-beton-500">Son kullanma: {fresh.expires.slice(0, 10)}</span>
          </div>
        </div>
      )}

      {err && <p className="text-xs text-red-400">{err}</p>}

      {feeds.length > 0 && (
        <ul className="divide-y divide-beton-800/60">
          {feeds.map((f) => (
            <li key={f.id} className="flex items-center gap-3 py-2 text-xs">
              <span className="flex-1 text-beton-300">Oluşturuldu {f.created_at} · son kullanma {f.expires_at}</span>
              <button onClick={() => revoke(f.id)} className="text-red-400 hover:underline">İptal et</button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
