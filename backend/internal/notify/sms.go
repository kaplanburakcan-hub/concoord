package notify

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SMSSender — TR sağlayıcı soyutlaması (Plan §2: "sağlayıcı değişimi tek
// adaptör dosyası"). Yeni sağlayıcı = bu arayüzü uygulayan yeni adaptör.
type SMSSender interface {
	Send(ctx context.Context, phone, message string) error
	Name() string
}

// NewSMSSender — konfigürasyona göre adaptör seçer. Bilinmeyen/boş sağlayıcı →
// LogSMS (gönderim loglanır, hata dönmez; geliştirme ve sağlayıcısız kurulum).
func NewSMSSender(provider, apiURL, user, pass, header string, log *slog.Logger) SMSSender {
	switch strings.ToLower(provider) {
	case "netgsm":
		return &NetgsmSMS{APIURL: apiURL, User: user, Pass: pass, Header: header}
	default:
		return &LogSMS{log: log}
	}
}

// LogSMS — gerçek gönderim yapmaz; içerik loglanır.
type LogSMS struct{ log *slog.Logger }

func (l *LogSMS) Name() string { return "log" }
func (l *LogSMS) Send(_ context.Context, phone, message string) error {
	l.log.Info("SMS (log adaptörü, gönderilmedi)", "phone", phone, "message", message)
	return nil
}

// NetgsmSMS — Netgsm HTTP GET API adaptörü (get_sms). İleti Merkezi vb. için
// aynı arayüzü uygulayan yeni bir adaptör eklenir.
type NetgsmSMS struct {
	APIURL string // varsayılan: https://api.netgsm.com.tr/sms/send/get
	User   string
	Pass   string
	Header string // onaylı gönderici başlığı
}

func (n *NetgsmSMS) Name() string { return "netgsm" }

func (n *NetgsmSMS) Send(ctx context.Context, phone, message string) error {
	api := n.APIURL
	if api == "" {
		api = "https://api.netgsm.com.tr/sms/send/get"
	}
	q := url.Values{}
	q.Set("usercode", n.User)
	q.Set("password", n.Pass)
	q.Set("gsmno", phone)
	q.Set("message", message)
	q.Set("msgheader", n.Header)
	q.Set("dil", "TR")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
	code := strings.TrimSpace(string(body))
	// Netgsm: "00 ..." = başarı; diğer kodlar hata.
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(code, "00") {
		return fmt.Errorf("netgsm hata yanıtı: http=%d body=%q", res.StatusCode, code)
	}
	return nil
}
