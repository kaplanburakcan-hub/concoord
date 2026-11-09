package documents

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	driveScope      = "https://www.googleapis.com/auth/drive.readonly"
	driveFilesURL   = "https://www.googleapis.com/drive/v3/files"
	defaultTokenURI = "https://oauth2.googleapis.com/token"
	maxDrivePages   = 10
)

var driveFolderIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// driveClient — Google Drive'ı yalnızca okuyan servis hesabı istemcisi.
// Erişim jetonu, servis hesabı anahtarıyla imzalanan JWT ile alınır; ek bir
// bağımlılık gerektirmez. Jeton önbelleğe alınır ve süresi dolmadan yenilenir.
type driveClient struct {
	email    string
	key      *rsa.PrivateKey
	tokenURI string
	filesURL string
	http     *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func newDriveClient(raw string) (*driveClient, error) {
	var sa serviceAccountKey
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return nil, fmt.Errorf("servis hesabı JSON'u okunamadı: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("servis hesabı JSON'unda client_email veya private_key yok")
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, errors.New("servis hesabı özel anahtarı PEM biçiminde değil")
	}
	key, err := parseRSAKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("servis hesabı özel anahtarı okunamadı: %w", err)
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}
	return &driveClient{
		email:    sa.ClientEmail,
		key:      key,
		tokenURI: tokenURI,
		filesURL: driveFilesURL,
		http:     &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// parseRSAKey — Google anahtarları PKCS#8 olarak gelir; PKCS#1 de desteklenir.
func parseRSAKey(der []byte) (*rsa.PrivateKey, error) {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("anahtar RSA değil")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(der)
}

func b64JSON(v interface{}) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// accessToken — süresi dolmamış bir erişim jetonu döner, gerekirse yeniler.
func (d *driveClient) accessToken(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.token != "" && time.Now().Before(d.expires.Add(-time.Minute)) {
		return d.token, nil
	}

	now := time.Now()
	header, err := b64JSON(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := b64JSON(map[string]interface{}{
		"iss":   d.email,
		"scope": driveScope,
		"aud":   d.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	signingInput := header + "." + claims
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, d.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("jwt imzalanamadı: %w", err)
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("jeton isteği başarısız: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jeton alınamadı: HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("jeton yanıtı okunamadı: %w", err)
	}
	d.token = out.AccessToken
	d.expires = now.Add(time.Duration(out.ExpiresIn) * time.Second)
	return d.token, nil
}

type driveFile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ModifiedTime time.Time `json:"modifiedTime"`
	WebViewLink  string    `json:"webViewLink"`
}

// listFolder — klasördeki (çöpe taşınmamış) dosyaları ada göre sıralı döner.
func (d *driveClient) listFolder(ctx context.Context, folderID string) ([]driveFile, error) {
	tok, err := d.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	var files []driveFile
	pageToken := ""
	for page := 0; page < maxDrivePages; page++ {
		q := url.Values{}
		q.Set("q", fmt.Sprintf("'%s' in parents and trashed = false", folderID))
		q.Set("fields", "nextPageToken,files(id,name,modifiedTime,webViewLink)")
		q.Set("orderBy", "name")
		q.Set("pageSize", "200")
		q.Set("supportsAllDrives", "true")
		q.Set("includeItemsFromAllDrives", "true")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.filesURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := d.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("drive isteği başarısız: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("drive dosya listesi alınamadı: HTTP %d", resp.StatusCode)
		}
		var out struct {
			Files         []driveFile `json:"files"`
			NextPageToken string      `json:"nextPageToken"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("drive yanıtı okunamadı: %w", err)
		}
		files = append(files, out.Files...)
		if out.NextPageToken == "" {
			break
		}
		pageToken = out.NextPageToken
	}
	return files, nil
}

// ConfigureDrive — Çizim kaynağı olarak gösterilecek Drive klasörünü etkinleştirir.
// Klasör kimliği boşsa özellik kapalı kalır ve hata dönmez.
func (h *Handler) ConfigureDrive(serviceAccountJSON, folderID string) error {
	if folderID == "" {
		return nil
	}
	if !driveFolderIDPattern.MatchString(folderID) {
		return errors.New("drive klasör kimliği geçersiz")
	}
	d, err := newDriveClient(serviceAccountJSON)
	if err != nil {
		return err
	}
	h.drive, h.driveFolderID = d, folderID
	return nil
}

// driveDrawingRecords — Çizim klasöründeki dosyaları modül kaydı gibi döner.
// Dosyalar Drive'da kalır; kayıt yalnızca Drive'daki bağlantıyı gösterir.
// Kimlik, Drive dosya kimliğinden türetilen sabit bir UUID'dir.
func (h *Handler) driveDrawingRecords(ctx context.Context, category string) ([]moduleRecordDTO, error) {
	if h.drive == nil || (category != "" && category != "Drawing") {
		return nil, nil
	}
	files, err := h.drive.listFolder(ctx, h.driveFolderID)
	if err != nil {
		return nil, err
	}
	out := make([]moduleRecordDTO, 0, len(files))
	for _, f := range files {
		out = append(out, moduleRecordDTO{
			Source:      "drive",
			SourceLabel: "Drive",
			ID:          uuid.NewSHA1(uuid.NameSpaceURL, []byte("drive:"+f.ID)),
			Title:       f.Name,
			DocCategory: "Drawing",
			Date:        f.ModifiedTime,
			Link:        f.WebViewLink,
		})
	}
	return out, nil
}
