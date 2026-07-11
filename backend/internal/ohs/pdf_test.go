package ohs

import (
	"bytes"
	"testing"
)

func TestBuildPenaltyPDF(t *testing.T) {
	amt := 2500.0
	pdf := BuildPenaltyPDF(PenaltyPDFData{
		PenaltyNo: "ISG-007", ProjectName: "Konut Projesi", ProjectCode: "KNT-01",
		Subcontractor: "Yılmaz İnşaat", ViolationType: "Baretsiz çalışma",
		PenaltyLevel: "Fine", Amount: &amt,
		Note: "B blok 3. kat kalıp ekibinde iki işçi baretsiz tespit edildi.",
		IssuedBy: "Ayşe Şahin", IssuedAt: "07.07.2026 14:30", HasEvidence: true,
	})
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("PDF başlığı yok")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatal("PDF sonlandırıcısı yok")
	}
	if !bytes.Contains(pdf, []byte("ISG-007")) {
		t.Fatal("tutanak numarası içerikte yok")
	}
	if bytes.ContainsRune(pdf[:64], 0) {
		t.Fatal("başlıkta beklenmeyen null bayt")
	}
}

func TestNum2(t *testing.T) {
	if got := num2(1234567.89); got != "1.234.567,89" {
		t.Fatalf("beklenen 1.234.567,89, alınan %s", got)
	}
	if got := num2(500); got != "500,00" {
		t.Fatalf("beklenen 500,00, alınan %s", got)
	}
}

func TestWrapText(t *testing.T) {
	lines := wrapText("bir iki uc dort bes", 7)
	if len(lines) != 3 || lines[0] != "bir iki" {
		t.Fatalf("sarma hatalı: %v", lines)
	}
}
