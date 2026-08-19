package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

type invoiceLine struct {
	kind        string
	description string
	quantity    int
	unitAmount  money.Cents
	amount      money.Cents
}

func (s *Seeder) createInvoice(ctx context.Context, q db.Querier, resID, customerID int64,
	status string, issuedAt time.Time, tax money.Cents, lines []invoiceLine) (int64, error) {

	var subtotal money.Cents
	for _, l := range lines {
		subtotal += l.amount
	}
	total := subtotal + tax

	var id int64
	err := q.QueryRow(ctx, `
		INSERT INTO invoices (invoice_number, reservation_id, customer_id, status,
			issued_at, due_at, subtotal_cents, tax_cents, total_cents, created_at, updated_at)
		VALUES ('', $1,$2,$3,$4,$5,$6,$7,$8,$4,$4) RETURNING id`,
		resID, customerID, status, issuedAt, issuedAt.AddDate(0, 0, 14),
		subtotal, tax, total).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating invoice: %w", err)
	}
	if _, err := q.Exec(ctx, `UPDATE invoices SET invoice_number=$2 WHERE id=$1`,
		id, fmt.Sprintf("INV-%s-%05d", issuedAt.Format("2006"), id)); err != nil {
		return 0, err
	}

	for i, l := range lines {
		if l.quantity == 0 {
			l.quantity = 1
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO invoice_lines (invoice_id, kind, description, quantity,
				unit_amount_cents, amount_cents, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			id, l.kind, l.description, l.quantity, l.unitAmount, l.amount, i); err != nil {
			return 0, err
		}
	}
	return id, nil
}

type payment struct {
	invoiceID     *int64
	reservationID int64
	customerID    int64
	kind          string
	amount        money.Cents
	at            time.Time
}

// Card brands are spread across the customer base so the payments list does not
// look like everyone banks in the same place.
var cardBrands = []struct{ brand, prefix string }{
	{"Visa", "4"}, {"Mastercard", "5"}, {"American Express", "3"}, {"Discover", "6"},
}

func (s *Seeder) recordPayment(ctx context.Context, q db.Querier, p payment) error {
	card := cardBrands[s.rng.Intn(len(cardBrands))]
	last4 := fmt.Sprintf("%04d", s.rng.Intn(10000))

	_, err := q.Exec(ctx, `
		INSERT INTO payments (invoice_id, reservation_id, customer_id, kind, amount_cents,
			status, gateway, gateway_ref, card_brand, card_last4, created_at)
		VALUES ($1,$2,$3,$4,$5,'succeeded','fake',$6,$7,$8,$9)`,
		p.invoiceID, p.reservationID, p.customerID, p.kind, p.amount,
		fmt.Sprintf("auth_%012x", s.rng.Int63()), card.brand, last4, p.at)
	return err
}
