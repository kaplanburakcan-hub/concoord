// Work_items toplu içe aktarma. Bilinçli karar (ADR-0003 deseniyle tutarlı):
// harici Excel kütüphanesi EKLENMEZ — .xlsx yalnızca standart kütüphaneyle
// (archive/zip + encoding/xml) okunur; .csv de desteklenir. Beklenen sütun
// düzeni (başlık satırı atlanır): A=poz_no, B=açıklama, C=birim, D=sözleşme_miktarı,
// E=birim_fiyat.
package payments

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

// ImportedRow — içe aktarılan tek satır (ham; handler doğrular ve yazar).
type ImportedRow struct {
	PozNo       string
	Description string
	Unit        string
	ContractQty float64
	UnitPrice   float64
}

// ParseImport — dosya adına göre .xlsx/.csv ayrımı yapar.
func ParseImport(filename string, data []byte) ([]ImportedRow, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return parseXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return parseCSV(data)
	default:
		// İçeriğe göre tahmin: XLSX bir ZIP'tir (PK imzası).
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return parseXLSX(data)
		}
		return parseCSV(data)
	}
}

func parseCSV(data []byte) ([]ImportedRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv çözümlenemedi: %w", err)
	}
	var out []ImportedRow
	for i, rec := range recs {
		if i == 0 && looksLikeHeader(rec) {
			continue
		}
		out = append(out, rowFromCells(rec))
	}
	return out, nil
}

// --- XLSX (stdlib) -----------------------------------------------------------

type xlsxSST struct {
	Items []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type xlsxSheet struct {
	Rows []struct {
		Cells []struct {
			R string `xml:"r,attr"` // hücre referansı, ör. "B3"
			T string `xml:"t,attr"` // tip: "s"=shared string, "str", "inlineStr", boş=sayı
			V string `xml:"v"`
			IS struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func parseXLSX(data []byte) ([]ImportedRow, error) {
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
			var sst xlsxSST
			if err := xml.Unmarshal(b, &sst); err != nil {
				return nil, fmt.Errorf("sharedStrings çözümlenemedi: %w", err)
			}
			for _, si := range sst.Items {
				if si.T != "" {
					shared = append(shared, si.T)
					continue
				}
				var sb strings.Builder // zengin metin parçalarını birleştir
				for _, r := range si.R {
					sb.WriteString(r.T)
				}
				shared = append(shared, sb.String())
			}
		case f.Name == "xl/worksheets/sheet1.xml":
			sheetXML, err = readZip(f)
			if err != nil {
				return nil, err
			}
		}
	}
	if sheetXML == nil {
		// İlk sayfayı isimden bağımsız bul.
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

	var sheet xlsxSheet
	if err := xml.Unmarshal(sheetXML, &sheet); err != nil {
		return nil, fmt.Errorf("sayfa çözümlenemedi: %w", err)
	}

	var out []ImportedRow
	for i, row := range sheet.Rows {
		cells := make([]string, 5) // A..E
		for _, c := range row.Cells {
			col := colIndex(c.R)
			if col < 0 || col > 4 {
				continue
			}
			var val string
			switch c.T {
			case "s": // shared string index
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
		if i == 0 && looksLikeHeader(cells) {
			continue
		}
		if allEmpty(cells) {
			continue
		}
		out = append(out, rowFromCells(cells))
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

// colIndex — "B3" → 1 (A=0). Harf kısmını ayrıştırır.
func colIndex(ref string) int {
	letters := ""
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			letters += string(ch)
		} else if ch >= 'a' && ch <= 'z' {
			letters += strings.ToUpper(string(ch))
		} else {
			break
		}
	}
	if letters == "" {
		return -1
	}
	n := 0
	for _, ch := range letters {
		n = n*26 + int(ch-'A'+1)
	}
	return n - 1
}

func rowFromCells(cells []string) ImportedRow {
	get := func(i int) string {
		if i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return ImportedRow{
		PozNo:       get(0),
		Description: get(1),
		Unit:        get(2),
		ContractQty: parseNum(get(3)),
		UnitPrice:   parseNum(get(4)),
	}
}

// parseNum — hem "1.234,56" (TR) hem "1234.56" biçimini tolere eder.
func parseNum(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func looksLikeHeader(cells []string) bool {
	joined := strings.ToLower(strings.Join(cells, " "))
	return strings.Contains(joined, "poz") || strings.Contains(joined, "açıklama") ||
		strings.Contains(joined, "aciklama") || strings.Contains(joined, "birim")
}

func allEmpty(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
