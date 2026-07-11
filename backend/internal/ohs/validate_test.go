package ohs

import "testing"

func f(v float64) *float64 { return &v }

func TestValidateTemplateItems(t *testing.T) {
	if errs := validateTemplateItems(nil); len(errs) == 0 {
		t.Fatal("boş şablon kabul edilmemeli")
	}
	if errs := validateTemplateItems([]TemplateItem{{No: 1, Text: "Baret"}, {No: 1, Text: "Emniyet kemeri"}}); len(errs) == 0 {
		t.Fatal("tekrar eden madde no kabul edilmemeli")
	}
	if errs := validateTemplateItems([]TemplateItem{{No: 1, Text: "  "}}); len(errs) == 0 {
		t.Fatal("boş metin kabul edilmemeli")
	}
	if errs := validateTemplateItems([]TemplateItem{{No: 1, Text: "Baret"}, {No: 2, Text: "İskele", Critical: true}}); len(errs) != 0 {
		t.Fatalf("geçerli şablon reddedildi: %v", errs)
	}
}

func TestValidateResults(t *testing.T) {
	tmpl := []TemplateItem{{No: 1, Text: "a"}, {No: 2, Text: "b"}}
	if errs := validateResults(tmpl, []ResultItem{{No: 1, Answer: "ok"}}); len(errs) == 0 {
		t.Fatal("eksik yanıt kabul edilmemeli")
	}
	if errs := validateResults(tmpl, []ResultItem{{No: 1, Answer: "ok"}, {No: 2, Answer: "belki"}}); len(errs) == 0 {
		t.Fatal("geçersiz yanıt kabul edilmemeli")
	}
	if errs := validateResults(tmpl, []ResultItem{{No: 1, Answer: "ok"}, {No: 2, Answer: "ok"}, {No: 3, Answer: "ok"}}); len(errs) == 0 {
		t.Fatal("şablon dışı madde kabul edilmemeli")
	}
	if errs := validateResults(tmpl, []ResultItem{{No: 1, Answer: "ok"}, {No: 2, Answer: "na"}}); len(errs) != 0 {
		t.Fatalf("geçerli sonuç reddedildi: %v", errs)
	}
}

func TestScore(t *testing.T) {
	// 3 ok, 1 fail, 1 na → 3/4 = %75
	s := Score([]ResultItem{
		{No: 1, Answer: "ok"}, {No: 2, Answer: "ok"}, {No: 3, Answer: "ok"},
		{No: 4, Answer: "fail"}, {No: 5, Answer: "na"},
	})
	if s == nil || *s != 75.0 {
		t.Fatalf("beklenen 75, alınan %v", s)
	}
	// 1 ok, 2 fail → %33.33
	s = Score([]ResultItem{{No: 1, Answer: "ok"}, {No: 2, Answer: "fail"}, {No: 3, Answer: "fail"}})
	if s == nil || *s != 33.33 {
		t.Fatalf("beklenen 33.33, alınan %v", s)
	}
	// hepsi na → tanımsız
	if s := Score([]ResultItem{{No: 1, Answer: "na"}}); s != nil {
		t.Fatalf("tümü na iken skor nil olmalı, alınan %v", *s)
	}
}

func TestFindingTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"Open", "InProgress", true},
		{"Open", "Closed", true},
		{"InProgress", "Closed", true},
		{"Closed", "Open", false},
		{"Closed", "InProgress", false},
		{"InProgress", "Open", false},
	}
	for _, c := range cases {
		if got := CanTransitionFinding(c.from, c.to); got != c.want {
			t.Errorf("%s→%s: beklenen %v, alınan %v", c.from, c.to, c.want, got)
		}
	}
}

func TestValidatePenalty(t *testing.T) {
	if errs := validatePenalty("Fine", nil, "Baretsiz çalışma"); len(errs) == 0 {
		t.Fatal("tutarsız para cezası kabul edilmemeli")
	}
	if errs := validatePenalty("Fine", f(-5), "Baretsiz çalışma"); len(errs) == 0 {
		t.Fatal("negatif tutar kabul edilmemeli")
	}
	if errs := validatePenalty("Warning", f(100), "Baretsiz çalışma"); len(errs) == 0 {
		t.Fatal("uyarıda tutar kabul edilmemeli")
	}
	if errs := validatePenalty("Fine", f(500), ""); len(errs) == 0 {
		t.Fatal("boş ihlal türü kabul edilmemeli")
	}
	if errs := validatePenalty("Fine", f(500), "Baretsiz çalışma"); len(errs) != 0 {
		t.Fatalf("geçerli ceza reddedildi: %v", errs)
	}
	if errs := validatePenalty("Warning", nil, "Uygunsuz istif"); len(errs) != 0 {
		t.Fatalf("geçerli uyarı reddedildi: %v", errs)
	}
}

func TestDecodeDataURL(t *testing.T) {
	if _, _, err := decodeDataURL("http://example.com/a.jpg"); err == nil {
		t.Fatal("data-URL olmayan girdi reddedilmeli")
	}
	if _, _, err := decodeDataURL("data:text/plain;base64,aGk="); err == nil {
		t.Fatal("image/* dışı mime reddedilmeli")
	}
	mime, raw, err := decodeDataURL("data:image/png;base64,aGVsbG8=")
	if err != nil || mime != "image/png" || string(raw) != "hello" {
		t.Fatalf("çözümleme hatalı: %v %s %q", err, mime, raw)
	}
}
