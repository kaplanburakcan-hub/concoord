// Package httpx — API sözleşmesinin ortak parçaları: hata zarfı (error envelope)
// ve JSON yanıt yardımcıları. Tüm endpoint'ler yalnızca bu yardımcılarla yanıt döner;
// böylece istemci her hatayı aynı şemayla parse eder.
package httpx

import (
	"encoding/json"
	"net/http"
)

// ErrorBody — sabit API hata zarfı:
//
//	{ "error": { "code": "...", "message": "...", "details": {...}, "request_id": "..." } }
type ErrorBody struct {
	Error ErrorInfo `json:"error"`
}

type ErrorInfo struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// Standart hata kodları — modüller genişletir, format değişmez.
const (
	CodeValidation   = "validation_error"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict" // optimistic locking (row_version) çakışması dahil
	CodeInternal     = "internal_error"
	CodeRateLimited  = "rate_limited" // Faz 10: istek hız sınırı aşıldı (429)
)

func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	JSON(w, status, ErrorBody{Error: ErrorInfo{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: RequestIDFrom(r.Context()),
	}})
}

func Internal(w http.ResponseWriter, r *http.Request) {
	Error(w, r, http.StatusInternalServerError, CodeInternal, "Beklenmeyen bir hata oluştu.", nil)
}
