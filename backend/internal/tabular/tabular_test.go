package tabular

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestReadCSVComma(t *testing.T) {
	rows, err := Read("a.csv", []byte("kod,tutar\nCC-01,1234.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "CC-01" || rows[1][1] != "1234.5" {
		t.Fatalf("beklenmeyen satırlar: %v", rows)
	}
}

// Türkçe Excel CSV'si: noktalı virgül ve BOM.
func TestReadCSVSemicolonBOM(t *testing.T) {
	data := append([]byte("\xef\xbb\xbf"), []byte("kod;tutar\nCC-01;\"1.234,56\"\n")...)
	rows, err := Read("a.csv", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "kod" || rows[1][1] != "1.234,56" {
		t.Fatalf("noktalı virgül çözümlenemedi: %v", rows)
	}
}

func TestReadXLSXInlineAndSharedStrings(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"xl/sharedStrings.xml": `<sst><si><t>kod</t></si><si><t>CC-02</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>1</v></c><c r="C2" t="inlineStr"><is><t>5.000</t></is></c></row>` +
			`</sheetData></worksheet>`,
	}
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()

	rows, err := Read("a.xlsx", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "kod" || rows[1][0] != "CC-02" {
		t.Fatalf("xlsx satırları yanlış: %v", rows)
	}
	// C2 hücresi B sütununa kaymadan 3. sıraya yazılmalı
	if len(rows[1]) != 3 || rows[1][2] != "5.000" || rows[1][1] != "" {
		t.Fatalf("sütun konumu korunmadı: %v", rows[1])
	}
}

func TestParseNum(t *testing.T) {
	cases := map[string]struct {
		want float64
		ok   bool
	}{
		"1.234,56": {1234.56, true},
		"1234.5":   {1234.5, true},
		"1234,5":   {1234.5, true},
		"":         {0, true},
		"abc":      {0, false},
	}
	for in, c := range cases {
		got, ok := ParseNum(in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseNum(%q) = %v,%v; istenen %v,%v", in, got, ok, c.want, c.ok)
		}
	}
}
