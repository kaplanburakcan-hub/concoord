// Package server — HTTP router, sağlık uçları ve Faz 1 kimlik/yetki rotaları.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/admin"
	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/config"
	"github.com/ipks/ipks/backend/internal/contracts"
	"github.com/ipks/ipks/backend/internal/design"
	"github.com/ipks/ipks/backend/internal/machines"
	"github.com/ipks/ipks/backend/internal/meetings"
	"github.com/ipks/ipks/backend/internal/personnel"
	"github.com/ipks/ipks/backend/internal/statements"
	"github.com/ipks/ipks/backend/internal/survey"
	"github.com/ipks/ipks/backend/internal/warehouse"
	"github.com/ipks/ipks/backend/internal/dashboard"
	"github.com/ipks/ipks/backend/internal/documents"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/materials"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/ohs"
	"github.com/ipks/ipks/backend/internal/payments"
	"github.com/ipks/ipks/backend/internal/procurement"
	"github.com/ipks/ipks/backend/internal/projects"
	"github.com/ipks/ipks/backend/internal/rbac"
	"github.com/ipks/ipks/backend/internal/reports"
	"github.com/ipks/ipks/backend/internal/storage"
	"github.com/ipks/ipks/backend/internal/tasks"
)

func New(cfg *config.Config, pool *pgxpool.Pool, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// Faz 10 — prod hata bildirimi (5xx → webhook + log).
	errRep := httpx.NewErrorReporter(cfg.ErrorWebhookURL, cfg.Env, log)

	// Middleware zinciri (dıştan içe):
	//   request-id → güvenlik başlıkları → CORS → hız sınırı → access log →
	//   5xx bildirim → panic recover → audit meta
	// Faz 10 eklemeleri (güvenlik başlıkları, CORS, hız sınırı, hata bildirimi)
	// bilinçle en dışta: yetkisiz/aşırı istek handler'lara ve DB'ye ulaşmadan
	// kesilir.
	r.Use(httpx.RequestID)
	r.Use(httpx.SecureHeaders)
	r.Use(httpx.CORS(cfg.CORSOrigins))
	r.Use(httpx.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst))
	r.Use(httpx.AccessLog(log))
	r.Use(httpx.ErrorNotify(errRep))
	r.Use(httpx.Recover(log))
	r.Use(audit.Middleware)

	// --- Faz 1 bağımlılıkları ---
	signer := auth.NewTokenSigner(cfg.JWTSecret)
	eval := rbac.NewEvaluator(pool)
	recorder := audit.NewRecorder(pool, log)
	authSvc := auth.NewService(pool, signer, cfg.AccessTTL, cfg.RefreshTTL, cfg.ResetTTL)
	authH := auth.NewHandler(authSvc, eval, log, cfg.Env == "development")
	mw := auth.NewMiddleware(signer, eval)
	adminH := admin.NewHandler(pool, eval, recorder, log)

	// --- Faz 2 bağımlılıkları ---
	store := storage.New(storage.Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
		UseSSL:         cfg.S3UseSSL,
		ClamdAddr:      cfg.ClamdAddr, // Faz 10: opsiyonel antivirüs
	})
	projectH := projects.NewHandler(pool, eval, recorder, log)
	docH := documents.NewHandler(pool, store, recorder, log)

	// --- Faz 3 bağımlılıkları (taşeron + hakediş finansal çekirdeği) ---
	payH := payments.NewHandler(pool, eval, recorder, log)

	// --- Faz 4 bağımlılıkları (görev yönetimi + merkezi bildirim motoru) ---
	notifySvc := notify.NewService(pool, log)
	notifyH := notify.NewHandler(pool, log)
	taskH := tasks.NewHandler(pool, eval, recorder, notifySvc, log)

	// --- Faz 5 bağımlılıkları (malzeme onay süreci / MAR) ---
	marH := materials.NewHandler(pool, eval, recorder, notifySvc, log)

	// --- Faz 6 bağımlılıkları (günlük/haftalık saha raporlama) ---
	reportH := reports.NewHandler(pool, eval, recorder, notifySvc, store, log,
		cfg.WeatherEnabled, cfg.WeatherAPIURL)

	// --- Faz 7 bağımlılıkları (satınalma ve tedarik zinciri) ---
	procH := procurement.NewHandler(pool, eval, recorder, notifySvc, log)

	// --- Faz 8 bağımlılıkları (İSG: checklist, denetim, bulgu, ceza otomasyonu) ---
	ohsH := ohs.NewHandler(pool, eval, recorder, notifySvc, store, log)

	// --- Faz 9 bağımlılıkları (dashboard, EVM, aylık yönetim raporu) ---
	dashH := dashboard.NewHandler(pool, eval, recorder, notifySvc, store, log)

	// --- Faz 18: Ana Sözleşme ---
	contractH := contracts.NewHandler(pool)

	// --- Faz 19: Proje Keşfi ---
	surveyH := survey.NewHandler(pool)

	// --- Faz 20: Tasarım ve Projeler ---
	designH := design.NewHandler(pool)
	warehouseH := warehouse.NewHandler(pool)
	meetingsH := meetings.NewHandler(pool)

	// --- Faz 21: Personel & Puantaj ---
	personnelH := personnel.NewHandler(pool)

	// --- Faz 22: Tedarikçi Ekstreler ---
	statementsH := statements.NewHandler(pool)

	// --- Faz 26: Makine & Ekipman ---
	machinesH := machines.NewHandler(pool)

	// Liveness — süreç ayakta mı?
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Readiness — bağımlılıklar hazır mı? (docker healthcheck bunu kullanır)
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.Error(w, req, http.StatusServiceUnavailable, httpx.CodeInternal,
				"Veritabanına ulaşılamıyor.", nil)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/meta", func(w http.ResponseWriter, req *http.Request) {
			httpx.JSON(w, http.StatusOK, map[string]string{
				"name":    "İPKS API",
				"version": "1.0.0",
				"env":     cfg.Env,
			})
		})

		// ---- Herkese açık kimlik uçları ----
		// Faz 10: kaba kuvvet parola denemesine karşı IP başına sıkı sabit
		// pencere sınırı yalnızca kimlik uçlarında uygulanır.
		loginLimit := httpx.LoginRateLimit(cfg.LoginRateLimitPerMin)
		api.Route("/auth", func(a chi.Router) {
			a.With(loginLimit).Post("/login", authH.Login)
			a.With(loginLimit).Post("/refresh", authH.Refresh)
			a.Post("/logout", authH.Logout)
			a.With(loginLimit).Post("/password/forgot", authH.ForgotPassword)
			a.With(loginLimit).Post("/password/reset", authH.ResetPassword)

			// ---- Kimlik doğrulaması gerektiren uçlar ----
			a.Group(func(pr chi.Router) {
				pr.Use(mw.Authenticate)
				pr.Get("/me", authH.Me)
				pr.Post("/password/change", authH.ChangePassword)
			})
		})

		// ---- Admin Paneli v1 (tümü kimlik doğrulamalı + izin korumalı) ----
		api.Route("/admin", func(ad chi.Router) {
			ad.Use(mw.Authenticate)

			// Kullanıcı yönetimi
			ad.With(mw.RequirePermission("admin.manage_users")).Group(func(u chi.Router) {
				u.Get("/users", adminH.ListUsers)
				u.Post("/users", adminH.CreateUser)
				u.Get("/users/{id}", adminH.GetUser)
				u.Patch("/users/{id}", adminH.UpdateUser)
				u.Delete("/users/{id}", adminH.DeleteUser)
			})

			// İzin/rol yönetimi (matris + override)
			ad.With(mw.RequirePermission("admin.manage_permissions")).Group(func(p chi.Router) {
				p.Get("/roles", adminH.ListRoles)
				p.Get("/permissions", adminH.ListPermissions)
				p.Get("/users/{id}/permissions", adminH.GetUserMatrix)
				p.Put("/users/{id}/permissions/{code}", adminH.SetOverride)
			})

			// Proje üyeliği = rol atama (proje kapsamlı izin: projects.manage_members)
			ad.With(mw.RequirePermission("projects.manage_members")).Get("/projects/{projectID}/members", adminH.ListMembers)
			ad.With(mw.RequirePermission("projects.manage_members")).Post("/projects/{projectID}/members", adminH.AddMember)
			ad.With(mw.RequirePermission("projects.manage_members")).Delete("/projects/{projectID}/members/{userID}", adminH.RemoveMember)

			// Denetim izi görüntüleyici
			ad.With(mw.RequirePermission("admin.view_audit_log")).Get("/audit-logs", adminH.ListAuditLogs)
		})

		// ---- Faz 2: Proje çekirdeği + Doküman motoru (kimlik doğrulamalı) ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Proje listesi (seçici kaynağı) + oluşturma. Liste kişiye özel
			// filtrelenir (üyelik/global), bu yüzden yalnızca kimlik yeter.
			pr.Get("/projects", projectH.ListProjects)
			pr.With(mw.RequirePermission("projects.create")).Post("/projects", projectH.CreateProject)

			// Tekil proje (proje kapsamlı izinler; projectID URL'den çözülür).
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}", projectH.GetProject)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}", projectH.UpdateProject)
			pr.With(mw.RequirePermission("projects.delete")).Delete("/projects/{projectID}", projectH.DeleteProject)

			// Milestone'lar (görüntüleme: projects.view; değişiklik: projects.edit)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/milestones", projectH.ListMilestones)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/milestones", projectH.CreateMilestone)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/milestones/{id}", projectH.UpdateMilestone)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/milestones/{id}", projectH.DeleteMilestone)

			// Klasör ağacı
			pr.With(mw.RequirePermission("documents.view")).Get("/projects/{projectID}/folders", docH.ListFolders)
			pr.With(mw.RequirePermission("documents.manage_folders")).Post("/projects/{projectID}/folders", docH.CreateFolder)
			pr.With(mw.RequirePermission("documents.manage_folders")).Patch("/projects/{projectID}/folders/{id}", docH.UpdateFolder)
			pr.With(mw.RequirePermission("documents.manage_folders")).Delete("/projects/{projectID}/folders/{id}", docH.DeleteFolder)

			// Dokümanlar + versiyonlama
			pr.With(mw.RequirePermission("documents.view")).Get("/projects/{projectID}/documents", docH.ListDocuments)
			pr.With(mw.RequirePermission("documents.upload")).Post("/projects/{projectID}/documents", docH.CreateDocument)
			pr.With(mw.RequirePermission("documents.view")).Get("/projects/{projectID}/documents/{id}", docH.GetDocument)
			pr.With(mw.RequirePermission("documents.upload")).Patch("/projects/{projectID}/documents/{id}", docH.UpdateDocument)
			pr.With(mw.RequirePermission("documents.delete")).Delete("/projects/{projectID}/documents/{id}", docH.DeleteDocument)
			pr.With(mw.RequirePermission("documents.upload")).Post("/projects/{projectID}/documents/{id}/versions", docH.UploadVersion)
			pr.With(mw.RequirePermission("documents.download")).Get("/projects/{projectID}/documents/{id}/versions/{versionNo}/download", docH.DownloadVersion)
		})

		// ---- Faz 3: Taşeron + Hakediş (finansal çekirdek) ----
		// Taşeron/sözleşme/birim-fiyat CRUD'u contracts.* izinleriyle korunur
		// (ayrı modül izni yok); hakediş iş akışı progress_payments.* ile.
		// Satır seviyesi güvenlik ve view_financials maskeleme handler içinde.
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Taşeron kartları
			pr.With(mw.RequirePermission("contracts.view")).Get("/projects/{projectID}/subcontractors", payH.ListSubcontractors)
			pr.With(mw.RequirePermission("contracts.upload")).Post("/projects/{projectID}/subcontractors", payH.CreateSubcontractor)
			pr.With(mw.RequirePermission("contracts.view")).Get("/projects/{projectID}/subcontractors/{id}", payH.GetSubcontractor)
			pr.With(mw.RequirePermission("contracts.upload")).Patch("/projects/{projectID}/subcontractors/{id}", payH.UpdateSubcontractor)
			pr.With(mw.RequirePermission("contracts.delete")).Delete("/projects/{projectID}/subcontractors/{id}", payH.DeleteSubcontractor)

			// Birim fiyat cetveli (work_items) + Excel/CSV içe aktarma
			pr.With(mw.RequirePermission("contracts.view")).Get("/projects/{projectID}/subcontractors/{subID}/work-items", payH.ListWorkItems)
			pr.With(mw.RequirePermission("contracts.upload")).Post("/projects/{projectID}/subcontractors/{subID}/work-items", payH.CreateWorkItem)
			pr.With(mw.RequirePermission("contracts.upload")).Post("/projects/{projectID}/subcontractors/{subID}/work-items/import", payH.ImportWorkItems)
			pr.With(mw.RequirePermission("contracts.upload")).Patch("/projects/{projectID}/subcontractors/{subID}/work-items/{id}", payH.UpdateWorkItem)
			pr.With(mw.RequirePermission("contracts.delete")).Delete("/projects/{projectID}/subcontractors/{subID}/work-items/{id}", payH.DeleteWorkItem)

			// Sözleşmeler (alt sözleşme arşivi)
			pr.With(mw.RequirePermission("contracts.view")).Get("/projects/{projectID}/contracts", payH.ListContracts)
			pr.With(mw.RequirePermission("contracts.upload")).Post("/projects/{projectID}/contracts", payH.CreateContract)
			pr.With(mw.RequirePermission("contracts.upload")).Patch("/projects/{projectID}/contracts/{id}", payH.UpdateContract)
			pr.With(mw.RequirePermission("contracts.delete")).Delete("/projects/{projectID}/contracts/{id}", payH.DeleteContract)

			// Hakedişler + iş akışı
			// Faz 11 — kesinti kataloğu (grup + kalem listesi). Proje bağımsızdır.
			pr.With(mw.RequirePermission("progress_payments.view")).Get("/deduction-catalog", payH.DeductionCatalog)
			// Faz 11 — çok adımlı onay zinciri. Adım izni zincir verisinden gelir,
			// bu yüzden rota düzeyinde yalnızca görüntüleme izni aranır.
			// Faz 11 — teminat bakiyesi ve iade akışı
			pr.With(mw.RequirePermission("progress_payments.view_financials")).Get("/projects/{projectID}/retention", payH.RetentionBalances)
			pr.With(mw.RequirePermission("progress_payments.view_financials")).Get("/projects/{projectID}/subcontractors/{subID}/refunds", payH.ListRefunds)
			pr.With(mw.RequirePermission("progress_payments.finalize")).Post("/projects/{projectID}/subcontractors/{subID}/refunds", payH.CreateRefund)
			pr.With(mw.RequirePermission("progress_payments.view")).Get("/payments/{id}/approvals", payH.ApprovalChain)
			pr.With(mw.RequirePermission("progress_payments.view")).Post("/payments/{id}/approvals", payH.ApproveStep)
			pr.With(mw.RequirePermission("progress_payments.view")).Get("/projects/{projectID}/payments", payH.ListPayments)
			pr.With(mw.RequirePermission("progress_payments.create_draft")).Post("/projects/{projectID}/payments", payH.CreatePayment)
			pr.With(mw.RequirePermission("progress_payments.view")).Get("/projects/{projectID}/payments/{id}", payH.GetPayment)
			pr.With(mw.RequirePermission("progress_payments.edit_draft")).Patch("/projects/{projectID}/payments/{id}", payH.UpdatePaymentDraft)
			pr.With(mw.RequirePermission("progress_payments.submit")).Post("/projects/{projectID}/payments/{id}/submit", payH.SubmitPayment)
			pr.With(mw.RequirePermission("progress_payments.approve")).Post("/projects/{projectID}/payments/{id}/approve", payH.ApprovePayment)
			pr.With(mw.RequirePermission("progress_payments.approve")).Post("/projects/{projectID}/payments/{id}/reject", payH.RejectPayment)
			pr.With(mw.RequirePermission("progress_payments.finalize")).Post("/projects/{projectID}/payments/{id}/finalize", payH.FinalizePayment)
			pr.With(mw.RequirePermission("progress_payments.view_financials")).Get("/projects/{projectID}/payments/{id}/summary.pdf", payH.SummaryPDF)
		})

		// ---- Faz 5: Malzeme Onay Süreci (Submittals / MAR) ----
		// Satır seviyesi güvenlik (taşeron kapsamı) ve Client kısıtlı görünüm
		// ("kendisine sunulanlar") handler içinde ZORUNLU uygulanır.
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			pr.With(mw.RequirePermission("material_approvals.view")).Get("/projects/{projectID}/materials", marH.ListMARs)
			pr.With(mw.RequirePermission("material_approvals.create")).Post("/projects/{projectID}/materials", marH.CreateMAR)
			pr.With(mw.RequirePermission("material_approvals.view")).Get("/projects/{projectID}/materials/{id}", marH.GetMAR)
			pr.With(mw.RequirePermission("material_approvals.create")).Patch("/projects/{projectID}/materials/{id}", marH.UpdateMAR)
			pr.With(mw.RequirePermission("material_approvals.review")).Post("/projects/{projectID}/materials/{id}/review", marH.ReviewMAR)
			pr.With(mw.RequirePermission("material_approvals.decide")).Post("/projects/{projectID}/materials/{id}/decide", marH.DecideMAR)

			// MAR kayıt defteri dışa aktarımı (CSV; kapsam filtreli)
			pr.With(mw.RequirePermission("material_approvals.view")).Get("/projects/{projectID}/materials/register.csv", marH.ExportRegister)
		})

		// ---- Faz 6: Günlük/Haftalık Saha Raporlama ----
		// Günlük rapor: Draft→Submitted; Submitted DB kilidiyle değişmez,
		// düzeltme /revise ile yeni revizyon açar (Plan Faz 6).
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/daily-report-context", reportH.DailyContext)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/daily-reports", reportH.ListDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Post("/projects/{projectID}/daily-reports", reportH.CreateDaily)
			// Hava durumu ön doldurma (opsiyonel; IPKS_WEATHER_ENABLED)
			pr.With(mw.RequirePermission("reports.create_daily")).Get("/projects/{projectID}/daily-reports/weather", reportH.WeatherPrefill)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/daily-reports/{id}", reportH.GetDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Put("/projects/{projectID}/daily-reports/{id}", reportH.UpdateDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Post("/projects/{projectID}/daily-reports/{id}/submit", reportH.SubmitDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Post("/projects/{projectID}/daily-reports/{id}/revise", reportH.ReviseDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Delete("/projects/{projectID}/daily-reports/{id}", reportH.DeleteDaily)
			pr.With(mw.RequirePermission("reports.create_daily")).Post("/projects/{projectID}/daily-reports/{id}/cover-photo", reportH.SetCoverPhoto)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/daily-reports/{id}/cover-photo", reportH.GetCoverPhoto)

			// Haftalık rapor: snapshot API'de dondurulur, PDF worker'da üretilir.
			pr.With(mw.RequirePermission("reports.generate_weekly")).Get("/projects/{projectID}/weekly-reports", reportH.ListWeekly)
			pr.With(mw.RequirePermission("reports.generate_weekly")).Post("/projects/{projectID}/weekly-reports", reportH.GenerateWeekly)
			pr.With(mw.RequirePermission("reports.generate_weekly")).Get("/projects/{projectID}/weekly-reports/{id}", reportH.GetWeekly)
			pr.With(mw.RequirePermission("reports.generate_weekly")).Get("/projects/{projectID}/weekly-reports/{id}/download", reportH.DownloadWeekly)
		})

		// ---- Faz 7: Satınalma ve Tedarik Zinciri ----
		// Zincir: PR (procurement.create_pr → approve_pr) → PO (manage_po) →
		// teslimat (upload_delivery). Pano herkese açık görünüm (view).
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Tedarik durum panosu (geciken PR/PO vurgulu)
			pr.With(mw.RequirePermission("procurement.view")).Get("/projects/{projectID}/procurement/board", procH.Board)

			// Satınalma talepleri (PR)
			pr.With(mw.RequirePermission("procurement.view")).Get("/projects/{projectID}/purchase-requests", procH.ListPRs)
			pr.With(mw.RequirePermission("procurement.create_pr")).Post("/projects/{projectID}/purchase-requests", procH.CreatePR)
			pr.With(mw.RequirePermission("procurement.view")).Get("/projects/{projectID}/purchase-requests/{id}", procH.GetPR)
			pr.With(mw.RequirePermission("procurement.create_pr")).Patch("/projects/{projectID}/purchase-requests/{id}", procH.UpdatePR)
			pr.With(mw.RequirePermission("procurement.create_pr")).Delete("/projects/{projectID}/purchase-requests/{id}", procH.DeletePR)
			pr.With(mw.RequirePermission("procurement.create_pr")).Post("/projects/{projectID}/purchase-requests/{id}/submit", procH.SubmitPR)
			pr.With(mw.RequirePermission("procurement.approve_pr")).Post("/projects/{projectID}/purchase-requests/{id}/approve", procH.ApprovePR)
			pr.With(mw.RequirePermission("procurement.approve_pr")).Post("/projects/{projectID}/purchase-requests/{id}/reject", procH.RejectPR)
			pr.With(mw.RequirePermission("procurement.manage_po")).Post("/projects/{projectID}/purchase-requests/{id}/convert", procH.ConvertPR)

			// Siparişler (PO) + teslimatlar
			pr.With(mw.RequirePermission("procurement.view")).Get("/projects/{projectID}/purchase-orders", procH.ListPOs)
			pr.With(mw.RequirePermission("procurement.manage_po")).Post("/projects/{projectID}/purchase-orders", procH.CreatePO)
			pr.With(mw.RequirePermission("procurement.view")).Get("/projects/{projectID}/purchase-orders/{id}", procH.GetPO)
			pr.With(mw.RequirePermission("procurement.manage_po")).Patch("/projects/{projectID}/purchase-orders/{id}", procH.UpdatePO)
			pr.With(mw.RequirePermission("procurement.manage_po")).Post("/projects/{projectID}/purchase-orders/{id}/cancel", procH.CancelPO)
			pr.With(mw.RequirePermission("procurement.upload_delivery")).Post("/projects/{projectID}/purchase-orders/{id}/deliveries", procH.AddDelivery)
		})


		// ---- Faz 8: İSG (checklist şablonları, denetimler, bulgular, cezalar) ----
		// Ceza otomasyonu: kesme isteği içinde PDF üretilir + bildirim yayılır;
		// para cezası taşeronun sonraki taslak hakedişinde kesinti önerisi olur.
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Checklist şablonları — global (proje bağımsız) tanımlar.
			pr.With(mw.RequirePermission("ohs.view")).Get("/ohs/checklist-templates", ohsH.ListTemplates)
			pr.With(mw.RequirePermission("ohs.manage_checklists")).Post("/ohs/checklist-templates", ohsH.CreateTemplate)
			pr.With(mw.RequirePermission("ohs.manage_checklists")).Patch("/ohs/checklist-templates/{id}", ohsH.UpdateTemplate)
			pr.With(mw.RequirePermission("ohs.manage_checklists")).Delete("/ohs/checklist-templates/{id}", ohsH.DeleteTemplate)

			// Denetimler (mobil; offline kuyruk POST'u da buraya düşer).
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/inspections", ohsH.ListInspections)
			pr.With(mw.RequirePermission("ohs.perform_inspection")).Post("/projects/{projectID}/ohs/inspections", ohsH.CreateInspection)
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/inspections/{id}", ohsH.GetInspection)

			// Bulgular — yaşam döngüsü Open→InProgress→Closed, termin takibi.
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/findings", ohsH.ListFindings)
			pr.With(mw.RequirePermission("ohs.perform_inspection")).Post("/projects/{projectID}/ohs/findings", ohsH.CreateFinding)
			pr.With(mw.RequirePermission("ohs.view")).Post("/projects/{projectID}/ohs/findings/{id}/start", ohsH.StartFinding)
			pr.With(mw.RequirePermission("ohs.perform_inspection")).Post("/projects/{projectID}/ohs/findings/{id}/close", ohsH.CloseFinding)

			// Ceza tutanakları.
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/penalties", ohsH.ListPenalties)
			pr.With(mw.RequirePermission("ohs.issue_penalty")).Post("/projects/{projectID}/ohs/penalties", ohsH.CreatePenalty)
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/penalties/{id}", ohsH.GetPenalty)
			pr.With(mw.RequirePermission("ohs.view")).Get("/projects/{projectID}/ohs/penalties/{id}/pdf", ohsH.DownloadPenaltyPDF)
			pr.With(mw.RequirePermission("ohs.view")).Post("/projects/{projectID}/ohs/penalties/{id}/acknowledge", ohsH.AcknowledgePenalty)
		})

		// ---- Faz 9: Dashboard, EVM ve yönetim raporlaması ----
		// Finansal görünürlük ayrımı (Plan §4): dashboard herkese açılır ama
		// EVM/tutar bloğu handler içinde reports.view_financial_reports ile
		// süzülür; aylık rapor uçları izni doğrudan route'ta zorunlu kılar.
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Portföy görünümü (Plan §3) — projeler arası özet kartlar.
			pr.With(mw.RequirePermission("projects.view")).Get("/portfolio", dashH.Portfolio)

			// Rol duyarlı proje dashboard'u.
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/dashboard", dashH.ProjectDashboard)

			// PV aylık dağılım girişi (EVM S-eğrisini zenginleştirir).
			pr.With(mw.RequirePermission("reports.view_financial_reports")).Get("/projects/{projectID}/pv-plan", dashH.GetPVPlan)
			pr.With(mw.RequirePermission("projects.edit")).Put("/projects/{projectID}/pv-plan", dashH.PutPVPlan)

			// Kontrol eşikleri (uyarı parametreleri — veri olarak).
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/control-settings", dashH.GetControlSettings)
			pr.With(mw.RequirePermission("projects.edit")).Put("/projects/{projectID}/control-settings", dashH.PutControlSettings)

			// Aylık yönetim raporu (EVM + finansal + İSG + tedarik).
			pr.With(mw.RequirePermission("reports.view_financial_reports")).Get("/projects/{projectID}/monthly-reports", dashH.ListMonthly)
			pr.With(mw.RequirePermission("reports.view_financial_reports"),
				mw.RequirePermission("reports.generate_weekly")).Post("/projects/{projectID}/monthly-reports", dashH.GenerateMonthly)
			pr.With(mw.RequirePermission("reports.view_financial_reports")).Get("/projects/{projectID}/monthly-reports/{id}", dashH.GetMonthly)
			pr.With(mw.RequirePermission("reports.view_financial_reports")).Get("/projects/{projectID}/monthly-reports/{id}/download", dashH.DownloadMonthly)
		})

		// ---- Faz 18: Ana Sözleşme (işveren-yüklenici) ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/main-contract", contractH.Get)
			pr.With(mw.RequirePermission("projects.edit")).Put("/projects/{projectID}/main-contract", contractH.Upsert)
		})

		// ---- Faz 19: Proje Keşfi ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/survey-items", surveyH.List)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/survey-items", surveyH.Create)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/survey-items/{id}", surveyH.Update)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/survey-items/{id}", surveyH.Delete)
		})

		// ---- Faz 20: Tasarım ve Projeler ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/design-docs", designH.List)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/design-docs", designH.Create)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/design-docs/{id}", designH.Update)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/design-docs/{id}", designH.Delete)
		})

		// ---- Faz 21: Personel & Puantaj ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/personnel", personnelH.ListPersonel)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/personnel", personnelH.CreatePersonel)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/personnel/{id}", personnelH.UpdatePersonel)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/personnel/{id}", personnelH.DeletePersonel)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/puantaj", personnelH.GetPuantaj)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/puantaj", personnelH.UpsertPuantaj)
		})

		// ---- Faz 22: Tedarikçi Ekstreler ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("contracts.view")).Get("/projects/{projectID}/supplier-statements", statementsH.List)
			pr.With(mw.RequirePermission("contracts.upload")).Post("/projects/{projectID}/supplier-statements", statementsH.Create)
			pr.With(mw.RequirePermission("contracts.upload")).Patch("/projects/{projectID}/supplier-statements/{id}", statementsH.Update)
			pr.With(mw.RequirePermission("contracts.delete")).Delete("/projects/{projectID}/supplier-statements/{id}", statementsH.Delete)
		})

		// ---- Faz 4: Görevler (Kanban) + Bildirimler ----
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)

			// Kanban panosu ve görev CRUD'u. edit_own/edit_all ayrımı ile
			// atama izni (tasks.assign) handler içinde ayrıca denetlenir.
			pr.With(mw.RequirePermission("tasks.view")).Get("/projects/{projectID}/tasks", taskH.ListTasks)
			pr.With(mw.RequirePermission("tasks.create")).Post("/projects/{projectID}/tasks", taskH.CreateTask)
			pr.With(mw.RequirePermission("tasks.view")).Get("/projects/{projectID}/tasks/{id}", taskH.GetTask)
			pr.With(mw.RequirePermission("tasks.view")).Patch("/projects/{projectID}/tasks/{id}", taskH.UpdateTask)
			pr.With(mw.RequirePermission("tasks.view")).Post("/projects/{projectID}/tasks/{id}/move", taskH.MoveTask)
			pr.With(mw.RequirePermission("tasks.view")).Delete("/projects/{projectID}/tasks/{id}", taskH.DeleteTask)

			// Yorumlar + @mention
			pr.With(mw.RequirePermission("tasks.view")).Get("/projects/{projectID}/tasks/{id}/comments", taskH.ListComments)
			pr.With(mw.RequirePermission("tasks.view")).Post("/projects/{projectID}/tasks/{id}/comments", taskH.CreateComment)

			// Atama ve @mention için proje üyeleri
			pr.With(mw.RequirePermission("tasks.view")).Get("/projects/{projectID}/assignable-users", taskH.AssignableUsers)

			// Bildirimler — kişiye özel; ayrı izin gerekmez (user_id = oturum).
			pr.Get("/notifications", notifyH.List)
			pr.Post("/notifications/read-all", notifyH.MarkAllRead)
			pr.Post("/notifications/{id}/read", notifyH.MarkRead)
			pr.Get("/notification-preferences", notifyH.GetPreferences)
			pr.Put("/notification-preferences", notifyH.SetPreference)
		})

		// Faz 24 — Depo
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/warehouse-items", warehouseH.ListItems)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/warehouse-items", warehouseH.CreateItem)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/warehouse-items/{id}", warehouseH.UpdateItem)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/warehouse-items/{id}", warehouseH.DeleteItem)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/warehouse-movements", warehouseH.ListMovements)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/warehouse-movements", warehouseH.CreateMovement)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/warehouse-movements/{id}", warehouseH.DeleteMovement)
		})

		// Faz 25 — Toplantı Tutanakları
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("reports.view")).Get("/projects/{projectID}/meetings", meetingsH.List)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/meetings", meetingsH.Create)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/meetings/{id}", meetingsH.Update)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/meetings/{id}", meetingsH.Delete)
		})

		// Faz 26 — Makine & Ekipman
		api.Group(func(pr chi.Router) {
			pr.Use(mw.Authenticate)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/machines", machinesH.ListMachines)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/machines", machinesH.CreateMachine)
			pr.With(mw.RequirePermission("projects.edit")).Patch("/projects/{projectID}/machines/{id}", machinesH.UpdateMachine)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/machines/{id}", machinesH.DeleteMachine)
			pr.With(mw.RequirePermission("projects.view")).Get("/projects/{projectID}/machines/{machineID}/logs", machinesH.ListLogs)
			pr.With(mw.RequirePermission("projects.edit")).Post("/projects/{projectID}/machines/{machineID}/logs", machinesH.CreateLog)
			pr.With(mw.RequirePermission("projects.edit")).Delete("/projects/{projectID}/machines/{machineID}/logs/{id}", machinesH.DeleteLog)
		})

		api.NotFound(func(w http.ResponseWriter, req *http.Request) {
			httpx.Error(w, req, http.StatusNotFound, httpx.CodeNotFound, "Kaynak bulunamadı.", nil)
		})
	})

	return r
}
