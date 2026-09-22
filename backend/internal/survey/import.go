// Proje Keşfi toplu içe aktarma. payments/import.go'daki desenle tutarlı
// (ADR-0003): harici Excel kütüphanesi yok, .xlsx yalnızca standart
// kütüphaneyle (archive/zip + encoding/xml) okunur, .csv de desteklenir.
// Sütun düzeni (başlık satırı atlanır):
//
//	A=kategori, B=poz_no, C=tanım, D=birim, E=miktar, F=birim_fiyat,
//	G=para_birimi, H=açıklama
package survey

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

const importCols = 8

// importedRow — içe aktarılan tek ham satır; handler doğrular ve yazar.
type importedRow struct {
	Kategori   string
	PozNo      string
	Tanim      string
	Birim      string
	Miktar     float64
	BirimFiyat float64
	ParaBirimi string
	Aciklama   string
}

// parseImport — dosya adına/içeriğine göre .xlsx/.csv ayrımı yapar.
func parseImport(filename string, data []byte) ([]importedRow, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return parseImportXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return parseImportCSV(data)
	default:
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' { // xlsx bir ZIP'tir
			return parseImportXLSX(data)
		}
		return parseImportCSV(data)
	}
}

func parseImportCSV(data []byte) ([]importedRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv çözümlenemedi: %w", err)
	}
	var out []importedRow
	for i, rec := range recs {
		if i == 0 && looksLikeImportHeader(rec) {
			continue
		}
		if allImportCellsEmpty(rec) {
			continue
		}
		out = append(out, importRowFromCells(rec))
	}
	return out, nil
}

type importXLSXSST struct {
	Items []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type importXLSXSheet struct {
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

func parseImportXLSX(data []byte) ([]importedRow, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx (zip) açılamadı: %w", err)
	}

	var shared []string
	var sheetXML []byte
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			b, err := readImportZip(f)
			if err != nil {
				return nil, err
			}
			var sst importXLSXSST
			if err := xml.Unmarshal(b, &sst); err != nil {
				return nil, fmt.Errorf("sharedStrings çözümlenemedi: %w", err)
			}
			for _, si := range sst.Items {
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
		case f.Name == "xl/worksheets/sheet1.xml":
			sheetXML, err = readImportZip(f)
			if err != nil {
				return nil, err
			}
		}
	}
	if sheetXML == nil {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
				if sheetXML, err = readImportZip(f); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	if sheetXML == nil {
		return nil, fmt.Errorf("xlsx içinde çalışma sayfası bulunamadı")
	}

	var sheet importXLSXSheet
	if err := xml.Unmarshal(sheetXML, &sheet); err != nil {
		return nil, fmt.Errorf("sayfa çözümlenemedi: %w", err)
	}

	var out []importedRow
	for i, row := range sheet.Rows {
		cells := make([]string, importCols)
		for _, c := range row.Cells {
			col := importColIndex(c.R)
			if col < 0 || col >= importCols {
				continue
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
		if i == 0 && looksLikeImportHeader(cells) {
			continue
		}
		if allImportCellsEmpty(cells) {
			continue
		}
		out = append(out, importRowFromCells(cells))
	}
	return out, nil
}

func readImportZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// importColIndex — "B3" → 1 (A=0).
func importColIndex(ref string) int {
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

func importRowFromCells(cells []string) importedRow {
	get := func(i int) string {
		if i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return importedRow{
		Kategori:   get(0),
		PozNo:      get(1),
		Tanim:      get(2),
		Birim:      get(3),
		Miktar:     parseImportNum(get(4)),
		BirimFiyat: parseImportNum(get(5)),
		ParaBirimi: get(6),
		Aciklama:   get(7),
	}
}

// parseImportNum — hem "1.234,56" (TR) hem "1234.56" biçimini tolere eder.
func parseImportNum(s string) float64 {
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

func looksLikeImportHeader(cells []string) bool {
	joined := strings.ToLower(strings.Join(cells, " "))
	return strings.Contains(joined, "kategori") || strings.Contains(joined, "poz") ||
		strings.Contains(joined, "tanım") || strings.Contains(joined, "tanim")
}

func allImportCellsEmpty(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
