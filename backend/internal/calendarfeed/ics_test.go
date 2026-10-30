package calendarfeed

import (
	"strings"
	"testing"
)

func TestEscapeText(t *testing.T) {
	cases := map[string]string{
		"a,b;c":       `a\,b\;c`,
		`d\e`:         `d\\e`,
		"satır\nyeni": `satır\nyeni`,
		"x\r\ny":      `x\ny`,
	}
	for in, want := range cases {
		if got := escapeText(in); got != want {
			t.Errorf("escapeText(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFoldLineKeepsRunesAndLimit(t *testing.T) {
	long := "Ödeme (havale): " + strings.Repeat("ğüşıöç", 30)
	folded := foldLine(long)
	for _, part := range strings.Split(folded, "\r\n") {
		if len(part) > 75 {
			t.Fatalf("satır 75 bayttan uzun: %d", len(part))
		}
	}
	unfolded := strings.ReplaceAll(folded, "\r\n ", "")
	if unfolded != long {
		t.Fatal("katlama içeriği bozdu")
	}
}

func TestFoldLineShortUnchanged(t *testing.T) {
	if got := foldLine("SUMMARY:kısa"); got != "SUMMARY:kısa" {
		t.Fatalf("kısa satır değişti: %q", got)
	}
}

func TestHashTokenStable(t *testing.T) {
	a, b := hashToken("abc"), hashToken("abc")
	if a != b || len(a) != 64 || a == "abc" {
		t.Fatalf("özet beklenen gibi değil: %s", a)
	}
}

func TestNewTokenUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok, err := newToken()
		if err != nil || len(tok) < 40 || seen[tok] {
			t.Fatalf("belirteç üretimi hatalı: %q %v", tok, err)
		}
		seen[tok] = true
	}
}
