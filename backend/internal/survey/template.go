// Proje Keşfi toplu içe aktarma için indirilebilir .xlsx şablonu. ADR-0003
// ile tutarlı: harici kütüphane yok, minimal geçerli bir OOXML paketi
// archive/zip ile elle yazılır (paylaşılan dizeler yerine inlineStr
// kullanılır — import.go'daki okuyucu bunu zaten destekliyor).
package survey

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
)

var templateHeaders = []string{
	"Kategori", "Poz No", "Tanım", "Birim", "Miktar", "Birim Fiyat", "Para Birimi", "Açıklama",
}

var templateExampleRows = [][]string{
	{"Betonarme", "Y.16.001", "Temel ve bodrum betonarme (C35/45)", "m3", "1200", "4500", "TRY", "Örnek satır — silip kendi verinizi girin"},
}

// BuildImportTemplateXLSX — boş/örnek satırlı, sistemin kendi import
// uçunun (bkz. import.go) doğrudan okuyabileceği bir .xlsx üretir.
func BuildImportTemplateXLSX() ([]byte, error) {
	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	writeTemplateRow(&sheet, 1, templateHeaders)
	for i, row := range templateExampleRows {
		writeTemplateRow(&sheet, i+2, row)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, content string }{
		{"[Content_Types].xml", templateContentTypesXML},
		{"_rels/.rels", templateRelsXML},
		{"xl/workbook.xml", templateWorkbookXML},
		{"xl/_rels/workbook.xml.rels", templateWorkbookRelsXML},
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(p.content)); err != nil {
			return nil, err
		}
	}
	f, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(sheet.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeTemplateRow(buf *bytes.Buffer, rowNum int, cells []string) {
	fmt.Fprintf(buf, `<row r="%d">`, rowNum)
	for i, val := range cells {
		if val == "" {
			continue // boş hücre için <c> yazmaya gerek yok
		}
		col := templateColLetter(i)
		fmt.Fprintf(buf, `<c r="%s%d" t="inlineStr"><is><t>`, col, rowNum)
		_ = xml.EscapeText(buf, []byte(val))
		buf.WriteString(`</t></is></c>`)
	}
	buf.WriteString(`</row>`)
}

// templateColLetter — 0=A, 25=Z, 26=AA ...
func templateColLetter(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

const templateContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`

const templateRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const templateWorkbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Proje Kesfi" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`

const templateWorkbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
