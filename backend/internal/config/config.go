// Package config — tüm yapılandırma ortam değişkenlerinden okunur (12-factor).
// Yeni ayar eklemek: struct'a alan + Load içinde okuma. Koda sabit gömülmez.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string // development | production
	HTTPAddr string
	LogLevel string

	DBDSN string

	S3Endpoint       string
	S3PublicEndpoint string // presign URL'lerinde kullanılan istemci-erişilebilir uç nokta
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
	S3UseSSL         bool

	// ---- Faz 1: kimlik & yetki ----
	JWTSecret  string        // access token imzası (HS256); prod'da zorunlu
	AccessTTL  time.Duration // access token ömrü
	RefreshTTL time.Duration // refresh token ömrü
	ResetTTL   time.Duration // şifre sıfırlama jetonu ömrü

	// İlk açılışta idempotent olarak oluşturulan platform admini.
	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// ---- Faz 4: Bildirim motoru (SMTP + TR SMS adaptörü) ----
	SMTPHost string // boşsa e-posta gönderimi atlanır (yalnızca loglanır)
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	SMSProvider string // "netgsm" | "" (log adaptörü — gönderim yapılmaz)
	SMSAPIURL   string
	SMSUser     string
	SMSPass     string
	SMSHeader   string // onaylı gönderici başlığı

	// ---- Faz 6: Saha raporlama ----
	// Hava durumu ön doldurma (opsiyonel). Kapalıysa dış çağrı yapılmaz.
	WeatherEnabled bool
	WeatherAPIURL  string // boşsa Open-Meteo varsayılanı

	// ---- Faz 10: Sertleştirme ve yayına alma ----
	// CORS: virgülle ayrılmış izinli origin listesi. Boşsa çapraz-origin
	// istekleri reddedilir (aynı-origin dağıtım varsayılanı; nginx arkasında
	// frontend + API aynı host'tan servis edilir, cross-origin gerekmez).
	CORSOrigins []string
	// Genel istek hız sınırı (IP başına token-bucket). RPS = dolum hızı,
	// Burst = kova kapasitesi. 0 → sınır kapalı.
	RateLimitRPS   float64
	RateLimitBurst int
	// Kimlik uçları (login/refresh/forgot) için sıkı sabit-pencere sınırı:
	// IP başına dakikada izin verilen deneme. 0 → kapalı.
	LoginRateLimitPerMin int
	// Antivirüs (opsiyonel): clamd TCP adresi (host:port). Boşsa yükleme
	// taraması atlanır (yalnızca MIME/uzantı doğrulaması uygulanır).
	ClamdAddr string
	// Prod hata bildirimi (opsiyonel): 5xx/panik olaylarını gönderen webhook
	// (Slack/Teams uyumlu JSON). Boşsa yalnızca loglanır.
	ErrorWebhookURL string

	// Makine/Ekipman/Araç Envanteri Faz E: dışarıdan (Render Cron Job)
	// tetiklenen /internal/cron/* uçları için paylaşılan gizli anahtar.
	// Boşsa bu uçlar 503 ile kapalı kalır (yanlışlıkla açık bırakılmasın).
	CronSecret string
}

func Load() (*Config, error) {
	c := &Config{
		Env:         getenv("IPKS_ENV", "development"),
		HTTPAddr:    getenv("IPKS_HTTP_ADDR", ":8080"),
		LogLevel:    getenv("IPKS_LOG_LEVEL", "info"),
		DBDSN:       os.Getenv("IPKS_DB_DSN"),
		S3Endpoint:       getenv("IPKS_S3_ENDPOINT", "minio:9000"),
		S3PublicEndpoint: os.Getenv("IPKS_S3_PUBLIC_ENDPOINT"), // boşsa S3Endpoint kullanılır
		S3AccessKey:      os.Getenv("IPKS_S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("IPKS_S3_SECRET_KEY"),
		S3Bucket:         getenv("IPKS_S3_BUCKET", "ipks"),
		S3Region:         getenv("IPKS_S3_REGION", "us-east-1"),

		JWTSecret:              os.Getenv("IPKS_JWT_SECRET"),
		AccessTTL:              getdur("IPKS_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:             getdur("IPKS_REFRESH_TTL", 720*time.Hour), // 30 gün
		ResetTTL:               getdur("IPKS_RESET_TTL", time.Hour),
		BootstrapAdminEmail:    getenv("IPKS_BOOTSTRAP_ADMIN_EMAIL", "admin@ipks.local"),
		BootstrapAdminPassword: os.Getenv("IPKS_BOOTSTRAP_ADMIN_PASSWORD"),

		SMTPHost: os.Getenv("IPKS_SMTP_HOST"),
		SMTPPort: getenv("IPKS_SMTP_PORT", "587"),
		SMTPUser: os.Getenv("IPKS_SMTP_USER"),
		SMTPPass: os.Getenv("IPKS_SMTP_PASS"),
		SMTPFrom: os.Getenv("IPKS_SMTP_FROM"),

		SMSProvider: os.Getenv("IPKS_SMS_PROVIDER"),
		SMSAPIURL:   os.Getenv("IPKS_SMS_API_URL"),
		SMSUser:     os.Getenv("IPKS_SMS_USER"),
		SMSPass:     os.Getenv("IPKS_SMS_PASS"),
		SMSHeader:   os.Getenv("IPKS_SMS_HEADER"),
	}
	c.S3UseSSL, _ = strconv.ParseBool(getenv("IPKS_S3_USE_SSL", "false"))
	c.WeatherEnabled, _ = strconv.ParseBool(getenv("IPKS_WEATHER_ENABLED", "false"))
	c.WeatherAPIURL = os.Getenv("IPKS_WEATHER_API_URL")

	// ---- Faz 10 ----
	c.CORSOrigins = splitCSV(os.Getenv("IPKS_CORS_ORIGINS"))
	c.RateLimitRPS = getfloat("IPKS_RATE_LIMIT_RPS", 20)
	c.RateLimitBurst = getint("IPKS_RATE_LIMIT_BURST", 40)
	c.LoginRateLimitPerMin = getint("IPKS_LOGIN_RATE_LIMIT", 10)
	c.ClamdAddr = os.Getenv("IPKS_CLAMD_ADDR")
	c.ErrorWebhookURL = os.Getenv("IPKS_ERROR_WEBHOOK_URL")
	c.CronSecret = os.Getenv("IPKS_CRON_SECRET")

	if c.DBDSN == "" {
		return nil, fmt.Errorf("IPKS_DB_DSN zorunludur")
	}
	if c.Env == "production" && c.JWTSecret == "" {
		return nil, fmt.Errorf("IPKS_JWT_SECRET production ortamında zorunludur")
	}
	if c.JWTSecret == "" {
		// Geliştirmede güvenli olmayan sabit; prod'da yukarıda engellenir.
		c.JWTSecret = "gelistirme-icin-guvensiz-jwt-anahtari-degistir"
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getdur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getint(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getfloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// splitCSV — "a, b ,c" → ["a","b","c"]; boş girdilerde nil döner.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
