package storage

// Faz 10 — Yükleme güvenliği: dosya tipi doğrulama + opsiyonel antivirüs.
//
// İki savunma katmanı:
//   1. ValidateUpload — uzantı + sihirli-bayt (magic byte) çapraz kontrolü.
//      Tarayıcının bildirdiği Content-Type'a GÜVENİLMEZ; ilk 512 bayt
//      http.DetectContentType ile koklanır ve uzantıyla tutarlılığı denetlenir.
//      Yürütülebilir/betik içerik reddedilir. İzin listesi (allowlist) yaklaşımı:
//      yalnızca bilinen güvenli belge/görsel/ofis tipleri kabul edilir.
//   2. ScanClamd — opsiyonel. clamd (ClamAV) TCP soketine INSTREAM protokolüyle
//      tarama. Adres boşsa (yapılandırılmamışsa) atlanır.

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// izinli uzantı → beklenen sihirli-bayt sınıfı. Sınıf "sniffed" MIME'in
// başlangıç eşleşmesiyle doğrulanır.
var allowedExt = map[string][]string{
	".pdf":  {"application/pdf"},
	".png":  {"image/png"},
	".jpg":  {"image/jpeg"},
	".jpeg": {"image/jpeg"},
	".gif":  {"image/gif"},
	".webp": {"image/webp"},
	".heic": {"image/heic", "application/octet-stream"}, // mobil kamera; sniff çoğu zaman octet-stream
	".heif": {"image/heif", "application/octet-stream"},
	".csv":  {"text/plain", "text/csv", "application/csv"},
	".txt":  {"text/plain"},
	// Ofis belgeleri ZIP tabanlıdır (docx/xlsx) → sniff "application/zip";
	// eski format (.xls/.doc) OLE → "application/x-ole-storage" veya octet-stream.
	".xlsx": {"application/zip", "application/octet-stream"},
	".docx": {"application/zip", "application/octet-stream"},
	".xls":  {"application/x-ole-storage", "application/octet-stream"},
	".doc":  {"application/x-ole-storage", "application/octet-stream"},
	".dwg":  {"application/octet-stream", "image/vnd.dwg"}, // teknik çizim
}

// tehlikeli sniff sonuçları — uzantı ne olursa olsun kesin ret.
var deniedSniff = map[string]bool{
	"application/x-msdownload":        true, // .exe / .dll
	"application/x-mach-binary":       true,
	"application/x-elf":               true,
	"application/x-executable":        true,
	"application/x-sharedlib":         true,
	"application/vnd.microsoft.portable-executable": true,
}

const sniffLen = 512

// ValidateUpload — filename ve içerik başlangıcına göre yükleme tipini doğrular.
// r okunur (ilk sniffLen bayt) ve BAŞA SARILIR; çağıran aynı okuyucuyu yükleme
// için tekrar kullanabilir. Döndürülen mime, depolamada saklanacak güvenli
// (koklanmış) değerdir. Doğrulama başarısızsa hata döner (kullanıcıya 4xx).
func ValidateUpload(r io.ReadSeeker, filename string) (mime string, err error) {
	ext := strings.ToLower(extOf(filename))
	expected, ok := allowedExt[ext]
	if !ok {
		return "", fmt.Errorf("desteklenmeyen dosya uzantısı: %q", ext)
	}

	head := make([]byte, sniffLen)
	n, rerr := io.ReadFull(r, head)
	if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
		return "", fmt.Errorf("dosya okunamadı: %w", rerr)
	}
	head = head[:n]
	if _, serr := r.Seek(0, io.SeekStart); serr != nil {
		return "", fmt.Errorf("dosya başa sarılamadı: %w", serr)
	}

	sniff := trimMediaType(http.DetectContentType(head))
	if deniedSniff[sniff] {
		return "", fmt.Errorf("yürütülebilir/tehlikeli içerik reddedildi (%s)", sniff)
	}

	for _, want := range expected {
		if sniff == want {
			return sniff, nil
		}
	}
	// CSV/TXT için DetectContentType bazen "text/plain; charset=..." döndürür;
	// prefix eşleşmesini de kabul ediyoruz.
	for _, want := range expected {
		if strings.HasPrefix(sniff, "text/") && strings.HasPrefix(want, "text/") {
			return sniff, nil
		}
	}
	return "", fmt.Errorf("dosya içeriği uzantıyla uyuşmuyor (uzantı %s, içerik %s)", ext, sniff)
}

func extOf(name string) string {
	for i := len(name) - 1; i >= 0 && name[i] != '/'; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}

func trimMediaType(s string) string {
	if i := strings.IndexByte(s, ';'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// ScanClamd — clamd INSTREAM taraması. addr boşsa tarama atlanır (nil döner).
// Temiz → nil; virüs/bulaşma → hata; clamd'a ulaşılamazsa da hata (fail-closed:
// tarama açıksa ve çalışmıyorsa yükleme reddedilir — güvenli varsayılan).
func ScanClamd(addr string, r io.Reader) error {
	if addr == "" {
		return nil
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("antivirüs servisine ulaşılamadı: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("antivirüs komutu yazılamadı: %w", err)
	}
	buf := make([]byte, 32*1024)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			var sz [4]byte
			binary.BigEndian.PutUint32(sz[:], uint32(n))
			if _, err := conn.Write(sz[:]); err != nil {
				return fmt.Errorf("antivirüs chunk boyutu yazılamadı: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return fmt.Errorf("antivirüs chunk yazılamadı: %w", err)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("dosya okunamadı: %w", rerr)
		}
	}
	// Sıfır uzunlukta chunk = akış sonu.
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("antivirüs akış sonu yazılamadı: %w", err)
	}

	resp, err := bufio.NewReader(conn).ReadString('\x00')
	if err != nil && err != io.EOF {
		return fmt.Errorf("antivirüs yanıtı okunamadı: %w", err)
	}
	// Beklenen: "stream: OK\x00" veya "stream: <Signature> FOUND\x00"
	if strings.Contains(resp, "FOUND") {
		return fmt.Errorf("zararlı içerik tespit edildi: %s", strings.TrimSpace(resp))
	}
	if !strings.Contains(resp, "OK") {
		return fmt.Errorf("antivirüs beklenmeyen yanıt: %s", strings.TrimSpace(resp))
	}
	return nil
}
