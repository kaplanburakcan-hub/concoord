package notify

import (
	"fmt"
	"mime"
	"net/smtp"
	"strings"
)

// EmailSender — SMTP göndericisi (Plan §2: SMTP). Host boşsa gönderim atlanır
// ve loglanır (geliştirme ortamı); worker işi "başarılı" sayar ki kuyruk dolmasın.
type EmailSender struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func (e *EmailSender) Configured() bool { return e.Host != "" && e.From != "" }

func (e *EmailSender) Send(to, subject, body string) error {
	if !e.Configured() {
		return fmt.Errorf("smtp yapılandırılmamış")
	}
	addr := e.Host + ":" + e.Port
	// Türkçe karakterler için konu satırı RFC 2047 ile kodlanır.
	encSubject := mime.QEncoding.Encode("utf-8", subject)
	msg := strings.Join([]string{
		"From: " + e.From,
		"To: " + to,
		"Subject: " + encSubject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	var auth smtp.Auth
	if e.User != "" {
		auth = smtp.PlainAuth("", e.User, e.Pass, e.Host)
	}
	return smtp.SendMail(addr, auth, e.From, []string{to}, []byte(msg))
}
