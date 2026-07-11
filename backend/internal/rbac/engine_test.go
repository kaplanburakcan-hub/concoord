package rbac

import "testing"

// Karar önceliği: DENY > GRANT > rol varsayılanı (Plan §4).
func TestDecidePrecedence(t *testing.T) {
	cases := []struct {
		name    string
		effects []string
		roleHas bool
		want    bool
	}{
		{"rol yok, override yok", nil, false, false},
		{"rol var, override yok", nil, true, true},
		{"GRANT rolü ezer (rol yok)", []string{"GRANT"}, false, true},
		{"DENY rolü ezer (rol var)", []string{"DENY"}, true, false},
		{"DENY, GRANT'ı ezer", []string{"GRANT", "DENY"}, true, false},
		{"DENY, GRANT'ı ezer (sıra farklı)", []string{"DENY", "GRANT"}, false, false},
		{"global GRANT + proje yok rol", []string{"GRANT"}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.effects, c.roleHas); got != c.want {
				t.Fatalf("Decide(%v,%v)=%v, beklenen %v", c.effects, c.roleHas, got, c.want)
			}
		})
	}
}

// Sözlük tutarlılığı: kod benzersiz, module.action ile eşleşiyor.
func TestPermissionDictionaryConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllPermissions {
		code := p.Code()
		if seen[code] {
			t.Fatalf("tekrarlı izin kodu: %s", code)
		}
		seen[code] = true
		if code != p.Module+"."+p.Action {
			t.Fatalf("kod türetme hatası: %s", code)
		}
	}
}

// Rol varsayılanları sözlükteki gerçek izinlere işaret etmeli (typo koruması).
func TestRoleDefaultsReferenceRealPermissions(t *testing.T) {
	valid := map[string]bool{}
	for _, p := range AllPermissions {
		valid[p.Code()] = true
	}
	for _, r := range Roles {
		for _, code := range RoleDefaults(r.Code) {
			if !valid[code] {
				t.Fatalf("rol %s tanımsız izne bağlı: %s", r.Code, code)
			}
		}
	}
}

// Admin tüm izinlere sahip olmalı.
func TestAdminHasEverything(t *testing.T) {
	for _, p := range AllPermissions {
		if !RoleHasDefault("Admin", p.Code()) {
			t.Fatalf("Admin %s iznine sahip olmalı", p.Code())
		}
	}
}

// ---------------------------------------------------------------------------
// OTOMATİK YETKİ TEST MATRİSİ (Plan Faz 1 kabul kriteri)
// Kritik endpoint → gerektirdiği izin. Her rol × endpoint için beklenen sonuç,
// rol varsayılanlarından türetilir; böylece seed'in yazacağı yetki dağılımı
// koddan bağımsız bir referansla doğrulanır.
// ---------------------------------------------------------------------------

var criticalEndpoints = map[string]string{
	"POST   /admin/users":                          "admin.manage_users",
	"PUT    /admin/users/{id}/permissions/{code}":  "admin.manage_permissions",
	"GET    /admin/audit-logs":                     "admin.view_audit_log",
	"POST   /projects/{id}/progress-payments":      "progress_payments.create_draft",
	"POST   /progress-payments/{id}/approve":       "progress_payments.approve",
	"POST   /progress-payments/{id}/finalize":      "progress_payments.finalize",
	"GET    /progress-payments/{id}/financials":    "progress_payments.view_financials",
	"POST   /ohs/penalties":                        "ohs.issue_penalty",
	"POST   /material-approvals/{id}/decide":       "material_approvals.decide",
	"POST   /procurement/purchase-orders":          "procurement.manage_po",
}

func TestRoleEndpointAuthorizationMatrix(t *testing.T) {
	// Beklenen izin dağılımı (seed ile birebir olmalı). Her hücre açık yazılır;
	// böylece yanlışlıkla genişleyen bir yetki testi kırar.
	expect := map[string]map[string]bool{
		"progress_payments.finalize": {
			"Admin": true, "ProjectManager": true,
			"SiteEngineer": false, "SubcontractorRep": false, "Client": false,
			"ProcurementOfficer": false, "OHSExpert": false,
		},
		"progress_payments.approve": {
			"Admin": true, "ProjectManager": true, "SiteEngineer": true,
			"SubcontractorRep": false, "Client": false,
			"ProcurementOfficer": false, "OHSExpert": false,
		},
		"progress_payments.view_financials": {
			"Admin": true, "ProjectManager": true, "SiteEngineer": false,
			"SubcontractorRep": true, "Client": true,
			"ProcurementOfficer": false, "OHSExpert": false,
		},
		"ohs.issue_penalty": {
			"Admin": true, "ProjectManager": true, "OHSExpert": true,
			"SiteEngineer": false, "SubcontractorRep": false, "Client": false,
			"ProcurementOfficer": false,
		},
		"material_approvals.decide": {
			"Admin": true, "ProjectManager": true, "Client": true,
			"SiteEngineer": false, "SubcontractorRep": false,
			"ProcurementOfficer": false, "OHSExpert": false,
		},
		"admin.manage_users": {
			"Admin": true, "ProjectManager": false, "SiteEngineer": false,
			"SubcontractorRep": false, "Client": false,
			"ProcurementOfficer": false, "OHSExpert": false,
		},
	}

	for perm, byRole := range expect {
		for role, want := range byRole {
			got := RoleHasDefault(role, perm)
			if got != want {
				t.Errorf("matris ihlali: rol=%s izin=%s → got=%v want=%v", role, perm, got, want)
			}
		}
	}

	// Her kritik endpoint bilinen bir izne bağlı olmalı (yetim endpoint yok).
	valid := map[string]bool{}
	for _, p := range AllPermissions {
		valid[p.Code()] = true
	}
	for ep, perm := range criticalEndpoints {
		if !valid[perm] {
			t.Errorf("endpoint %q tanımsız izne bağlı: %s", ep, perm)
		}
	}
}
