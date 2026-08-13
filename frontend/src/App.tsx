import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { ThemeProvider } from "./theme/ThemeContext";
import { AuthProvider } from "./auth/AuthContext";
import { RequireAuth, RequirePerm } from "./auth/guards";
import { ProjectProvider } from "./projects/ProjectContext";
import AppShell from "./components/AppShell";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import UsersPage from "./pages/admin/UsersPage";
import PermissionMatrixPage from "./pages/admin/PermissionMatrixPage";
import AuditLogPage from "./pages/admin/AuditLogPage";
import ProjectsPage from "./pages/projects/ProjectsPage";
import ProjectDetailPage from "./pages/projects/ProjectDetailPage";
import DocumentsPage from "./pages/documents/DocumentsPage";
import SubcontractorsPage from "./pages/payments/SubcontractorsPage";
import TaseronDashboardPage from "./pages/payments/TaseronDashboardPage";
import ProgressPaymentsPage from "./pages/payments/ProgressPaymentsPage";
import ProgressPaymentDetailPage from "./pages/payments/ProgressPaymentDetailPage";
import TasksPage from "./pages/tasks/TasksPage";
import MaterialApprovalsPage from "./pages/materials/MaterialApprovalsPage";
import MaterialApprovalDetailPage from "./pages/materials/MaterialApprovalDetailPage";
import DailyReportsPage from "./pages/reports/DailyReportsPage";
import DailyReportFormPage from "./pages/reports/DailyReportFormPage";
import WeeklyReportsPage from "./pages/reports/WeeklyReportsPage";
import MonthlyReportsPage from "./pages/reports/MonthlyReportsPage";
import PortfolioPage from "./pages/dashboard/PortfolioPage";
import PurchaseRequestsPage from "./pages/procurement/PurchaseRequestsPage";
import ProcurementBoardPage from "./pages/procurement/ProcurementBoardPage";
import PurchaseRequestDetailPage from "./pages/procurement/PurchaseRequestDetailPage";
import PurchaseOrdersPage from "./pages/procurement/PurchaseOrdersPage";
import PurchaseOrderDetailPage from "./pages/procurement/PurchaseOrderDetailPage";
import NotificationPrefsPage from "./pages/notifications/NotificationPrefsPage";
import FindingsPage from "./pages/ohs/FindingsPage";
import InspectionsPage from "./pages/ohs/InspectionsPage";
import InspectionFormPage from "./pages/ohs/InspectionFormPage";
import PenaltiesPage from "./pages/ohs/PenaltiesPage";
import ChecklistTemplatesPage from "./pages/ohs/ChecklistTemplatesPage";
import ProjectSummaryPage from "./pages/project/ProjectSummaryPage";
import ProjePaydaslariPage from "./pages/project/ProjePaydaslariPage";
import ProjeKesfiPage from "./pages/project/ProjeKesfiPage";
import IdariHakedisPage from "./pages/payments/IdariHakedisPage";
import PersonelPage from "./pages/project/PersonelPage";
import ImalatRaporlariPage from "./pages/project/ImalatRaporlariPage";
import ProjeIzlemeRaporlariPage from "./pages/project/ProjeIzlemeRaporlariPage";
import DepoPage from "./pages/project/DepoPage";
import ToplantiPage from "./pages/project/ToplantiPage";
import FotograflarPage from "./pages/project/FotograflarPage";
import AraclarPage from "./pages/makine/AraclarPage";
import IsMakineleriPage from "./pages/makine/IsMakineleriPage";
import EkipmanlarPage from "./pages/makine/EkipmanlarPage";
import ComingSoon from "./components/ComingSoon";
import SahaTutanaklariPage from "./pages/project/SahaTutanaklariPage";
import AnaSozlesmePage from "./pages/project/AnaSozlesmePage";
import TasarimVeProjelerPage from "./pages/project/TasarimVeProjelerPage";
import PersonelPuantajPage from "./pages/project/PersonelPuantajPage";
import TedarikciEkstrerlerPage from "./pages/payments/TedarikciEkstrerlerPage";
import PaymentPlanApprovalsPage from "./pages/payments/PaymentPlanApprovalsPage";
import SabitGiderlerPage from "./pages/payments/SabitGiderlerPage";
import GelenEvrakPage from "./pages/project/GelenEvrakPage";
import GidenEvrakPage from "./pages/project/GidenEvrakPage";

// Basit ComingSoon sarmalayıcı — yeni sayfalar için geçici placeholder
function ComingSoonPage({ title, description }: { title: string; description: string }) {
  return <ComingSoon title={title} description={description} />;
}

export default function App() {
  return (
    <ThemeProvider>
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={
              <RequireAuth>
                <ProjectProvider>
                  <AppShell>
                    <Routes>
                      <Route path="/" element={<Dashboard />} />
                      <Route
                        path="/portfoy"
                        element={
                          <RequirePerm perm="projects.view">
                            <PortfolioPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/aylik-raporlar"
                        element={
                          <RequirePerm perm="reports.view_financial_reports">
                            <MonthlyReportsPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/projects"
                        element={
                          <RequirePerm perm="projects.view">
                            <ProjectsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/projects/:id"
                        element={
                          <RequirePerm perm="projects.view">
                            <ProjectDetailPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/documents"
                        element={
                          <RequirePerm perm="documents.view">
                            <DocumentsPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/taseronlar/dashboard"
                        element={
                          <RequirePerm perm="contracts.view">
                            <TaseronDashboardPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/taseronlar"
                        element={
                          <RequirePerm perm="contracts.view">
                            <SubcontractorsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/hakedis"
                        element={
                          <RequirePerm perm="progress_payments.view">
                            <ProgressPaymentsPage />
                          </RequirePerm>
                        }
                      />
                      {/* ÖNEMLİ: /hakedis/idari, /hakedis/:id'den ÖNCE tanımlanmalı */}
                      <Route
                        path="/hakedis/idari"
                        element={
                          <RequirePerm perm="progress_payments.view">
                            <IdariHakedisPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/hakedis/:id"
                        element={
                          <RequirePerm perm="progress_payments.view">
                            <ProgressPaymentDetailPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/malzeme-onaylari"
                        element={
                          <RequirePerm perm="material_approvals.view">
                            <MaterialApprovalsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/malzeme-onaylari/:id"
                        element={
                          <RequirePerm perm="material_approvals.view">
                            <MaterialApprovalDetailPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/saha-raporlari"
                        element={
                          <RequirePerm perm="reports.view">
                            <DailyReportsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/saha-raporlari/haftalik"
                        element={
                          <RequirePerm perm="reports.generate_weekly">
                            <WeeklyReportsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/saha-raporlari/yeni"
                        element={
                          <RequirePerm perm="reports.create_daily">
                            <DailyReportFormPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/saha-raporlari/:id"
                        element={
                          <RequirePerm perm="reports.view">
                            <DailyReportFormPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/satinalma"
                        element={
                          <RequirePerm perm="procurement.view">
                            <ProcurementBoardPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/satinalma/talepler"
                        element={
                          <RequirePerm perm="procurement.view">
                            <PurchaseRequestsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/satinalma/talepler/:id"
                        element={
                          <RequirePerm perm="procurement.view">
                            <PurchaseRequestDetailPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/satinalma/siparisler"
                        element={
                          <RequirePerm perm="procurement.view">
                            <PurchaseOrdersPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/satinalma/siparisler/:id"
                        element={
                          <RequirePerm perm="procurement.view">
                            <PurchaseOrderDetailPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/isg"
                        element={
                          <RequirePerm perm="ohs.view">
                            <FindingsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/isg/denetimler"
                        element={
                          <RequirePerm perm="ohs.view">
                            <InspectionsPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/isg/denetimler/yeni"
                        element={
                          <RequirePerm perm="ohs.perform_inspection">
                            <InspectionFormPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/isg/cezalar"
                        element={
                          <RequirePerm perm="ohs.view">
                            <PenaltiesPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/isg/sablonlar"
                        element={
                          <RequirePerm perm="ohs.manage_checklists">
                            <ChecklistTemplatesPage />
                          </RequirePerm>
                        }
                      />

                      <Route
                        path="/gorevler"
                        element={
                          <RequirePerm perm="tasks.view">
                            <TasksPage />
                          </RequirePerm>
                        }
                      />
                      <Route path="/bildirim-ayarlari" element={<NotificationPrefsPage />} />

                      <Route
                        path="/admin/users"
                        element={
                          <RequirePerm perm="admin.manage_users">
                            <UsersPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/admin/permissions"
                        element={
                          <RequirePerm perm="admin.manage_permissions">
                            <PermissionMatrixPage />
                          </RequirePerm>
                        }
                      />
                      <Route
                        path="/admin/audit"
                        element={
                          <RequirePerm perm="admin.view_audit_log">
                            <AuditLogPage />
                          </RequirePerm>
                        }
                      />
                      <Route path="*" element={<Navigate to="/" replace />} />

                      {/* ── PROJE yeni modüller ── */}
                      <Route path="/proje/ozet" element={<RequirePerm perm="projects.view"><ProjectSummaryPage /></RequirePerm>} />
                      <Route path="/proje/paydaslar" element={<RequirePerm perm="projects.view"><ProjePaydaslariPage /></RequirePerm>} />
                      <Route path="/proje/kesif" element={<RequirePerm perm="projects.view"><ProjeKesfiPage /></RequirePerm>} />
                      <Route path="/proje/personel" element={<RequirePerm perm="reports.view"><PersonelPage /></RequirePerm>} />
                      <Route path="/aylik-raporlar/imalat" element={<RequirePerm perm="reports.view_financial_reports"><ImalatRaporlariPage /></RequirePerm>} />
                      <Route path="/proje/ilerleme-raporlari" element={<RequirePerm perm="reports.view"><ProjeIzlemeRaporlariPage /></RequirePerm>} />
                      <Route path="/proje/depo" element={<RequirePerm perm="reports.view"><DepoPage /></RequirePerm>} />
                      <Route path="/proje/toplanti" element={<RequirePerm perm="reports.view"><ToplantiPage /></RequirePerm>} />
                      <Route path="/proje/fotograflar" element={<RequirePerm perm="documents.view"><FotograflarPage /></RequirePerm>} />
                      <Route path="/proje/ana-sozlesme" element={<RequirePerm perm="projects.view"><AnaSozlesmePage /></RequirePerm>} />
                      <Route path="/proje/tasarim-projeler" element={<RequirePerm perm="projects.view"><TasarimVeProjelerPage /></RequirePerm>} />
                      <Route path="/tedarikci-ekstreler" element={<RequirePerm perm="contracts.view"><TedarikciEkstrerlerPage /></RequirePerm>} />
                      <Route path="/odeme-plani-onaylari" element={<RequirePerm perm="payments.approve_plan_change"><PaymentPlanApprovalsPage /></RequirePerm>} />
                      <Route path="/sabit-giderler" element={<RequirePerm perm="progress_payments.view"><SabitGiderlerPage /></RequirePerm>} />
                      <Route path="/proje/personel-puantaj" element={<RequirePerm perm="reports.view"><PersonelPuantajPage /></RequirePerm>} />
                      <Route path="/saha/tutanaklar" element={<RequirePerm perm="reports.view"><SahaTutanaklariPage /></RequirePerm>} />
                      {/* ── YAZIŞMALAR ── */}
                      <Route path="/yazismalar" element={<Navigate to="/yazismalar/gelen" replace />} />
                      <Route path="/yazismalar/gelen" element={<RequirePerm perm="correspondence.view"><GelenEvrakPage /></RequirePerm>} />
                      <Route path="/yazismalar/giden" element={<RequirePerm perm="correspondence.view"><GidenEvrakPage /></RequirePerm>} />
                      {/* ── MAKİNE & EKİPMAN ── */}
                      <Route path="/makine/araclar" element={<RequirePerm perm="projects.view"><AraclarPage /></RequirePerm>} />
                      <Route path="/makine/is-makineleri" element={<RequirePerm perm="projects.view"><IsMakineleriPage /></RequirePerm>} />
                      <Route path="/makine/ekipmanlar" element={<RequirePerm perm="projects.view"><EkipmanlarPage /></RequirePerm>} />
                    </Routes>
                  </AppShell>
                </ProjectProvider>
              </RequireAuth>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
    </ThemeProvider>
  );
}
