import { type ReactNode, useState, useEffect } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Can } from "../auth/guards";
import { useProjects } from "../projects/ProjectContext";
import { useTheme } from "../theme/ThemeContext";
import NotificationBell from "./NotificationBell";

type AltKirilim = { to: string; label: string; end?: boolean };
type NavDef = {
  to: string; label: string; perm?: string; icon: ReactNode; badge?: number; end?: boolean;
  children?: AltKirilim[];
};
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
  toplanti: <svg viewBox="0 0 24 24"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>,
  fotograf: <svg viewBox="0 0 24 24"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>,
  evrak: <svg viewBox="0 0 24 24"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>,
  makine: <svg viewBox="0 0 24 24"><rect x="2" y="7" width="20" height="14" rx="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/><line x1="12" y1="12" x2="12" y2="16"/><line x1="10" y1="14" x2="14" y2="14"/></svg>,
  ozet: <svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/><rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/></svg>,
  idarihakedis: <svg viewBox="0 0 24 24"><path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="1"/></svg>,
  rapor: <svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8"/></svg>,
  personel: <svg viewBox="0 0 24 24"><circle cx="9" cy="7" r="4"/><path d="M3 21v-2a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v2"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/><path d="M21 21v-2a4 4 0 0 0-3-3.87"/></svg>,
  depo: <svg viewBox="0 0 24 24"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>,
  sozlesme: <svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="M8 13h8M8 17h5"/></svg>,
  tasarim: <svg viewBox="0 0 24 24"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>,
  ekstrem: <svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M9 21V9"/></svg>,
  puantaj: <svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/><path d="M8 14h.01M12 14h.01M16 14h.01M8 18h.01M12 18h.01"/></svg>,
};

const GROUPS: NavGroup[] = [
  {
    title: "Genel",
    items: [
      { to: "/", label: "Panel", icon: I.panel },
      { to: "/projects", label: "Projeler", perm: "projects.view", icon: I.proje },
      { to: "/portfoy", label: "Portföy EVM", perm: "projects.view", icon: I.portfoy },
    ],
  },
  {
    title: "Proje",
    items: [
      { to: "/proje/ozet", label: "Özet / Dashboard", perm: "projects.view", icon: I.ozet },
      { to: "/proje/paydaslar", label: "Proje Paydaşları", perm: "projects.view", icon: I.kullanici },
      { to: "/proje/kesif", label: "Proje Keşfi", perm: "projects.view", icon: I.dok },
      { to: "/proje/ana-sozlesme", label: "Ana Sözleşme", perm: "projects.view", icon: I.sozlesme },
      { to: "/proje/tasarim-projeler", label: "Tasarım ve Projeler", perm: "projects.view", icon: I.tasarim },
      {
        to: "/yazismalar", label: "Yazışmalar", perm: "correspondence.view", icon: I.evrak,
        children: [
          { to: "/yazismalar/gelen", label: "Gelen Evrak" },
          { to: "/yazismalar/giden", label: "Giden Evrak" },
        ],
      },
      { to: "/hakedis/idari", label: "İdari Hakedişler", perm: "progress_payments.view", icon: I.idarihakedis, end: true },
      {
        to: "/satinalma", label: "Satın Alma ve Tedarik", perm: "procurement.view", icon: I.satinalma,
        children: [
          { to: "/satinalma", label: "Akış Panosu", end: true },
          { to: "/satinalma/talepler", label: "Talepler (PR)" },
          { to: "/satinalma/siparisler", label: "Siparişler (PO)" },
        ],
      },
      { to: "/malzeme-onaylari", label: "Malzeme Onayları", perm: "material_approvals.view", icon: I.malzeme },
      { to: "/proje/ilerleme-raporlari", label: "Proje İzleme Raporları", perm: "reports.view", icon: I.rapor },
      { to: "/aylik-raporlar/imalat", label: "İmalat Raporları", perm: "reports.view_financial_reports", icon: I.aylik },
      { to: "/proje/personel", label: "Personel Yönetimi", perm: "reports.view", icon: I.personel, end: true },
    ],
  },
  {
    title: "Taşeron ve Tedarikçi Yönetimi",
    items: [
      { to: "/taseronlar/dashboard", label: "Dashboard", perm: "contracts.view", icon: I.panel },
      { to: "/taseronlar", label: "Taşeron-Tedarikçi Sözleşmeleri", perm: "contracts.view", icon: I.taseron, end: true },
      { to: "/documents", label: "Dokümanlar", perm: "documents.view", icon: I.dok },
      { to: "/hakedis", label: "Taşeron Hakedişleri", perm: "progress_payments.view", icon: I.hakedis, end: true },
      { to: "/tedarikci-ekstreler", label: "Tedarikçi Ekstreler", perm: "contracts.view", icon: I.ekstrem },
    ],
  },
  {
    title: "Saha",
    items: [
      { to: "/saha-raporlari", label: "Günlük Rapor Girişi", perm: "reports.view", icon: I.saha, end: true },
      { to: "/saha-raporlari/haftalik", label: "Haftalık Raporlar", perm: "reports.generate_weekly", icon: I.rapor },
      { to: "/saha/tutanaklar", label: "Saha Tutanakları", perm: "reports.view", icon: I.rapor },
      { to: "/proje/personel-puantaj", label: "Personel & Puantaj Girişi", perm: "reports.view", icon: I.puantaj },
      { to: "/proje/depo", label: "Depo Raporları", perm: "reports.view", icon: I.depo },
      {
        to: "/isg", label: "İSG & OSGB", perm: "ohs.view", icon: I.isg,
        children: [
          { to: "/isg", label: "Bulgular", end: true },
          { to: "/isg/denetimler", label: "Denetimler" },
          { to: "/isg/cezalar", label: "Cezalar" },
        ],
      },
      { to: "/proje/toplanti", label: "Toplantı Tutanakları", perm: "reports.view", icon: I.toplanti },
      { to: "/gorevler", label: "Personel Görevleri", perm: "tasks.view", icon: I.gorev },
      { to: "/proje/fotograflar", label: "Fotoğraflar", perm: "documents.view", icon: I.fotograf },
    ],
  },
  {
    title: "Ayarlar",
    items: [
      { to: "/admin/users", label: "Kullanıcılar", perm: "admin.manage_users", icon: I.kullanici },
      { to: "/admin/permissions", label: "İzin Matrisi", perm: "admin.manage_permissions", icon: I.izin },
      { to: "/admin/audit", label: "Denetim İzi", perm: "admin.view_audit_log", icon: I.denetim },
    ],
  },
  {
    title: "Makine & Ekipman",
    items: [
      { to: "/makine/araclar", label: "Tanımlı Araçlar", perm: "projects.view", icon: I.makine },
      { to: "/makine/is-makineleri", label: "İş Makineleri", perm: "projects.view", icon: I.makine },
      { to: "/makine/ekipmanlar", label: "Ekipmanlar", perm: "projects.view", icon: I.makine },
    ],
  },
];

const STORAGE_KEY = "ipks.sidebar.collapsed";

function loadCollapsed(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "{}");
  } catch {
    return {};
  }
}

function groupHasActiveRoute(group: NavGroup, pathname: string): boolean {
  return group.items.some((it) => {
    if (it.to === "/" && pathname === "/") return true;
    if (it.to !== "/" && pathname.startsWith(it.to)) return true;
    return false;
  });
}

export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { projects, current, select } = useProjects();
  const { theme, toggle } = useTheme();
  const nav = useNavigate();
  const { pathname } = useLocation();

  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(loadCollapsed);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(collapsed));
  }, [collapsed]);

  function toggleGroup(title: string) {
    setCollapsed((prev) => ({ ...prev, [title]: !prev[title] }));
  }

  function isGroupOpen(group: NavGroup): boolean {
    if (groupHasActiveRoute(group, pathname)) return true;
    return !collapsed[group.title];
  }

  async function doLogout() {
    await logout();
    nav("/login", { replace: true });
  }

  const initials = (user?.full_name ?? "?")
    .split(" ").map((s) => s[0]).slice(0, 2).join("").toUpperCase();

  return (
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-[250px_1fr]">
      <aside
        className="no-print hidden md:flex flex-col sticky top-0 h-screen border-r"
        style={{ background: "var(--chrome)", color: "var(--chrome-text)", borderColor: "var(--chrome-border)" }}
      >
        <Link
          to="/"
          className="flex items-center justify-center border-b shrink-0 overflow-hidden"
          style={{
            height: "100px",
            borderColor: "#F5A800",
            boxShadow: "0 2px 8px rgba(0,0,0,0.4)",
          }}
        >
          <img src="/logo.png" alt="ConCoord" className="w-full h-full object-cover" />
        </Link>

        <nav className="flex-1 overflow-y-auto px-3 pb-3">
          {GROUPS.map((g) => {
            const open = isGroupOpen(g);
            return (
              <div key={g.title}>
                <button
                  onClick={() => toggleGroup(g.title)}
                  className="w-full flex items-center justify-between mt-4 mb-1.5 px-3 py-0.5 rounded hover:opacity-80 transition-opacity"
                  style={{ color: "var(--chrome-text-3)" }}
                >
                  <span className="flex-1 min-w-0 text-left text-[10.5px] font-medium uppercase tracking-[0.15em] leading-snug">{g.title}</span>
                  <svg
                    width="11" height="11" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" strokeWidth="2.5"
                    className="ml-2 shrink-0"
                    style={{ transition: "transform .2s", transform: open ? "rotate(0deg)" : "rotate(-90deg)" }}
                  >
                    <path d="M6 9l6 6 6-6" />
                  </svg>
                </button>
                {open && g.items.map((it) =>
                  it.perm ? (
                    <Can key={it.to} perm={it.perm}>
                      <SideLink item={it} />
                    </Can>
                  ) : (
                    <SideLink key={it.to} item={it} />
                  )
                )}
              </div>
            );
          })}
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

      <div className="flex flex-col min-w-0 app-canvas">
        <header
          className="no-print h-16 flex items-center gap-3 px-5 sticky top-0 z-10 border-b"
          style={{ background: "var(--chrome)", color: "var(--chrome-text)", borderColor: "var(--chrome-border)" }}
        >
          <span className="md:hidden text-white font-medium tracking-wide">ConCoord</span>

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
  const { pathname } = useLocation();
  const inSection =
    !!item.children &&
    (pathname === item.to || pathname.startsWith(item.to.replace(/\/$/, "") + "/")) &&
    !(item.end && pathname !== item.to);

  return (
    <>
      <NavLink
        to={item.to}
        end={item.to === "/" || !!item.end}
        className={({ isActive }) =>
          "relative flex items-center gap-3 px-3 py-2.5 rounded-[9px] text-[14px] transition min-w-0 " +
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
            <span className="flex-1 min-w-0 truncate">{item.label}</span>
            {isActive && (
              <span
                className="absolute -left-3 top-1.5 bottom-1.5 w-[3px] rounded-r"
                style={{ background: "var(--accent-sky)" }}
              />
            )}
          </>
        )}
      </NavLink>

      {inSection && (
        <div className="mb-1 ml-[30px] border-l pl-3" style={{ borderColor: "var(--chrome-border)" }}>
          {item.children!.map((c) => (
            <NavLink
              key={c.to}
              to={c.to}
              end={c.end}
              className="block py-1.5 text-[13px] transition"
              style={({ isActive }) => ({ color: isActive ? "var(--accent-sky)" : "var(--chrome-text-2)" })}
            >
              {c.label}
            </NavLink>
          ))}
        </div>
      )}
    </>
  );
}
