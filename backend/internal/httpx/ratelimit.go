package httpx

// Faz 10 — İstek hız sınırlama (rate limiting).
//
// İki katman:
//   1. RateLimit      — genel amaçlı, IP başına token-bucket (tüm API).
//   2. LoginRateLimit — kimlik uçları için sıkı sabit-pencere sayacı
//      (kaba kuvvet parola denemesine karşı).
//
// Bilinçli olarak süreç-içi (in-memory): tek VPS/tek api replikası dağıtım
// senaryosu (Plan §2) için yeterli ve bağımlılıksız. Yatay ölçekleme
// gerektiğinde Redis tabanlı bir uygulama arkasına aynı arayüzle geçilebilir.
// IP, nginx'in ilettiği X-Forwarded-For'un ilk parçasından çözülür.

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Katman 1: token-bucket ---

type bucket struct {
	tokens float64
	last   time.Time
}

// TokenBucketLimiter — IP başına token-bucket. rps dolum hızı, burst kapasite.
type TokenBucketLimiter struct {
	rps   float64
	burst float64
	mu    sync.Mutex
	seen  map[string]*bucket
}

// RateLimit — genel hız sınırı middleware'i. rps<=0 ise sınır uygulanmaz
// (middleware kimliğe dokunmadan geçirir).
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	l := &TokenBucketLimiter{
		rps:   rps,
		burst: float64(burst),
		seen:  make(map[string]*bucket),
	}
	go l.gc()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(realIP(r)) {
				tooMany(w, r, int(l.rps))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *TokenBucketLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.seen[key]
	if !ok {
		l.seen[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}
	// Geçen süreye göre kovayı doldur.
	b.tokens += now.Sub(b.last).Seconds() * l.rps
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gc — uzun süredir görülmeyen kovaları temizler (bellek sızıntısını önler).
func (l *TokenBucketLimiter) gc() {
	t := time.NewTicker(10 * time.Minute)
	for range t.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for k, b := range l.seen {
			if b.last.Before(cutoff) {
				delete(l.seen, k)
			}
		}
		l.mu.Unlock()
	}
}

// --- Katman 2: kimlik uçları için sabit-pencere ---

type window struct {
	count int
	start time.Time
}

type fixedWindowLimiter struct {
	limit int
	span  time.Duration
	mu    sync.Mutex
	seen  map[string]*window
}

// LoginRateLimit — IP başına 1 dakikalık pencerede en fazla `perMin` deneme.
// perMin<=0 ise sınır uygulanmaz.
func LoginRateLimit(perMin int) func(http.Handler) http.Handler {
	if perMin <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	l := &fixedWindowLimiter{
		limit: perMin,
		span:  time.Minute,
		seen:  make(map[string]*window),
	}
	go l.gc()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(realIP(r)) {
				tooMany(w, r, l.limit)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *fixedWindowLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	win, ok := l.seen[key]
	if !ok || now.Sub(win.start) >= l.span {
		l.seen[key] = &window{count: 1, start: now}
		return true
	}
	if win.count >= l.limit {
		return false
	}
	win.count++
	return true
}

func (l *fixedWindowLimiter) gc() {
	t := time.NewTicker(10 * time.Minute)
	for range t.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for k, w := range l.seen {
			if w.start.Before(cutoff) {
				delete(l.seen, k)
			}
		}
		l.mu.Unlock()
	}
}

// tooMany — standart 429 zarfı + Retry-After başlığı.
func tooMany(w http.ResponseWriter, r *http.Request, perSecHint int) {
	w.Header().Set("Retry-After", "1")
	if perSecHint > 0 {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(perSecHint))
	}
	Error(w, r, http.StatusTooManyRequests, CodeRateLimited,
		"Çok fazla istek gönderildi, lütfen kısa süre sonra tekrar deneyin.", nil)
}

// realIP — X-Forwarded-For'un ilk (istemci) parçasını, yoksa RemoteAddr'ı döner.
// clientIP (middleware.go) tam değeri (virgüllü liste dahil) döndürdüğü için
// hız sınırında ilk parçaya normalize ediyoruz.
func realIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
