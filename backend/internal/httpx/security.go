package httpx

// Faz 10 — CORS ve güvenlik başlıkları (defense in depth).
//
// Dağıtım varsayılanı aynı-origin'dir: nginx frontend'i ve /api'yi tek host'tan
// servis eder, dolayısıyla tarayıcı CORS preflight'ı tetiklenmez ve izinli
// origin listesi boş bırakılabilir. Ayrı bir origin'den (örn. yerel geliştirme
// veya ileride ayrık bir istemci) erişim gerektiğinde IPKS_CORS_ORIGINS ile
// açıkça izin verilir — joker (*) bilinçli olarak DESTEKLENMEZ; kimlik bilgili
// (Authorization başlıklı) isteklerde güvensizdir.

import (
	"net/http"
	"strings"
)

// CORS — izinli origin allowlist'ine göre CORS başlıklarını yönetir ve
// preflight (OPTIONS) isteklerini yanıtlar. allowed boşsa middleware hiçbir
// CORS başlığı eklemez (aynı-origin dağıtım).
func CORS(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[strings.ToLower(strings.TrimRight(o, "/"))] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && len(set) > 0 {
				if _, ok := set[strings.ToLower(strings.TrimRight(origin, "/"))]; ok {
					h := w.Header()
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Vary", "Origin")
					h.Set("Access-Control-Allow-Credentials", "true")
					h.Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
					h.Set("Access-Control-Max-Age", "600")
				}
			}
			// Preflight'ı burada sonlandır (izinli değilse de gövdesiz 204;
			// tarayıcı eksik CORS başlıkları nedeniyle zaten reddeder).
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders — temel güvenlik başlıkları. nginx prod'da bir kısmını zaten
// ekler; uygulama katmanında da uygulamak nginx dışı erişim (doğrudan port,
// geliştirme) ve savunma derinliği için güvence sağlar.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
