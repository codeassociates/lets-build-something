package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
)

// seedEmails backfills the message log so the admin email view has history
// from the moment the system is first opened, rather than filling up only
// after the worker has been running for a week.
func (s *Seeder) seedEmails(ctx context.Context, q db.Querier, resID, customerID int64,
	number string, p plan, start, end, createdAt time.Time) error {

	s.logEmail(ctx, q, customerID, resID, "booking_confirmation",
		fmt.Sprintf("Booking confirmed — %s", number), createdAt)

	pickupReminder := start.AddDate(0, 0, -1).Add(9 * time.Hour)
	if pickupReminder.Before(s.today.Add(24*time.Hour)) && !pickupReminder.After(time.Now().UTC()) {
		s.logEmail(ctx, q, customerID, resID, "pickup_reminder",
			fmt.Sprintf("Pickup tomorrow — %s", number), pickupReminder)
	}

	if p.status == "picked_up" || p.status == "returned" {
		returnReminder := end.AddDate(0, 0, -1).Add(9 * time.Hour)
		if !returnReminder.After(time.Now().UTC()) {
			s.logEmail(ctx, q, customerID, resID, "return_reminder",
				fmt.Sprintf("Return due %s — %s", end.Format("2 Jan"), number), returnReminder)
		}
	}

	// Overdue rentals have been chased once a day since they went late.
	if p.status == "picked_up" && end.Before(s.today) {
		for day := end.AddDate(0, 0, 1); !day.After(s.today); day = day.AddDate(0, 0, 1) {
			daysLate := int(day.Sub(end).Hours() / 24)
			s.logEmail(ctx, q, customerID, resID, "overdue_notice",
				fmt.Sprintf("Overdue: %s — %d day%s late", number, daysLate,
					pluralSuffix(daysLate)), day.Add(7*time.Hour))
		}
	}
	return nil
}

func (s *Seeder) logEmail(ctx context.Context, q db.Querier, customerID, resID int64,
	template, subject string, at time.Time) {

	var email, name string
	if err := q.QueryRow(ctx, `SELECT email, full_name FROM users WHERE id=$1`, customerID).
		Scan(&email, &name); err != nil {
		return
	}

	// A small share of messages bounce, so the log has something to show for
	// its failure filter.
	status, errText := "sent", ""
	if s.rng.Intn(40) == 0 {
		status = "failed"
		errText = "550 5.1.1 recipient address rejected: user unknown"
	}

	q.Exec(ctx, `
		INSERT INTO emails (to_address, to_name, subject, template, body_text, body_html,
			reservation_id, status, error, created_at)
		VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,$9)`,
		email, name, subject, template,
		fmt.Sprintf("(Seeded message — the live text is generated when the worker sends %s.)", template),
		resID, status, errText, at)
}
