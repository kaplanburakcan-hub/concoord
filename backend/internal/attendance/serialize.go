package attendance

import (
	"net/http"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// Konum maskeleme — SERİLEŞTİRME KATMANI (handler'da değil).
//
// Konum alanı taşıyan her DTO locationMasker arayüzünü uygular. Bir yanıt
// bu alanları içeriyorsa handler kendi if/perm kontrolünü YAZMAZ — bunun
// yerine writeLocationAwareJSON'u çağırır; izin kontrolü, maskeleme VE
// "konum verisine her erişim denetim izine yazılsın" kuralı TEK bir yerden
// uygulanır. Yeni bir uç konum alanı döndürecekse tek yapması gereken DTO'suna
// maskLocation() eklemek ve bu fonksiyonu kullanmaktır — ayrı bir izin
// kontrolü icat etmesi gerekmez, dolayısıyla unutup sızdırması zorlaşır.
// ---------------------------------------------------------------------------

type locationMasker interface {
	maskLocation()
}

func (e *eventDTO) maskLocation() {
	e.Lat = nil
	e.Lng = nil
	e.AccuracyM = nil
	e.DistanceM = nil
}

// writeLocationAwareJSON — attendance.view_location kontrolünü kendisi yapar.
// İzin yoksa maskables içindeki her DTO'yu maskeler (payload zaten bu DTO'lara
// referans tutar, yerinde değişir). İzin varsa gerçek konum döndürüldüğü için
// bir VIEW denetim kaydı düşer (yazma değil, OKUMA denetimi — KVKK gereği).
func (h *Handler) writeLocationAwareJSON(
	w http.ResponseWriter, r *http.Request, status int,
	entity, entityID string, maskables []locationMasker, payload any,
) {
	uid, _ := auth.UserIDFrom(r.Context())
	projectID := auth.ProjectFromRequest(r)
	allowed, err := h.eval.Can(r.Context(), uid, projectID, "attendance.view_location")
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !allowed {
		for _, m := range maskables {
			m.maskLocation()
		}
	} else if len(maskables) > 0 {
		meta := audit.MetaFrom(r.Context())
		h.rec.Record(r.Context(), audit.Entry{
			ActorID: uid.String(), Entity: entity, EntityID: entityID, Action: audit.ActionView,
			After: map[string]any{"count": len(maskables)}, IP: meta.IP, ReqID: meta.ReqID,
		})
	}
	httpx.JSON(w, status, payload)
}
