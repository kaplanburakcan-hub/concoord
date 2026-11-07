// Package tabular — toplu içe aktarmalar için ortak tablo okuyucu.
// ADR-0003 gereği harici Excel kütüphanesi kullanılmaz: .xlsx ham zip + XML
// olarak (archive/zip, encoding/xml) okunur, .csv encoding/csv ile okunur.
//
// CSV ayırıcısı otomatik seçilir: Türkçe Excel'in "Farklı kaydet → CSV" çıktısı
// noktalı virgül kullanır; ilk satırda virgül yoksa ';' kabul edilir.
package tabular

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Rows — başlık dahil ham satırlar; her satır hücre metinlerinden oluşur.
// xlsx'te boş hücreler "" ile doldurulur, sütun konumu korunur.
type Rows [][]string

// Read — dosya adına göre .xlsx / .csv ayrımı yapar; uzantı yoksa içeriğe bakar.
func Read(filename string, data []byte) (Rows, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return readXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return readCSV(data)
	default:
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return readXLSX(data)
		}
		return readCSV(data)
	}
}

func readCSV(data []byte) (Rows, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM (Excel)
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	comma := bytes.Count(line, []byte(","))
	semi := bytes.Count(line, []byte(";"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	if semi > comma {
		r.Comma = ';'
	}
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv çözümlenemedi: %w", err)
	}
	out := make(Rows, 0, len(recs))
	for _, rec := range recs {
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
		out = append(out, rec)
	}
	return out, nil
}

type sst struct {
	Items []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type sheet struct {
	Rows []struct {
		Cells []struct {
			R  string `xml:"r,attr"`
			T  string `xml:"t,attr"`
			V  string `xml:"v"`
			IS struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func readXLSX(data []byte) (Rows, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx (zip) açılamadı: %w", err)
	}
	var shared []string
	var sheetXML []byte
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			b, err := readZip(f)
			if err != nil {
				return nil, err
			}
			var s sst
			if err := xml.Unmarshal(b, &s); err != nil {
				return nil, fmt.Errorf("sharedStrings çözümlenemedi: %w", err)
			}
			for _, si := range s.Items {
				if si.T != "" {
					shared = append(shared, si.T)
					continue
				}
				var sb strings.Builder
				for _, r := range si.R {
					sb.WriteString(r.T)
				}
				shared = append(shared, sb.String())
			}
		case sheetXML == nil && f.Name == "xl/worksheets/sheet1.xml":
			if sheetXML, err = readZip(f); err != nil {
				return nil, err
			}
		}
	}
	if sheetXML == nil {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
				if sheetXML, err = readZip(f); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	if sheetXML == nil {
		return nil, fmt.Errorf("xlsx içinde çalışma sayfası bulunamadı")
	}
	var sh sheet
	if err := xml.Unmarshal(sheetXML, &sh); err != nil {
		return nil, fmt.Errorf("sayfa çözümlenemedi: %w", err)
	}

	out := make(Rows, 0, len(sh.Rows))
	for _, row := range sh.Rows {
		var cells []string
		for _, c := range row.Cells {
			col := colIndex(c.R)
			if col < 0 {
				continue
			}
			for len(cells) <= col {
				cells = append(cells, "")
			}
			var val string
			switch c.T {
			case "s":
				if idx, err := strconv.Atoi(strings.TrimSpace(c.V)); err == nil && idx >= 0 && idx < len(shared) {
					val = shared[idx]
				}
			case "inlineStr":
				val = c.IS.T
			default:
				val = c.V
			}
			cells[col] = strings.TrimSpace(val)
		}
		out = append(out, cells)
	}
	return out, nil
}

func readZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// colIndex — "B3" → 1 (A=0).
func colIndex(ref string) int {
	n := 0
	seen := false
	for _, ch := range strings.ToUpper(ref) {
		if ch < 'A' || ch > 'Z' {
			break
		}
		seen = true
		n = n*26 + int(ch-'A'+1)
	}
	if !seen {
		return -1
	}
	return n - 1
}

// ParseNum — hem "1.234,56" (TR) hem "1234.56" biçimini kabul eder. Boş → (0, true).
func ParseNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// IsEmpty — satırın tüm hücreleri boş mu.
func IsEmpty(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// Cell — i. hücre (yoksa boş).
func Cell(cells []string, i int) string {
	if i < len(cells) {
		return strings.TrimSpace(cells[i])
	}
	return ""
}
