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
import PurchaseRequestDetailPage from "./pages/procurement/PurchaseRequestDetailPage";
import PurchaseOrdersPage from "./pages/procurement/PurchaseOrdersPage";
import PurchaseOrderDetailPage from "./pages/procurement/PurchaseOrderDetailPage";
import NotificationPrefsPage from "./pages/notifications/NotificationPrefsPage";
import FindingsPage from "./pages/ohs/FindingsPage";
import InspectionsPage from "./pages/ohs/InspectionsPage";
import InspectionFormPage from "./pages/ohs/InspectionFormPage";
import PenaltiesPage from "./pages/ohs/PenaltiesPage";
import ChecklistTemplatesPage from "./pages/ohs/ChecklistTemplatesPage";

// Faz 1 kabuğu + Faz 2 proje/doküman modülleri. ProjectProvider AuthProvider'ın
// içinde: seçili proje değişince izinler o kapsamda yeniden çözülür.
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
                      {/* Faz 9: portföy görünümü */}
                      <Route
                        path="/portfoy"
                        element={
                          <RequirePerm perm="projects.view">
                            <PortfolioPage />
                          </RequirePerm>
                        }
                      />
                      {/* Faz 9: aylık yönetim raporu (finansal izin gerektirir) */}
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
                          <RequirePerm perm="reports.view">
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
