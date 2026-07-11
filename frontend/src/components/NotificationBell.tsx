import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client";

// Faz 4 — üst bar bildirim zili: 30 sn'de bir okunmamış sayacını yoklar,
// açılır listede son bildirimleri gösterir, okundu işaretler.

type Notif = {
  id: string; type: string; title: string; body?: string;
  entity_type?: string; entity_id?: string; project_id?: string;
  read_at?: string; created_at: string;
};

export default function NotificationBell() {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Notif[]>([]);
  const [unread, setUnread] = useState(0);
  const box = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    try {
      const r = await api<{ notifications: Notif[]; unread_count: number }>(
        "/notifications?limit=15");
      setItems(r.notifications);
      setUnread(r.unread_count);
    } catch {
      /* oturum düşmüşse sessiz geç */
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 30_000);
    return () => clearInterval(t);
  }, [load]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  async function markRead(id: string) {
    try {
      await api(`/notifications/${id}/read`, { method: "POST" });
      load();
    } catch { /* */ }
  }
  async function markAll() {
    try {
      await api("/notifications/read-all", { method: "POST" });
      load();
    } catch { /* */ }
  }

  return (
    <div className="relative" ref={box}>
      <button
        onClick={() => setOpen((o) => !o)}
        title="Bildirimler"
        className="relative rounded-md border border-beton-800 px-2.5 py-1 text-beton-200 hover:border-emniyet-500 transition"
      >
        🔔
        {unread > 0 && (
          <span className="absolute -top-1.5 -right-1.5 min-w-[18px] rounded-full bg-red-500 px-1 text-center text-[10px] font-bold text-white">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 z-50 mt-2 w-80 rounded-lg border border-beton-800 bg-beton-900 shadow-xl">
          <div className="flex items-center justify-between border-b border-beton-800 px-3 py-2">
            <span className="text-sm font-semibold text-beton-100">Bildirimler</span>
            <div className="flex items-center gap-2">
              {unread > 0 && (
                <button onClick={markAll} className="text-[11px] text-emniyet-500 hover:underline">
                  Tümünü okundu işaretle
                </button>
              )}
              <Link to="/bildirim-ayarlari" onClick={() => setOpen(false)}
                className="text-[11px] text-beton-400 hover:text-beton-200" title="Kanal tercihleri">
                ⚙
              </Link>
            </div>
          </div>
          <div className="max-h-96 overflow-y-auto">
            {items.length === 0 && (
              <p className="px-3 py-4 text-center text-xs text-beton-500">Bildirim yok.</p>
            )}
            {items.map((n) => (
              <div
                key={n.id}
                onClick={() => !n.read_at && markRead(n.id)}
                className={`border-b border-beton-800/60 px-3 py-2 last:border-0 cursor-pointer hover:bg-beton-950/60 ${
                  n.read_at ? "opacity-60" : ""
                }`}
              >
                <div className="flex items-start gap-2">
                  {!n.read_at && <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-emniyet-500" />}
                  <div className="min-w-0">
                    <p className="text-xs text-beton-100 leading-snug">{n.title}</p>
                    {n.body && <p className="mt-0.5 text-[11px] text-beton-400 line-clamp-2">{n.body}</p>}
                    <p className="mt-0.5 text-[10px] text-beton-500">
                      {new Date(n.created_at).toLocaleString("tr-TR")}
                    </p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
