// Package storage — MinIO / S3 uyumlu nesne deposu istemcisi.
//
// Bilinçli karar: harici SDK yerine yalnızca standart kütüphane + AWS
// İmza v4 (SigV4) ile imzalama kullanılır. Böylece go.mod'a yeni bağımlılık
// EKLENMEZ (mevcut minimalist yığın korunur) ve imzalama davranışı tamamen
// denetlenebilir kalır. İmzalama iki biçimde uygulanır:
//
//   - Başlık imzalama: sunucu → MinIO PutObject/GetObject (yükleme/indirme
//     API üzerinden geçer; presigned URL istemciye sızmaz — Plan §4/§5.2).
//   - Sorgu imzalama (presign): public bir S3 uç noktası yapılandırıldığında
//     kısa ömürlü indirme bağlantısı üretmek için (opsiyonel).
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	service        = "s3"
	unsignedload   = "UNSIGNED-PAYLOAD"
	emptySHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	amzDateFmt     = "20060102T150405Z"
	shortDateFmt   = "20060102"
)

// Config — nesne deposu bağlantı ayarları (config paketinden beslenir).
type Config struct {
	Endpoint       string // host:port — sunucu→MinIO (ör. minio:9000)
	PublicEndpoint string // istemciye sunulan presign uç noktası (boşsa Endpoint)
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string // boşsa us-east-1
	UseSSL         bool
	ClamdAddr      string // Faz 10: opsiyonel antivirüs (host:port); boşsa tarama atlanır
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

// CheckUpload — Faz 10 yükleme güvenlik geçidi: dosya tipi doğrulaması + (varsa)
// antivirüs taraması. r iki kez okunur ve her seferinde başa sarılır; çağıran
// dönüşten sonra r'yi yükleme için tekrar kullanabilir. Güvenli (koklanmış) mime
// döner. Handler'lar bu değeri Content-Type olarak saklamalıdır.
func (c *Client) CheckUpload(r io.ReadSeeker, filename string) (mime string, err error) {
	mime, err = ValidateUpload(r, filename)
	if err != nil {
		return "", err
	}
	if c.cfg.ClamdAddr != "" {
		if err := ScanClamd(c.cfg.ClamdAddr, r); err != nil {
			return "", err
		}
		if _, serr := r.Seek(0, io.SeekStart); serr != nil {
			return "", serr
		}
	}
	return mime, nil
}

func (c *Client) scheme() string {
	if c.cfg.UseSSL {
		return "https"
	}
	return "http"
}

// objectURL — path-style adresleme: scheme://endpoint/bucket/key
func (c *Client) objectURL(endpoint, key string) string {
	return c.scheme() + "://" + endpoint + "/" + c.cfg.Bucket + "/" + encodePath(key)
}

// PutObject — nesneyi MinIO'ya yazar. payloadSHA256Hex çağıran tarafından
// (yükleme sırasında akışla) hesaplanır; böylece istek tam imzalı olur.
func (c *Client) PutObject(ctx context.Context, key, contentType string, body io.Reader, size int64, payloadSHA256Hex string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.objectURL(c.cfg.Endpoint, key), body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	c.signHeader(req, payloadSHA256Hex, []string{"content-type", "host", "x-amz-content-sha256", "x-amz-date"})

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("minio PutObject %s: %s: %s", key, res.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// Object — GetObject sonucu (akış + meta).
type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// GetObject — nesneyi MinIO'dan akış olarak okur. Çağıran Body'yi kapatmalıdır.
func (c *Client) GetObject(ctx context.Context, key string) (*Object, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.objectURL(c.cfg.Endpoint, key), nil)
	if err != nil {
		return nil, err
	}
	c.signHeader(req, emptySHA256Hex, []string{"host", "x-amz-content-sha256", "x-amz-date"})

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		res.Body.Close()
		return nil, fmt.Errorf("minio GetObject %s: %s: %s", key, res.Status, strings.TrimSpace(string(b)))
	}
	return &Object{
		Body:        res.Body,
		ContentType: res.Header.Get("Content-Type"),
		Size:        res.ContentLength,
	}, nil
}

// PresignGet — kısa ömürlü, imzalı indirme bağlantısı (public uç nokta üzerinden).
// downloadName verilirse tarayıcıya indirme adı dayatılır (Content-Disposition).
func (c *Client) PresignGet(key string, expiry time.Duration, downloadName string) string {
	now := time.Now().UTC()
	amzDate := now.Format(amzDateFmt)
	dateStamp := now.Format(shortDateFmt)
	cred := c.cfg.AccessKey + "/" + dateStamp + "/" + c.cfg.Region + "/" + service + "/aws4_request"

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", cred)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(int(expiry.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")
	if downloadName != "" {
		q.Set("response-content-disposition", "attachment; filename=\""+downloadName+"\"")
	}

	host := c.cfg.PublicEndpoint
	canonicalURI := "/" + c.cfg.Bucket + "/" + encodePath(key)
	canonicalQuery := encodeQuery(q)
	canonicalHeaders := "host:" + host + "\n"
	canonicalRequest := strings.Join([]string{
		http.MethodGet, canonicalURI, canonicalQuery, canonicalHeaders, "host", unsignedload,
	}, "\n")

	scope := dateStamp + "/" + c.cfg.Region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, hashHex([]byte(canonicalRequest)),
	}, "\n")

	sig := hex.EncodeToString(hmacSHA256(c.signingKey(dateStamp), []byte(stringToSign)))
	q.Set("X-Amz-Signature", sig)

	return c.scheme() + "://" + host + canonicalURI + "?" + encodeQuery(q)
}

// signHeader — isteğe SigV4 Authorization başlığını ekler.
func (c *Client) signHeader(req *http.Request, payloadHash string, signed []string) {
	now := time.Now().UTC()
	amzDate := now.Format(amzDateFmt)
	dateStamp := now.Format(shortDateFmt)

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Host = req.URL.Host // canonical host = URL host

	sort.Strings(signed)
	var chBuilder strings.Builder
	for _, h := range signed {
		var v string
		if h == "host" {
			v = req.URL.Host
		} else {
			v = req.Header.Get(h)
		}
		chBuilder.WriteString(h + ":" + strings.TrimSpace(v) + "\n")
	}
	signedHeaders := strings.Join(signed, ";")

	// req.URL.EscapedPath() zaten path-encoded olduğundan doğrudan kullanılır.
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		encodeQuery(req.URL.Query()),
		chBuilder.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + c.cfg.Region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, hashHex([]byte(canonicalRequest)),
	}, "\n")
	sig := hex.EncodeToString(hmacSHA256(c.signingKey(dateStamp), []byte(stringToSign)))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.cfg.AccessKey, scope, signedHeaders, sig)
	req.Header.Set("Authorization", auth)
}

func (c *Client) signingKey(dateStamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+c.cfg.SecretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(c.cfg.Region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// --- yardımcılar ---

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// encodePath — S3 anahtar yolu kodlaması: '/' korunur, diğer rezerve olmayan
// karakterler dışındaki her şey yüzde-kodlanır.
func encodePath(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if isUnreserved(ch) || ch == '/' {
			b.WriteByte(ch)
		} else {
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

// encodeQuery — kanonik sorgu dizesi: anahtarlar sıralı, RFC3986 kodlaması.
func encodeQuery(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := v[k]
		sort.Strings(vals)
		for _, val := range vals {
			parts = append(parts, awsEncode(k)+"="+awsEncode(val))
		}
	}
	return strings.Join(parts, "&")
}

func awsEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if isUnreserved(ch) {
			b.WriteByte(ch)
		} else {
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

func isUnreserved(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_' || ch == '~'
}

// BuildDocumentKey — Plan §5.2 anahtar deseni. Son segment ASCII-güvenli hale
// getirilir (yalnızca [A-Za-z0-9._-]); böylece SigV4 kanonik yol kodlaması ile
// tel üstündeki yol birebir eşleşir (imza uyuşmazlığı riski ortadan kalkar).
// İnsan-okunur orijinal ad DB'de (files.original_name) korunur.
func BuildDocumentKey(projectID, docID string, versionNo int, filename string) string {
	safe := sanitizeFilename(filename)
	return fmt.Sprintf("project/%s/documents/%s/v%d/%s", projectID, docID, versionNo, safe)
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "dosya"
	}
	return out
}
