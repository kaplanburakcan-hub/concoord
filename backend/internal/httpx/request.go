package httpx

import (
	"encoding/json"
	"net/http"
)

// DecodeJSON — istek gövdesini katı biçimde çözer (bilinmeyen alan reddedilir,
// tek nesne beklenir, 1 MB sınır). Hata durumunda standart validation zarfı
// yazılır ve false döner.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		Error(w, r, http.StatusBadRequest, CodeValidation, "İstek gövdesi çözümlenemedi.",
			map[string]string{"detail": err.Error()})
		return false
	}
	if dec.More() {
		Error(w, r, http.StatusBadRequest, CodeValidation, "İstek gövdesinde birden fazla JSON nesnesi.", nil)
		return false
	}
	return true
}

// ValidationFailed — alan bazlı doğrulama hatalarını standart zarfla döner.
func ValidationFailed(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	Error(w, r, http.StatusBadRequest, CodeValidation, "Doğrulama hatası.", fields)
}
