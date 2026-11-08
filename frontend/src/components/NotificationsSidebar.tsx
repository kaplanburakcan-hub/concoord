import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";

// Sağ kenar çubuğundaki bildirim paneli (xl ekran). Üst bardaki zil ile aynı
// veri kaynağı; burada liste kalıcı olarak görünür. Liste ~10 satır yüksekliğinde
// bir alanda tutulur, daha fazlası alan içinde kaydırılarak gezilir.

type Notif = {
  id: string; type: string; title: string; body?: string;
  read_at?: string; created_at: string;
};

const FETCH_LIMIT = 50;

export default function NotificationsSidebar() {
  const { user } = useAuth();
  const [items, setItems] = useState<Notif[]>([]);
  const [unread, setUnread] = useState(0);

  const load = useCallback(async () => {
    try {
      const r = await api<{ notifications: Notif[]; unread_count: number }>(
        `/notifications?limit=${FETCH_LIMIT}`);
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

  const initials = (user?.username ?? "")
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();

  // Üstte kullanıcı, altta Bildirimler: iki ayrı, eş ebatlı çerçeve (proje seçicinin çerçevesiyle aynı stil)
  const frameStyle = { background: "rgba(255,255,255,.06)", borderColor: "var(--chrome-border)" };

  return (
    <>
      <div className="px-3 pt-3 space-y-2 shrink-0">
        <div className="h-[52px] flex items-center justify-between gap-3 rounded-[10px] px-3 border" style={frameStyle}>
          <div className="flex items-center gap-2.5 min-w-0">
            <div
              className="w-[30px] h-[30px] rounded-full grid place-items-center shrink-0 text-[12px] font-bold"
              style={{ background: "var(--group-accent)", color: "#141414" }}
            >
              {initials}
            </div>
            <div className="min-w-0">
              <div className="text-[13px] text-white truncate">{user?.username}</div>
              {user?.title && (
                <div className="text-[11px] truncate" style={{ color: "var(--chrome-text-3)" }}>{user.title}</div>
              )}
            </div>
          </div>
          <Link to="/bildirim-ayarlari" className="text-[12px] shrink-0" style={{ color: "var(--chrome-text-3)" }} title="Kanal tercihleri">
            ⚙
          </Link>
        </div>
        <div className="h-[52px] flex items-center justify-between gap-3 rounded-[10px] px-3 border" style={frameStyle}>
          <div className="flex items-center gap-2 min-w-0">
            <span
              className="text-[11px] font-bold uppercase tracking-[.1em] whitespace-nowrap"
              style={{ color: "var(--chrome-text-3)" }}
            >
              Bildirimler
            </span>
            {unread > 0 && (
              <span className="rounded-full bg-red-500 px-1.5 text-center text-[10px] font-bold leading-[18px] text-white">
                {unread > 99 ? "99+" : unread}
              </span>
            )}
          </div>
          {unread > 0 && (
            <button onClick={markAll} className="text-[11px] hover:underline shrink-0" style={{ color: "var(--group-accent)" }}>
              Tümü okundu
            </button>
          )}
        </div>
      </div>

      {/* ~10 satır yüksekliğinde kaydırılabilir alan */}
      <div className="px-3 pt-3">
        <div className="max-h-[560px] overflow-y-auto space-y-2 pr-1">
          {items.length === 0 && (
            <p className="px-2 py-4 text-center text-[12px]" style={{ color: "var(--chrome-text-3)" }}>
              Bildirim yok.
            </p>
          )}
          {items.map((n) => (
            <button
              key={n.id}
              onClick={() => !n.read_at && markRead(n.id)}
              className={`w-full text-left rounded-lg px-3 py-2 bg-[#fef9c3] text-[#14202e] border border-[#e9d977] transition hover:brightness-95 ${
                n.read_at ? "opacity-70" : ""
              }`}
            >
              <div className="flex items-start gap-2">
                {!n.read_at && <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-[#0b2545]" />}
                <div className="min-w-0">
                  <p className="text-[12px] font-semibold leading-snug">{n.title}</p>
                  {n.body && <p className="mt-0.5 text-[11px] leading-snug text-[#5b6877] line-clamp-1">{n.body}</p>}
                  <p className="mt-0.5 text-[10px] text-[#5b6877]">
                    {new Date(n.created_at).toLocaleString("tr-TR")}
                  </p>
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </>
  );
}
