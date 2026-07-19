import type { ReactNode } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Can } from "../auth/guards";
import { useProjects } from "../projects/ProjectContext";
import { useTheme } from "../theme/ThemeContext";
import NotificationBell from "./NotificationBell";

// Faz 10 arayüz yenileme — "Profesyonel Proje Kontrol Arayüzü".
// Sol lacivert (RAL 5026) kenar çubuğu (gruplu, ikonlu) + lacivert üst bar
// (proje seçici, tema düğmesi, bildirim, kullanıcı). İçerik <main.app-canvas>
// içinde (uçuk gökyüzü + nokta deseni). Navigasyon <Can> ile korunur.

type NavDef = { to: string; label: string; perm?: string; icon: ReactNode; badge?: number };
type NavGroup = { title: string; items: NavDef[] };

const I = {
  panel: <svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="9" /><rect x="14" y="3" width="7" height="5" /><rect x="14" y="12" width="7" height="9" /><rect x="3" y="16" width="7" height="5" /></svg>,
  portfoy: <svg viewBox="0 0 24 24"><path d="M3 3v18h18" /><path d="M7 14l4-4 3 3 5-6" /></svg>,
  proje: <svg viewBox="0 0 24 24"><path d="M3 7l9-4 9 4-9 4-9-4z" /><path d="M3 7v10l9 4 9-4V7" /></svg>,
  dok: <svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /></svg>,
  hakedis: <svg viewBox="0 0 24 24"><path d="M2 7h20v10H2z" /><circle cx="12" cy="12" r="2.5" /></svg>,
  taseron: <svg viewBox="0 0 24 24"><path d="M3 21h18" /><path d="M6 21V8l6-4 6 4v13" /><path d="M10 21v-5h4v5" /></svg>,
  malzeme: <svg viewBox="0 0 24 24"><path d="M9 11l3 3 8-8" /><path d="M20 12v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h9" /></svg>,
  satinalma: <svg viewBox="0 0 24 24"><path d="M4 8h16v12H4z" /><path d="M9 12h6" /></svg>,
  aylik: <svg viewBox="0 0 24 24"><path d="M4 4h16v5H4z" /><path d="M4 15h16v5H4z" /></svg>,
  saha: <svg viewBox="0 0 24 24"><path d="M4 20h16" /><path d="M6 20V10l6-4 6 4v10" /><path d="M9 20v-5h6v5" /></svg>,
  isg: <svg viewBox="0 0 24 24"><path d="M12 2l9 4v6c0 5-4 8-9 10-5-2-9-5-9-10V6z" /><path d="M12 8v4" /><circle cx="12" cy="15.5" r="1" /></svg>,
  gorev: <svg viewBox="0 0 24 24"><rect x="3" y="4" width="7" height="16" /><rect x="14" y="4" width="7" height="10" /></svg>,
  kullanici: <svg viewBox="0 0 24 24"><circle cx="9" cy="8" r="3" /><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6" /><circle cx="18" cy="9" r="2.4" /></svg>,
  izin: <svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="2" /><path d="M3 9h18M9 3v18" /></svg>,
  denetim: <svg viewBox="0 0 24 24"><path d="M12 8v4l3 2" /><circle cx="12" cy="12" r="9" /></svg>,
};

const GROUPS: NavGroup[] = [
  {
    title: "Genel",
    items: [
      { to: "/", label: "Panel", icon: I.panel },
      { to: "/portfoy", label: "Portföy", perm: "projects.view", icon: I.portfoy },
      { to: "/projects", label: "Projeler", perm: "projects.view", icon: I.proje },
      { to: "/documents", label: "Dokümanlar", perm: "documents.view", icon: I.dok },
    ],
  },
  {
    title: "Finans",
    items: [
      { to: "/hakedis", label: "Hakedişler", perm: "progress_payments.view", icon: I.hakedis },
      { to: "/taseronlar", label: "Taşeronlar", perm: "contracts.view", icon: I.taseron },
      { to: "/malzeme-onaylari", label: "Malzeme Onayları", perm: "material_approvals.view", icon: I.malzeme },
      { to: "/satinalma", label: "Satınalma", perm: "procurement.view", icon: I.satinalma },
      { to: "/aylik-raporlar", label: "Aylık Rapor", perm: "reports.view_financial_reports", icon: I.aylik },
    ],
  },
  {
    title: "Saha",
    items: [
      { to: "/saha-raporlari", label: "Saha Raporları", perm: "reports.view", icon: I.saha },
      { to: "/isg", label: "İSG", perm: "ohs.view", icon: I.isg },
      { to: "/gorevler", label: "Görevler", perm: "tasks.view", icon: I.gorev },
    ],
  },
  {
    title: "Yönetim",
    items: [
      { to: "/admin/users", label: "Kullanıcılar", perm: "admin.manage_users", icon: I.kullanici },
      { to: "/admin/permissions", label: "İzin Matrisi", perm: "admin.manage_permissions", icon: I.izin },
      { to: "/admin/audit", label: "Denetim İzi", perm: "admin.view_audit_log", icon: I.denetim },
    ],
  },
];

export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { projects, current, select } = useProjects();
  const { theme, toggle } = useTheme();
  const nav = useNavigate();

  async function doLogout() {
    await logout();
    nav("/login", { replace: true });
  }

  const initials = (user?.full_name ?? "?")
    .split(" ").map((s) => s[0]).slice(0, 2).join("").toUpperCase();

  return (
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-[250px_1fr]">
      {/* ---------------- Sidebar (RAL 5026) ---------------- */}
      <aside
        className="hidden md:flex flex-col sticky top-0 h-screen border-r"
        style={{ background: "var(--chrome)", color: "var(--chrome-text)", borderColor: "var(--chrome-border)" }}
      >
        <div className="flex items-center gap-3 px-5 pt-5 pb-4">
          <div
            className="w-9 h-9 rounded-[10px] grid place-items-center text-white font-medium text-[15px]"
            style={{ background: "linear-gradient(135deg,var(--accent),var(--accent-sky))" }}
          >
            İP
          </div>
          <div className="leading-tight">
            <div className="text-white font-medium text-[17px] tracking-wide">İPKS</div>
            <div className="text-[10.5px] tracking-[0.16em]" style={{ color: "var(--chrome-text-3)" }}>
              v1.0.0 · kontrol
            </div>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 pb-3">
          {GROUPS.map((g) => (
            <div key={g.title}>
              <div
                className="mt-4 mb-1.5 px-3 text-[10.5px] font-medium uppercase tracking-[0.15em]"
                style={{ color: "var(--chrome-text-3)" }}
              >
                {g.title}
              </div>
              {g.items.map((it) =>
                it.perm ? (
                  <Can key={it.to} perm={it.perm}>
                    <SideLink item={it} />
                  </Can>
                ) : (
                  <SideLink key={it.to} item={it} />
                )
              )}
            </div>
          ))}
        </nav>

        <div className="flex items-center gap-3 px-4 py-3 border-t" style={{ borderColor: "var(--chrome-border)" }}>
          <div
            className="w-9 h-9 rounded-full grid place-items-center text-white text-[13px] font-medium"
            style={{ background: "var(--chrome-2)" }}
          >
            {initials}
          </div>
          <div className="leading-tight min-w-0">
            <div className="text-[13px] text-white truncate">{user?.full_name}</div>
            <div className="text-[11px]" style={{ color: "var(--chrome-text-3)" }}>
              Oturum açık
            </div>
          </div>
        </div>
      </aside>

      {/* ---------------- Main ---------------- */}
      <div className="flex flex-col min-w-0 app-canvas">
        <header
          className="h-16 flex items-center gap-3 px-5 sticky top-0 z-10 border-b"
          style={{ background: "var(--chrome)", color: "var(--chrome-text)", borderColor: "var(--chrome-border)" }}
        >
          {/* mobil marka */}
          <span className="md:hidden text-white font-medium tracking-wide">İPKS</span>

          {projects.length > 0 && (
            <label
              className="flex items-center gap-2 rounded-[10px] px-3 py-2 border cursor-pointer"
              style={{ background: "rgba(255,255,255,.06)", borderColor: "var(--chrome-border)" }}
              title="Aktif proje"
            >
              <span className="text-[11.5px] font-medium" style={{ color: "var(--accent-sky)" }}>
                {current?.code ?? "PROJE"}
              </span>
              <select
                value={current?.id ?? ""}
                onChange={(e) => select(e.target.value || null)}
                className="bg-transparent border-0 outline-none text-[13px] max-w-[220px] cursor-pointer"
                style={{ color: "var(--chrome-text)" }}
              >
                {projects.map((p) => (
                  <option key={p.id} value={p.id} style={{ color: "#0f2036" }}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>
          )}

          <div className="ml-auto flex items-center gap-2.5">
            <NotificationBell />
            <button
              onClick={toggle}
              aria-label="Tema değiştir"
              className="w-10 h-10 rounded-[10px] grid place-items-center border transition"
              style={{ background: "rgba(255,255,255,.06)", borderColor: "var(--chrome-border)", color: "var(--chrome-text)" }}
            >
              {theme === "dark" ? (
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
                  <path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z" />
                </svg>
              ) : (
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
                  <circle cx="12" cy="12" r="4" />
                  <path d="M12 4V2M12 22v-2M4 12H2M22 12h-2M6 6 4.5 4.5M19.5 19.5 18 18M18 6l1.5-1.5M4.5 19.5 6 18" />
                </svg>
              )}
            </button>
            <button
              onClick={doLogout}
              className="h-10 px-4 rounded-[10px] text-[13px] border transition"
              style={{ background: "rgba(255,255,255,.06)", borderColor: "var(--chrome-border)", color: "var(--chrome-text)" }}
            >
              Çıkış
            </button>
          </div>
        </header>

        <main className="flex-1 app-canvas">
          <div className="max-w-6xl w-full mx-auto px-6 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}

function SideLink({ item }: { item: NavDef }) {
  return (
    <NavLink
      to={item.to}
      end={item.to === "/"}
      className={({ isActive }) =>
        "relative flex items-center gap-3 px-3 py-2.5 rounded-[9px] text-[14px] transition " +
        (isActive ? "text-white" : "hover:opacity-90")
      }
      style={({ isActive }) =>
        isActive
          ? { background: "var(--chrome-active)", color: "#fff" }
          : { color: "var(--chrome-text-2)" }
      }
    >
      {({ isActive }) => (
        <>
          <span
            className="flex-none [&>svg]:w-[18px] [&>svg]:h-[18px] [&>svg]:fill-none [&>svg]:stroke-current [&>svg]:[stroke-width:1.7]"
            style={{ color: isActive ? "var(--accent-sky)" : "currentColor" }}
          >
            {item.icon}
          </span>
          <span className="flex-1">{item.label}</span>
          {isActive && (
            <span
              className="absolute -left-3 top-1.5 bottom-1.5 w-[3px] rounded-r"
              style={{ background: "var(--accent-sky)" }}
            />
          )}
        </>
      )}
    </NavLink>
  );
}
