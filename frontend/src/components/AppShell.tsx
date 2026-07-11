import type { ReactNode } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Can } from "../auth/guards";
import { useProjects } from "../projects/ProjectContext";
import NotificationBell from "./NotificationBell";

// Navigasyon <Can> ile korunur: kullanıcı yalnızca yetkili olduğu bölümleri
// görür. Route guard'ları (RequirePerm) ayrıca doğrudan URL erişimini engeller.
export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { projects, current, select } = useProjects();
  const nav = useNavigate();

  async function doLogout() {
    await logout();
    nav("/login", { replace: true });
  }

  return (
    <div className="min-h-screen flex flex-col">
      <div
        className="h-2"
        style={{ background: "repeating-linear-gradient(-45deg,#f5b301 0 16px,#16181d 16px 32px)" }}
      />
      <header className="border-b border-beton-800 bg-beton-900">
        <div className="max-w-6xl mx-auto px-4 h-14 flex items-center gap-6">
          <span className="font-display font-extrabold text-white tracking-wide">İPKS</span>
          <nav className="flex items-center gap-1 text-sm">
            <Tab to="/">Panel</Tab>
            <Can perm="projects.view">
              <Tab to="/portfoy">Portföy</Tab>
            </Can>
            <Can perm="projects.view">
              <Tab to="/projects">Projeler</Tab>
            </Can>
            <Can perm="documents.view">
              <Tab to="/documents">Dokümanlar</Tab>
            </Can>
            <Can perm="contracts.view">
              <Tab to="/taseronlar">Taşeronlar</Tab>
            </Can>
            <Can perm="progress_payments.view">
              <Tab to="/hakedis">Hakedişler</Tab>
            </Can>
            <Can perm="material_approvals.view">
              <Tab to="/malzeme-onaylari">Malzeme Onayları</Tab>
            </Can>
            <Can perm="reports.view_financial_reports">
              <Tab to="/aylik-raporlar">Aylık Rapor</Tab>
            </Can>
            <Can perm="reports.view">
              <Tab to="/saha-raporlari">Saha Raporları</Tab>
            </Can>
            <Can perm="procurement.view">
              <Tab to="/satinalma">Satınalma</Tab>
            </Can>
            <Can perm="ohs.view">
              <Tab to="/isg">İSG</Tab>
            </Can>
            <Can perm="tasks.view">
              <Tab to="/gorevler">Görevler</Tab>
            </Can>
            <Can perm="admin.manage_users">
              <Tab to="/admin/users">Kullanıcılar</Tab>
            </Can>
            <Can perm="admin.manage_permissions">
              <Tab to="/admin/permissions">İzin Matrisi</Tab>
            </Can>
            <Can perm="admin.view_audit_log">
              <Tab to="/admin/audit">Denetim İzi</Tab>
            </Can>
          </nav>
          <div className="ml-auto flex items-center gap-3 text-sm">
            <NotificationBell />
            {projects.length > 0 && (
              <select
                value={current?.id ?? ""}
                onChange={(e) => select(e.target.value || null)}
                title="Aktif proje"
                className="max-w-[200px] rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-200 outline-none focus:border-emniyet-500"
              >
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.code} — {p.name}
                  </option>
                ))}
              </select>
            )}
            <span className="text-beton-400 hidden sm:inline">{user?.full_name}</span>
            <button
              onClick={doLogout}
              className="rounded-md border border-beton-800 px-3 py-1 text-beton-200 hover:border-emniyet-500 transition"
            >
              Çıkış
            </button>
          </div>
        </div>
      </header>
      <main className="flex-1 flex flex-col">
        <div className="max-w-6xl w-full mx-auto px-4 py-8 flex-1">{children}</div>
      </main>
    </div>
  );
}

function Tab({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end={to === "/"}
      className={({ isActive }) =>
        "px-3 py-1.5 rounded-md transition " +
        (isActive ? "bg-beton-800 text-white" : "text-beton-400 hover:text-beton-200")
      }
    >
      {children}
    </NavLink>
  );
}
