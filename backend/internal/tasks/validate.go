// Package tasks — Faz 4 görev yönetimi (Kanban, atama, yorum/@mention).
package tasks

import "strings"

var validStatuses = map[string]bool{
	"Backlog": true, "Todo": true, "InProgress": true, "Review": true, "Done": true,
}

var validPriorities = map[string]bool{
	"Low": true, "Normal": true, "High": true, "Urgent": true,
}

// Statü sırası — Kanban kolon dizilimi (UI ve raporlama aynı sırayı kullanır).
var StatusOrder = []string{"Backlog", "Todo", "InProgress", "Review", "Done"}

func ValidStatus(s string) bool   { return validStatuses[s] }
func ValidPriority(p string) bool { return validPriorities[p] }

// ValidateTitle — başlık zorunlu, 1..300 karakter (trim sonrası).
func ValidateTitle(t string) (string, bool) {
	t = strings.TrimSpace(t)
	if t == "" || len([]rune(t)) > 300 {
		return t, false
	}
	return t, true
}

// ValidateCommentBody — yorum gövdesi zorunlu, 1..4000 karakter.
func ValidateCommentBody(b string) (string, bool) {
	b = strings.TrimSpace(b)
	if b == "" || len([]rune(b)) > 4000 {
		return b, false
	}
	return b, true
}
