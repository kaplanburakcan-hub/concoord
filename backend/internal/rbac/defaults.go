// Package rbac — yetki sözlüğü, rol varsayılanları ve izin değerlendirme motoru.
//
// Bu dosya rol/izin verisinin TEK KAYNAĞIDIR: hem seed (DB'ye yazar) hem de
// otomatik yetki test matrisi buradan okur. Böylece "seed ne yazdıysa test onu
// doğrular" garantisi kod düzeyinde sağlanır (Plan Faz 1 kabul kriteri).
package rbac

// Karar önceliği (Plan §4): DENY > GRANT > rol varsayılanı.
const (
	EffectGrant = "GRANT"
	EffectDeny  = "DENY"
)

type PermissionDef struct {
	Module      string
	Action      string
	Description string
}

// Code — "modül.aksiyon" biçimindeki kanonik izin kodu.
func (p PermissionDef) Code() string { return p.Module + "." + p.Action }

// AllPermissions — Plan §4 izin sözlüğü. Yeni izin = buraya bir satır (kod değil,
// veri). Seed idempotent uygular; kaldırılan izin ayrı bir migration işidir.
var AllPermissions = []PermissionDef{
	{"projects", "view", "Proje künyesini görüntüle"},
	{"projects", "create", "Yeni proje oluştur"},
	{"projects", "edit", "Proje künyesini düzenle"},
	{"projects", "delete", "Projeyi arşivle/sil (soft delete)"},
	{"projects", "manage_members", "Proje üyelerini ve rollerini yönet"},

	{"contracts", "view", "Sözleşmeleri görüntüle"},
	{"contracts", "upload", "Sözleşme/zeyilname yükle"},
	{"contracts", "delete", "Sözleşme kaydını sil (soft delete)"},

	{"progress_payments", "view", "Hakedişleri görüntüle (metraj)"},
	{"progress_payments", "create_draft", "Hakediş taslağı oluştur"},
	{"progress_payments", "edit_draft", "Hakediş taslağını düzenle"},
	{"progress_payments", "submit", "Hakedişi onaya gönder"},
	{"progress_payments", "approve", "Hakedişi saha onayından geçir"},
	{"progress_payments", "finalize", "Hakedişi kesinleştir (kilitle)"},
	{"progress_payments", "view_financials", "Birim fiyat/tutar kolonlarını gör"},

	{"material_approvals", "view", "Malzeme onaylarını görüntüle"},
	{"material_approvals", "create", "Malzeme onay talebi (MAR) oluştur"},
	{"material_approvals", "review", "MAR incele"},
	{"material_approvals", "decide", "MAR kararı ver"},

	{"documents", "view", "Dokümanları görüntüle"},
	{"documents", "upload", "Doküman yükle / yeni versiyon"},
	{"documents", "download", "Doküman indir"},
	{"documents", "delete", "Doküman sil (soft delete)"},
	{"documents", "manage_folders", "Klasör ağacını yönet"},

	{"correspondence", "view", "Yazışmaları (gelen/giden evrak) görüntüle"},
	{"correspondence", "create", "Yazışma kaydı oluştur"},
	{"correspondence", "edit", "Yazışma kaydını düzenle"},
	{"correspondence", "delete", "Yazışma kaydını sil (soft delete)"},

	{"tasks", "view", "Görevleri görüntüle"},
	{"tasks", "create", "Görev oluştur"},
	{"tasks", "edit_own", "Kendi görevlerini düzenle"},
	{"tasks", "edit_all", "Tüm görevleri düzenle"},
	{"tasks", "assign", "Görev ata"},

	{"reports", "view", "Raporları görüntüle"},
	{"reports", "create_daily", "Günlük saha raporu gir"},
	{"reports", "generate_weekly", "Haftalık rapor üret"},
	{"reports", "view_financial_reports", "Finansal raporları gör"},

	{"procurement", "view", "Satınalma kayıtlarını görüntüle"},
	{"procurement", "create_pr", "Satınalma talebi (PR) oluştur"},
	{"procurement", "approve_pr", "PR onayla"},
	{"procurement", "manage_po", "Sipariş (PO) yönet"},
	{"procurement", "upload_delivery", "İrsaliye/teslimat yükle"},

	{"ohs", "view", "İSG kayıtlarını görüntüle"},
	{"ohs", "perform_inspection", "İSG denetimi yap"},
	{"ohs", "issue_penalty", "İSG ceza tutanağı kes"},
	{"ohs", "manage_checklists", "İSG checklist şablonlarını yönet"},

	{"admin", "manage_users", "Kullanıcıları yönet"},
	{"admin", "manage_permissions", "İzinleri ve override'ları yönet"},
	{"admin", "view_audit_log", "Denetim izini görüntüle"},
	{"admin", "manage_settings", "Sistem ayarlarını yönet"},
}

type RoleDef struct {
	Code string
	Name string
}

// Roles — Plan §4, 7 sistem rolü.
var Roles = []RoleDef{
	{"Admin", "Sistem Yöneticisi"},
	{"ProjectManager", "Proje Yöneticisi"},
	{"SiteManager", "Şantiye Şefi / İnşaat Müdürü"},
	{"SiteEngineer", "Saha Mühendisi"},
	{"SubcontractorRep", "Taşeron Temsilcisi"},
	{"Client", "İşveren / Müşteri"},
	{"ProcurementOfficer", "Satınalma Sorumlusu"},
	{"OHSExpert", "İSG Uzmanı"},
}

// roleDefaults — rol → varsayılan izin kodları. Admin tümünü alır (aşağıda
// programatik olarak doldurulur). Diğerleri açıkça listelenir; override her
// zaman bunun üzerine (DENY/GRANT) yazılabilir.
var roleDefaults = map[string][]string{
	"ProjectManager": {
		"projects.view", "projects.create", "projects.edit", "projects.manage_members",
		"contracts.view", "contracts.upload", "contracts.delete",
		"progress_payments.view", "progress_payments.create_draft", "progress_payments.edit_draft",
		"progress_payments.submit", "progress_payments.approve", "progress_payments.finalize",
		"progress_payments.view_financials",
		"material_approvals.view", "material_approvals.review", "material_approvals.decide",
		"documents.view", "documents.upload", "documents.download", "documents.delete", "documents.manage_folders",
		"correspondence.view", "correspondence.create", "correspondence.edit", "correspondence.delete",
		"tasks.view", "tasks.create", "tasks.edit_own", "tasks.edit_all", "tasks.assign",
		"reports.view", "reports.create_daily", "reports.generate_weekly", "reports.view_financial_reports",
		"procurement.view", "procurement.create_pr", "procurement.approve_pr", "procurement.manage_po", "procurement.upload_delivery",
		"ohs.view", "ohs.issue_penalty", "ohs.manage_checklists",
		"admin.view_audit_log",
	},
	"SiteManager": {
		"projects.view",
		"contracts.view",
		"progress_payments.view", "progress_payments.approve", "progress_payments.view_financials",
		"material_approvals.view", "material_approvals.create", "material_approvals.review", "material_approvals.decide",
		"documents.view", "documents.upload", "documents.download",
		"correspondence.view", "correspondence.create", "correspondence.edit",
		"tasks.view", "tasks.create", "tasks.edit_own", "tasks.edit_all", "tasks.assign",
		"reports.view", "reports.create_daily", "reports.generate_weekly", "reports.view_financial_reports",
		"procurement.view", "procurement.create_pr", "procurement.approve_pr",
		"ohs.view", "ohs.perform_inspection", "ohs.issue_penalty",
		"admin.view_audit_log",
	},
	"SiteEngineer": {
		"projects.view",
		"contracts.view",
		"progress_payments.view", "progress_payments.approve",
		"material_approvals.view", "material_approvals.create", "material_approvals.review",
		"documents.view", "documents.upload", "documents.download",
		"correspondence.view", "correspondence.create",
		"tasks.view", "tasks.create", "tasks.edit_own", "tasks.edit_all", "tasks.assign",
		"reports.view", "reports.create_daily",
		"procurement.view", "procurement.create_pr",
		"ohs.view", "ohs.perform_inspection",
		// Not: view_financials ve generate_weekly VARSAYILAN DEĞİL —
		// saha mühendisi günlük rapor girer, haftalık raporu göremez/üretemez.
	},
	"SubcontractorRep": {
		"projects.view",
		"contracts.view",
		"progress_payments.view", "progress_payments.create_draft", "progress_payments.edit_draft",
		"progress_payments.submit", "progress_payments.view_financials",
		"material_approvals.view", "material_approvals.create",
		"documents.view", "documents.upload", "documents.download",
		"correspondence.view",
		"tasks.view", "tasks.edit_own",
		"reports.view",
		"procurement.view",
		"ohs.view",
		// Satır seviyesi güvenlik: yalnızca kendi firmasının kayıtları (backend'de zorunlu).
	},
	"Client": {
		"projects.view",
		"contracts.view",
		"progress_payments.view", "progress_payments.view_financials",
		"material_approvals.view", "material_approvals.review", "material_approvals.decide",
		"documents.view", "documents.download",
		"correspondence.view",
		"tasks.view",
		"reports.view", "reports.view_financial_reports",
		"procurement.view",
		"ohs.view",
	},
	"ProcurementOfficer": {
		"projects.view",
		"contracts.view",
		"material_approvals.view",
		"documents.view", "documents.upload", "documents.download",
		"correspondence.view",
		"tasks.view", "tasks.edit_own",
		"reports.view",
		"procurement.view", "procurement.create_pr", "procurement.approve_pr",
		"procurement.manage_po", "procurement.upload_delivery",
	},
	"OHSExpert": {
		"projects.view",
		"material_approvals.view",
		"documents.view", "documents.upload", "documents.download",
		"correspondence.view",
		"tasks.view", "tasks.create", "tasks.edit_own", "tasks.assign",
		"reports.view",
		"ohs.view", "ohs.perform_inspection", "ohs.issue_penalty", "ohs.manage_checklists",
	},
}

// RoleDefaults — rol koduna göre varsayılan izin kodları kümesi.
// Admin her zaman tüm izinleri alır (sözlük büyüdükçe otomatik kapsar).
func RoleDefaults(roleCode string) []string {
	if roleCode == "Admin" {
		out := make([]string, 0, len(AllPermissions))
		for _, p := range AllPermissions {
			out = append(out, p.Code())
		}
		return out
	}
	return roleDefaults[roleCode]
}

// RoleHasDefault — rol varsayılan olarak izne sahip mi?
func RoleHasDefault(roleCode, permCode string) bool {
	for _, c := range RoleDefaults(roleCode) {
		if c == permCode {
			return true
		}
	}
	return false
}
