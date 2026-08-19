// Package notify renders and delivers the messages the system sends: booking
// confirmations, pickup and return reminders, overdue notices, and receipts.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Message is one email, ready to send.
type Message struct {
	To            string
	ToName        string
	Subject       string
	TextBody      string
	HTMLBody      string
	Template      string
	ReservationID *int64
}

// Mailer is the delivery transport. SMTP in every environment we run; swapping
// in a hosted API is a matter of implementing this one method.
type Mailer interface {
	Send(ctx context.Context, m Message) error
	Name() string
}

// SMTPMailer talks plain SMTP with no authentication, which is what Mailpit
// wants locally and what an internal relay wants in production. Credentials are
// supplied only when the server asks for them.
type SMTPMailer struct {
	Host     string
	Port     int
	From     string
	FromName string
	Username string
	Password string
}

func (m *SMTPMailer) Name() string { return fmt.Sprintf("smtp://%s:%d", m.Host, m.Port) }

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	body := m.compose(msg)
	if err := smtp.SendMail(addr, auth, m.From, []string{msg.To}, body); err != nil {
		return fmt.Errorf("sending mail via %s: %w", addr, err)
	}
	return nil
}

// compose builds a multipart/alternative message so a plain-text reader and an
// HTML client both get something sensible.
func (m *SMTPMailer) compose(msg Message) []byte {
	boundary := "----=_Part_" + fmt.Sprint(time.Now().UnixNano())
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s <%s>\r\n", mime.QEncoding.Encode("utf-8", m.FromName), m.From)
	if msg.ToName != "" {
		fmt.Fprintf(&b, "To: %s <%s>\r\n", mime.QEncoding.Encode("utf-8", msg.ToName), msg.To)
	} else {
		fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "X-Rental-Template: %s\r\n", msg.Template)
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)

	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n\r\n",
		boundary, msg.TextBody)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n\r\n",
		boundary, msg.HTMLBody)
	fmt.Fprintf(&b, "--%s--\r\n", boundary)

	return []byte(b.String())
}

// MemoryMailer keeps messages in memory instead of sending them. Tests use it,
// and it is what runs when SMTP is switched off.
type MemoryMailer struct {
	mu   sync.Mutex
	sent []Message
}

func NewMemoryMailer() *MemoryMailer { return &MemoryMailer{} }

func (m *MemoryMailer) Name() string { return "memory" }

func (m *MemoryMailer) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	slog.Info("email captured (not delivered)", "to", msg.To, "subject", msg.Subject)
	return nil
}

func (m *MemoryMailer) Sent() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Message(nil), m.sent...)
}
