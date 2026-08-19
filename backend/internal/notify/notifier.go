package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

// Notifier renders a template, delivers it, and records what happened. Every
// message is logged whether or not delivery succeeded, so the admin email log
// is a complete account of what the system tried to send.
type Notifier struct {
	pool        *db.DB
	mailer      Mailer
	companyName string
	baseURL     string
}

func New(pool *db.DB, mailer Mailer, companyName, baseURL string) *Notifier {
	return &Notifier{pool: pool, mailer: mailer, companyName: companyName, baseURL: baseURL}
}

// SendReservationEmail is the one entry point the worker uses.
func (n *Notifier) SendReservationEmail(ctx context.Context, template string, res *rental.Reservation, extra Data) error {
	if res == nil {
		return fmt.Errorf("cannot send %s: no reservation", template)
	}
	if res.CustomerEmail == "" {
		return fmt.Errorf("customer %d has no email address", res.CustomerID)
	}

	data := extra
	data.Reservation = res
	data.CompanyName = n.companyName
	data.BaseURL = n.baseURL

	subject, htmlBody, textBody, err := Render(template, data)
	if err != nil {
		return err
	}

	msg := Message{
		To: res.CustomerEmail, ToName: res.CustomerName, Subject: subject,
		HTMLBody: htmlBody, TextBody: textBody, Template: template, ReservationID: &res.ID,
	}

	sendErr := n.mailer.Send(ctx, msg)
	if logErr := n.log(ctx, msg, sendErr); logErr != nil {
		slog.Error("could not record sent email", "err", logErr, "template", template)
	}
	if sendErr != nil {
		return fmt.Errorf("delivering %s to %s: %w", template, res.CustomerEmail, sendErr)
	}
	slog.Info("email sent", "template", template, "to", res.CustomerEmail,
		"reservation", res.ReservationNumber)
	return nil
}

func (n *Notifier) log(ctx context.Context, m Message, sendErr error) error {
	status, errText := "sent", ""
	if sendErr != nil {
		status, errText = "failed", sendErr.Error()
	}
	_, err := n.pool.Exec(ctx, `
		INSERT INTO emails (to_address, to_name, subject, template, body_text, body_html,
			reservation_id, status, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		m.To, m.ToName, m.Subject, m.Template, m.TextBody, m.HTMLBody,
		m.ReservationID, status, errText)
	return err
}

// SentEmail is a row of the outbound log, shown in the admin interface.
type SentEmail struct {
	ID            int64     `json:"id"`
	ToAddress     string    `json:"to_address"`
	ToName        string    `json:"to_name"`
	Subject       string    `json:"subject"`
	Template      string    `json:"template"`
	BodyText      string    `json:"body_text"`
	BodyHTML      string    `json:"body_html"`
	ReservationID *int64    `json:"reservation_id"`
	Status        string    `json:"status"`
	Error         string    `json:"error"`
	CreatedAt     time.Time `json:"created_at"`
}

type LogFilter struct {
	Template      string
	Status        string
	ReservationID int64
	IncludeBodies bool
	Limit         int
	Offset        int
}

func (n *Notifier) Log(ctx context.Context, f LogFilter) ([]SentEmail, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	rows, err := n.pool.Query(ctx, `
		SELECT id, to_address, to_name, subject, template,
		       CASE WHEN $4 THEN body_text ELSE '' END,
		       CASE WHEN $4 THEN body_html ELSE '' END,
		       reservation_id, status, error, created_at,
		       COUNT(*) OVER () AS total
		FROM emails
		WHERE ($1 = '' OR template = $1)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = 0 OR reservation_id = $3)
		ORDER BY created_at DESC, id DESC
		LIMIT $5 OFFSET $6`,
		f.Template, f.Status, f.ReservationID, f.IncludeBodies, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("reading email log: %w", err)
	}
	defer rows.Close()

	out := []SentEmail{}
	total := 0
	for rows.Next() {
		var e SentEmail
		if err := rows.Scan(&e.ID, &e.ToAddress, &e.ToName, &e.Subject, &e.Template,
			&e.BodyText, &e.BodyHTML, &e.ReservationID, &e.Status, &e.Error,
			&e.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (n *Notifier) GetLogEntry(ctx context.Context, id int64) (*SentEmail, error) {
	var e SentEmail
	err := n.pool.QueryRow(ctx, `
		SELECT id, to_address, to_name, subject, template, body_text, body_html,
		       reservation_id, status, error, created_at
		FROM emails WHERE id = $1`, id).
		Scan(&e.ID, &e.ToAddress, &e.ToName, &e.Subject, &e.Template, &e.BodyText,
			&e.BodyHTML, &e.ReservationID, &e.Status, &e.Error, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
