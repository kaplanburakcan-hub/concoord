package idarihakedis

import "testing"

func TestNormalizeKesintiler_TemelKalemlerSilinemez(t *testing.T) {
	// İstemci yalnızca bir ceza satırı gönderse bile temel kalemler geri gelir.
	out := normalizeKesintiler([]KesintiKalem{{Ad: "Ekstra ceza", Tutar: 10}})
	kilitli := map[string]bool{}
	for _, k := range out {
		if k.Kilitli {
			kilitli[k.Kod] = true
		}
	}
	for _, kod := range []string{"STOPAJ", "DAMGA", "KDV_TEVKIFAT", "GECICI_KABUL", "FF_KESIN_TEMINAT", "SGK", "AVANS"} {
		if !kilitli[kod] {
			t.Errorf("%s geri eklenmedi / kilitli değil", kod)
		}
	}
	last := out[len(out)-1]
	if last.Kod != "" || last.Kilitli || last.Ad != "Ekstra ceza" {
		t.Errorf("serbest satır korunmalı ve kilitsiz olmalı: %+v", last)
	}
}

func TestNormalizeKesintiler_SilinebilirStandartKalem(t *testing.T) {
	out := normalizeKesintiler(nil)
	for _, k := range out {
		if k.Kod == "GECIKME_CEZASI" || k.Kod == "IDARE_MAKINESI" {
			t.Errorf("silinmiş silinebilir kalem geri eklenmemeli: %s", k.Kod)
		}
	}
}

func TestNormalizeKesintiler_EskiAdlarKodaBaglanir(t *testing.T) {
	out := normalizeKesintiler([]KesintiKalem{
		{Ad: "Gelir / Kurumlar Vergisi (E x %3)", Tutar: 300},
		{Ad: "Damga Vergisi (E x % …)", Tutar: 50},
		{Ad: "Gecikme Cezası", Tutar: 7},
	})
	got := map[string]float64{}
	for _, k := range out {
		got[k.Kod] = k.Tutar
	}
	if got["STOPAJ"] != 300 || got["DAMGA"] != 50 || got["GECIKME_CEZASI"] != 7 {
		t.Errorf("eski adlar koda bağlanmadı: %v", got)
	}
}

func TestOranDegisti(t *testing.T) {
	ref := oranlar{Var: true, Kdv: 20, Oran: map[string]float64{"STOPAJ": 3, "DAMGA": 0.948, "KDV_TEVKIFAT": 0, "GECICI_KABUL": 5}}
	r := func(v float64) *float64 { return &v }
	same := []KesintiKalem{{Kod: "STOPAJ", OranPct: r(3)}, {Kod: "GECICI_KABUL", OranPct: r(5)}}
	if oranDegisti(20, same, ref) {
		t.Error("aynı oranlar fark sayılmamalı")
	}
	if !oranDegisti(10, same, ref) {
		t.Error("KDV farkı yakalanmalı")
	}
	if !oranDegisti(20, []KesintiKalem{{Kod: "STOPAJ", OranPct: r(4)}}, ref) {
		t.Error("stopaj farkı yakalanmalı")
	}
	if !oranDegisti(20, []KesintiKalem{{Kod: "STOPAJ"}}, ref) {
		t.Error("oranın silinmesi de değişiklik sayılmalı")
	}
	// Referansta oran yoksa (eski kayıt) ve gönderilende de yoksa değişiklik değil.
	if oranDegisti(20, []KesintiKalem{{Kod: "STOPAJ"}}, oranlar{Var: true, Kdv: 20, Oran: map[string]float64{}}) {
		t.Error("oransız eski kayıt değişmemiş sayılmalı")
	}
}

func TestOranliTutarlar(t *testing.T) {
	r := func(v float64) *float64 { return &v }
	ks := normalizeKesintiler(nil)
	for i := range ks {
		switch ks[i].Kod {
		case "STOPAJ":
			ks[i].OranPct = r(3)
		case "KDV_TEVKIFAT":
			ks[i].OranPct = r(40)
		}
	}
	oranliTutarlar(ks, 1000, 200) // E=1000, F=200
	got := map[string]float64{}
	for _, k := range ks {
		got[k.Kod] = k.Tutar
	}
	if got["STOPAJ"] != 30 {
		t.Errorf("stopaj = E×%%3 = 30 olmalı, %v", got["STOPAJ"])
	}
	if got["KDV_TEVKIFAT"] != 80 {
		t.Errorf("tevkifat = F×%%40 = 80 olmalı, %v", got["KDV_TEVKIFAT"])
	}
}
