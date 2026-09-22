// Proje Keşfi toplu içe aktarma için indirilebilir .xlsx şablonu. ADR-0003
// ile tutarlı: harici kütüphane yok, minimal geçerli bir OOXML paketi
// archive/zip ile elle yazılır (paylaşılan dizeler yerine inlineStr
// kullanılır — import.go'daki okuyucu bunu zaten destekliyor).
//
// Kullanıcı ilk sürümü Excel'de açıp veri aralığını gerçek bir Excel
// Tablosu'na (filtre + banding + hazır boş satırlar) çevirip geri
// gösterdi — bu sürüm aynı sonucu baştan üretir: sütunlar Tablo1 olarak
// tanımlanır (yerleşik "TableStyleMedium2" stili, Excel'in kendi
// tanımı — burada renk/stil verisi taşımaya gerek yok), makul sütun
// genişlikleri ve doldurulmaya hazır ~26 boş satır eklenir.
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

var templateExampleRow = []string{
	"Betonarme", "Y.16.001", "Temel ve bodrum betonarme (C35/45)", "m3", "1200", "4500", "TRY", "Örnek satır — silip kendi verinizi girin",
}

// templateColWidths — sütun genişlikleri (Excel karakter birimiyle), başlık
// ve içerik uzunluğuna göre okunabilir olacak şekilde seçildi.
var templateColWidths = []float64{16, 14, 45, 10, 12, 14, 13, 45}

const templateBlankRows = 26 // örnek satırın altında doldurulmaya hazır boş satır sayısı

// BuildImportTemplateXLSX — başlık + bir örnek satır + doldurulmaya hazır
// boş satırlardan oluşan, gerçek bir Excel Tablosu (AutoFilter + banded
// stil) olarak biçimlenmiş .xlsx üretir.
func BuildImportTemplateXLSX() ([]byte, error) {
	totalRows := 1 + 1 + templateBlankRows // başlık + örnek + boşlar
	tableRef := fmt.Sprintf("A1:%s%d", templateColLetter(len(templateHeaders)-1), totalRows)

	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	fmt.Fprintf(&sheet, `<dimension ref="%s"/>`, tableRef)
	sheet.WriteString(`<cols>`)
	for i, w := range templateColWidths {
		fmt.Fprintf(&sheet, `<col min="%d" max="%d" width="%g" customWidth="1"/>`, i+1, i+1, w)
	}
	sheet.WriteString(`</cols>`)
	sheet.WriteString(`<sheetData>`)
	writeTemplateRow(&sheet, 1, templateHeaders)
	writeTemplateRow(&sheet, 2, templateExampleRow)
	for r := 3; r <= totalRows; r++ {
		fmt.Fprintf(&sheet, `<row r="%d"/>`, r)
	}
	sheet.WriteString(`</sheetData>`)
	sheet.WriteString(`<tableParts count="1"><tablePart r:id="rId1"/></tableParts>`)
	sheet.WriteString(`</worksheet>`)

	var table bytes.Buffer
	table.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	fmt.Fprintf(&table, `<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="ProjeKesfiTablosu" displayName="ProjeKesfiTablosu" ref="%s" totalsRowShown="0">`, tableRef)
	fmt.Fprintf(&table, `<autoFilter ref="%s"/>`, tableRef)
	fmt.Fprintf(&table, `<tableColumns count="%d">`, len(templateHeaders))
	for i, h := range templateHeaders {
		fmt.Fprintf(&table, `<tableColumn id="%d" name="`, i+1)
		_ = xml.EscapeText(&table, []byte(h))
		table.WriteString(`"/>`)
	}
	table.WriteString(`</tableColumns>`)
	table.WriteString(`<tableStyleInfo name="TableStyleMedium2" showFirstColumn="0" showLastColumn="0" showRowStripes="1" showColumnStripes="0"/>`)
	table.WriteString(`</table>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, content string }{
		{"[Content_Types].xml", templateContentTypesXML},
		{"_rels/.rels", templateRelsXML},
		{"xl/workbook.xml", templateWorkbookXML},
		{"xl/_rels/workbook.xml.rels", templateWorkbookRelsXML},
		{"xl/worksheets/_rels/sheet1.xml.rels", templateSheetRelsXML},
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
	if err := writeZipPart(zw, "xl/worksheets/sheet1.xml", sheet.Bytes()); err != nil {
		return nil, err
	}
	if err := writeZipPart(zw, "xl/tables/table1.xml", table.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipPart(zw *zip.Writer, name string, content []byte) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	return err
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
  <Override PartName="/xl/tables/table1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml"/>
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

const templateSheetRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table1.xml"/>
</Relationships>`
