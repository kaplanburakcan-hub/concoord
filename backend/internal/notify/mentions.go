package notify

import "regexp"

// @mention deseni: @ + kullanıcı adı (harf/rakam/._-). Kelime içindeki @
// (e-posta adresleri gibi) eşleşmez: @ öncesi kelime karakteri olamaz.
var mentionRe = regexp.MustCompile(`(^|[^\w@])@([a-zA-Z0-9._-]+)`)

// ParseMentions — metindeki @kullanıcıadı geçişlerini (tekilleştirilmiş,
// sıra korunarak) döner. Kullanıcı adlarının varlığı çağıranda DB'den doğrulanır.
func ParseMentions(body string) []string {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		u := m[2]
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}
