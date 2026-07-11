package notify

import (
	"reflect"
	"testing"
	"time"
)

func TestParseMentions(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"tek mention", "merhaba @ahmet, kontrol eder misin", []string{"ahmet"}},
		{"çoklu + tekrar", "@ahmet @mehmet lütfen bakın, @ahmet acil", []string{"ahmet", "mehmet"}},
		{"satır başı", "@saha.muh1 rapor hazır", []string{"saha.muh1"}},
		{"e-posta eşleşmez", "iletişim: ali@ornek.com üzerinden", []string{}},
		{"nokta/tire/altçizgi", "cc @ali_veli ve @ayse-k ve @m.demir", []string{"ali_veli", "ayse-k", "m.demir"}},
		{"mention yok", "normal bir yorum", []string{}},
		{"parantez içinde", "(@pm1 onayı gerekli)", []string{"pm1"}},
		{"çift @@ tek sayılır", "@@ahmet", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseMentions(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ParseMentions(%q) = %v, beklenen %v", c.in, got, c.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 30 * time.Second},
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{10, time.Hour}, // tavan
	}
	for _, c := range cases {
		if got := Backoff(c.attempt); got != c.want {
			t.Fatalf("Backoff(%d) = %v, beklenen %v", c.attempt, got, c.want)
		}
	}
}
