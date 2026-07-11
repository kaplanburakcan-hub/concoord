package httpx

// Faz 10 — Prod hata bildirimi (izleme).
//
// healthz/readyz canlılık/hazırlık sağlar; bu dosya ise SUNUCU HATASI (5xx)
// olaylarını operatöre bildirir. Yapılandırılmış log her zaman birincil kayıttır;
// webhook (Slack/Teams uyumlu {"text": "..."}) opsiyonel anlık uyarıdır. URL
// boşsa bildirim yalnızca loglanır. Gönderim asenkron ve en-iyi-çaba (best effort):
// izleme kanalının yavaşlığı istek yolunu ASLA bloklamaz.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ErrorReporter struct {
	url string
	env string
	log *slog.Logger
	cl  *http.Client
}

func NewErrorReporter(webhookURL, env string, log *slog.Logger) *ErrorReporter {
	return &ErrorReporter{
		url: webhookURL,
		env: env,
		log: log,
		cl:  &http.Client{Timeout: 5 * time.Second},
	}
}

// report — mesajı asenkron gönderir (best effort). url boşsa yalnızca loglanır.
func (er *ErrorReporter) report(title, detail string) {
	if er == nil {
		return
	}
	er.log.Warn("hata bildirimi", "title", title, "detail", detail)
	if er.url == "" {
		return
	}
	msg := "🚨 [İPKS/" + er.env + "] " + title
	if detail != "" {
		msg += "\n" + detail
	}
	body, _ := json.Marshal(map[string]string{"text": msg})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, er.url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := er.cl.Do(req)
		if err != nil {
			er.log.Error("hata webhook gönderilemedi", "err", err)
			return
		}
		_ = resp.Body.Close()
	}()
}

// ErrorNotify — yanıt durumu 5xx ise operatöre bildirim üretir. Recover
// middleware'inin panikleri 500'e çevirmesi de bu katmanda yakalanır. rep nil
// veya webhook kapalıysa yalnızca loglanır.
func ErrorNotify(rep *ErrorReporter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			if sw.status >= 500 {
				rep.report(
					"HTTP "+http.StatusText(sw.status)+" ("+itoa(sw.status)+")",
					r.Method+" "+r.URL.Path+"  request_id="+RequestIDFrom(r.Context()),
				)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
