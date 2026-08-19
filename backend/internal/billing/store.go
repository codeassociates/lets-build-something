package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

type Invoice struct {
	ID                int64         `json:"id"`
	InvoiceNumber     string        `json:"invoice_number"`
	ReservationID     int64         `json:"reservation_id"`
	ReservationNumber string        `json:"reservation_number"`
	CustomerID        int64         `json:"customer_id"`
	CustomerName      string        `json:"customer_name"`
	CustomerEmail     string        `json:"customer_email"`
	Status            string        `json:"status"`
	IssuedAt          time.Time     `json:"issued_at"`
	DueAt             time.Time     `json:"due_at"`
	SubtotalCents     money.Cents   `json:"subtotal_cents"`
	TaxCents          money.Cents   `json:"tax_cents"`
	TotalCents        money.Cents   `json:"total_cents"`
	AmountPaidCents   money.Cents   `json:"amount_paid_cents"`
	Lines             []InvoiceLine `json:"lines"`
	Payments          []Payment     `json:"payments"`
}

// BalanceCents is what the customer still owes.
func (i Invoice) BalanceCents() money.Cents { return i.TotalCents - i.AmountPaidCents }

type InvoiceLine struct {
	ID              int64       `json:"id"`
	Kind            string      `json:"kind"`
	Description     string      `json:"description"`
	Quantity        int         `json:"quantity"`
	UnitAmountCents money.Cents `json:"unit_amount_cents"`
	AmountCents     money.Cents `json:"amount_cents"`
	SortOrder       int         `json:"sort_order"`
}

type Payment struct {
	ID            int64       `json:"id"`
	InvoiceID     *int64      `json:"invoice_id"`
	ReservationID *int64      `json:"reservation_id"`
	CustomerID    int64       `json:"customer_id"`
	Kind          string      `json:"kind"`
	AmountCents   money.Cents `json:"amount_cents"`
	Status        string      `json:"status"`
	Gateway       string      `json:"gateway"`
	GatewayRef    string      `json:"gateway_ref"`
	CardBrand     string      `json:"card_brand"`
	CardLast4     string      `json:"card_last4"`
	FailureReason string      `json:"failure_reason"`
	CreatedAt     time.Time   `json:"created_at"`
}

type Store struct {
	pool    *db.DB
	gateway Gateway
}

func NewStore(pool *db.DB, gw Gateway) *Store { return &Store{pool: pool, gateway: gw} }

func (s *Store) Gateway() Gateway { return s.gateway }

type NewInvoice struct {
	ReservationID int64
	CustomerID    int64
	Status        string
	DueAt         time.Time
	TaxCents      money.Cents
	Lines         []InvoiceLine
}

// CreateInvoice writes an invoice and its lines, taking a Querier so it can join
// a caller's transaction — checking a rental back in creates the final invoice
// as part of the same atomic step.
func (s *Store) CreateInvoice(ctx context.Context, q db.Querier, in NewInvoice) (*Invoice, error) {
	if in.Status == "" {
		in.Status = "issued"
	}
	if in.DueAt.IsZero() {
		in.DueAt = time.Now().AddDate(0, 0, 14)
	}

	var subtotal money.Cents
	for _, l := range in.Lines {
		subtotal += l.AmountCents
	}
	total := subtotal + in.TaxCents

	var inv Invoice
	err := q.QueryRow(ctx, `
		INSERT INTO invoices (invoice_number, reservation_id, customer_id, status,
			due_at, subtotal_cents, tax_cents, total_cents)
		VALUES ('', $1, $2, $3, $4, $5, $6, $7)
		RETURNING id, issued_at`,
		in.ReservationID, in.CustomerID, in.Status, in.DueAt, subtotal, in.TaxCents, total).
		Scan(&inv.ID, &inv.IssuedAt)
	if err != nil {
		return nil, fmt.Errorf("creating invoice: %w", err)
	}

	// The human-facing number needs the id, so it is stamped immediately after.
	number := fmt.Sprintf("INV-%s-%05d", time.Now().UTC().Format("2006"), inv.ID)
	if _, err := q.Exec(ctx, `UPDATE invoices SET invoice_number = $2 WHERE id = $1`,
		inv.ID, number); err != nil {
		return nil, fmt.Errorf("numbering invoice: %w", err)
	}

	for i, l := range in.Lines {
		if l.Quantity == 0 {
			l.Quantity = 1
		}
		if _, err := q.Exec(ctx, `
			INSERT INTO invoice_lines (invoice_id, kind, description, quantity,
				unit_amount_cents, amount_cents, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			inv.ID, l.Kind, l.Description, l.Quantity, l.UnitAmountCents, l.AmountCents, i); err != nil {
			return nil, fmt.Errorf("adding invoice line: %w", err)
		}
	}

	inv.InvoiceNumber = number
	inv.ReservationID = in.ReservationID
	inv.CustomerID = in.CustomerID
	inv.Status = in.Status
	inv.DueAt = in.DueAt
	inv.SubtotalCents = subtotal
	inv.TaxCents = in.TaxCents
	inv.TotalCents = total
	inv.Lines = in.Lines
	return &inv, nil
}

const invoiceColumns = `i.id, i.invoice_number, i.reservation_id, r.reservation_number,
	i.customer_id, u.full_name, u.email, i.status, i.issued_at, i.due_at,
	i.subtotal_cents, i.tax_cents, i.total_cents, i.amount_paid_cents`

func scanInvoice(row pgx.Row, extra ...any) (*Invoice, error) {
	var i Invoice
	dest := []any{&i.ID, &i.InvoiceNumber, &i.ReservationID, &i.ReservationNumber,
		&i.CustomerID, &i.CustomerName, &i.CustomerEmail, &i.Status, &i.IssuedAt,
		&i.DueAt, &i.SubtotalCents, &i.TaxCents, &i.TotalCents, &i.AmountPaidCents}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	i.Lines = []InvoiceLine{}
	i.Payments = []Payment{}
	return &i, nil
}

type InvoiceFilter struct {
	CustomerID int64
	Status     string
	UnpaidOnly bool
	Limit      int
	Offset     int
}

func (s *Store) ListInvoices(ctx context.Context, f InvoiceFilter) ([]Invoice, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+invoiceColumns+`, COUNT(*) OVER () AS total
		FROM invoices i
		JOIN reservations r ON r.id = i.reservation_id
		JOIN users u ON u.id = i.customer_id
		WHERE ($1 = 0 OR i.customer_id = $1)
		  AND ($2 = '' OR i.status = $2)
		  AND (NOT $3 OR (i.amount_paid_cents < i.total_cents AND i.status <> 'void'))
		ORDER BY i.issued_at DESC
		LIMIT $4 OFFSET $5`,
		f.CustomerID, f.Status, f.UnpaidOnly, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()

	out := []Invoice{}
	total := 0
	for rows.Next() {
		inv, err := scanInvoice(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *inv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// An invoice without its lines is not an invoice anyone can read, and both
	// the customer's invoice list and the desk render them in full. Two batch
	// queries fill the whole page rather than two per row.
	if err := s.attachLinesBatch(ctx, out); err != nil {
		return nil, 0, err
	}
	if err := s.attachPaymentsBatch(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// attachLinesBatch loads the lines for a page of invoices in one query.
func (s *Store) attachLinesBatch(ctx context.Context, invoices []Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	ids := make([]int64, len(invoices))
	byID := make(map[int64]*Invoice, len(invoices))
	for i := range invoices {
		ids[i] = invoices[i].ID
		byID[invoices[i].ID] = &invoices[i]
	}

	rows, err := s.pool.Query(ctx, `
		SELECT invoice_id, id, kind, description, quantity, unit_amount_cents,
		       amount_cents, sort_order
		FROM invoice_lines WHERE invoice_id = ANY($1)
		ORDER BY invoice_id, sort_order, id`, ids)
	if err != nil {
		return fmt.Errorf("loading invoice lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var invoiceID int64
		var l InvoiceLine
		if err := rows.Scan(&invoiceID, &l.ID, &l.Kind, &l.Description, &l.Quantity,
			&l.UnitAmountCents, &l.AmountCents, &l.SortOrder); err != nil {
			return err
		}
		if inv, ok := byID[invoiceID]; ok {
			inv.Lines = append(inv.Lines, l)
		}
	}
	return rows.Err()
}

func (s *Store) attachPaymentsBatch(ctx context.Context, invoices []Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	ids := make([]int64, len(invoices))
	byID := make(map[int64]*Invoice, len(invoices))
	for i := range invoices {
		ids[i] = invoices[i].ID
		byID[invoices[i].ID] = &invoices[i]
	}

	rows, err := s.pool.Query(ctx, `
		SELECT invoice_id, id, reservation_id, customer_id, kind, amount_cents, status,
		       gateway, gateway_ref, card_brand, card_last4, failure_reason, created_at
		FROM payments WHERE invoice_id = ANY($1)
		ORDER BY created_at DESC, id DESC`, ids)
	if err != nil {
		return fmt.Errorf("loading invoice payments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var invoiceID int64
		var p Payment
		if err := rows.Scan(&invoiceID, &p.ID, &p.ReservationID, &p.CustomerID, &p.Kind,
			&p.AmountCents, &p.Status, &p.Gateway, &p.GatewayRef, &p.CardBrand,
			&p.CardLast4, &p.FailureReason, &p.CreatedAt); err != nil {
			return err
		}
		p.InvoiceID = &invoiceID
		if inv, ok := byID[invoiceID]; ok {
			inv.Payments = append(inv.Payments, p)
		}
	}
	return rows.Err()
}

func (s *Store) GetInvoice(ctx context.Context, id int64) (*Invoice, error) {
	inv, err := scanInvoice(s.pool.QueryRow(ctx, `
		SELECT `+invoiceColumns+`
		FROM invoices i
		JOIN reservations r ON r.id = i.reservation_id
		JOIN users u ON u.id = i.customer_id
		WHERE i.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading invoice: %w", err)
	}

	if err := s.attachLines(ctx, inv); err != nil {
		return nil, err
	}
	return inv, s.attachPayments(ctx, inv)
}

func (s *Store) attachLines(ctx context.Context, inv *Invoice) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, description, quantity, unit_amount_cents, amount_cents, sort_order
		FROM invoice_lines WHERE invoice_id = $1 ORDER BY sort_order, id`, inv.ID)
	if err != nil {
		return fmt.Errorf("loading invoice lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var l InvoiceLine
		if err := rows.Scan(&l.ID, &l.Kind, &l.Description, &l.Quantity,
			&l.UnitAmountCents, &l.AmountCents, &l.SortOrder); err != nil {
			return err
		}
		inv.Lines = append(inv.Lines, l)
	}
	return rows.Err()
}

func (s *Store) attachPayments(ctx context.Context, inv *Invoice) error {
	payments, err := s.ListPayments(ctx, PaymentFilter{InvoiceID: inv.ID})
	if err != nil {
		return err
	}
	inv.Payments = payments
	return nil
}

type PaymentFilter struct {
	InvoiceID     int64
	ReservationID int64
	CustomerID    int64
	Limit         int
}

func (s *Store) ListPayments(ctx context.Context, f PaymentFilter) ([]Payment, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, invoice_id, reservation_id, customer_id, kind, amount_cents, status,
		       gateway, gateway_ref, card_brand, card_last4, failure_reason, created_at
		FROM payments
		WHERE ($1 = 0 OR invoice_id = $1)
		  AND ($2 = 0 OR reservation_id = $2)
		  AND ($3 = 0 OR customer_id = $3)
		ORDER BY created_at DESC, id DESC
		LIMIT $4`, f.InvoiceID, f.ReservationID, f.CustomerID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("listing payments: %w", err)
	}
	defer rows.Close()

	out := []Payment{}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.InvoiceID, &p.ReservationID, &p.CustomerID, &p.Kind,
			&p.AmountCents, &p.Status, &p.Gateway, &p.GatewayRef, &p.CardBrand,
			&p.CardLast4, &p.FailureReason, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordPayment stores the outcome of a gateway call, successful or not. Failed
// attempts are kept deliberately: staff need to see that a card was declined.
func (s *Store) RecordPayment(ctx context.Context, q db.Querier, p Payment) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, `
		INSERT INTO payments (invoice_id, reservation_id, customer_id, kind, amount_cents,
			status, gateway, gateway_ref, card_brand, card_last4, failure_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		p.InvoiceID, p.ReservationID, p.CustomerID, p.Kind, p.AmountCents, p.Status,
		p.Gateway, p.GatewayRef, p.CardBrand, p.CardLast4, p.FailureReason).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("recording payment: %w", err)
	}
	return id, nil
}

var ErrAlreadySettled = errors.New("this invoice has already been settled")

// PayInvoice charges a card for the outstanding balance and settles the invoice.
// Authorization and capture happen together, which is how a counter terminal
// behaves for a payment that is due now.
func (s *Store) PayInvoice(ctx context.Context, invoiceID int64, card Card) (*Invoice, error) {
	inv, err := s.GetInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, pgx.ErrNoRows
	}
	if inv.Status == "void" {
		return nil, fmt.Errorf("this invoice has been voided")
	}
	balance := inv.BalanceCents()
	if balance <= 0 {
		return nil, ErrAlreadySettled
	}

	charge, err := s.gateway.Authorize(ctx, balance, card)
	if err != nil {
		// Record the decline so it is visible in the customer's payment history.
		s.RecordPayment(ctx, s.pool, Payment{
			InvoiceID: &inv.ID, ReservationID: &inv.ReservationID, CustomerID: inv.CustomerID,
			Kind: "authorization", AmountCents: balance, Status: "failed",
			Gateway: s.gateway.Name(), CardBrand: card.Brand(), CardLast4: card.Last4(),
			FailureReason: err.Error(),
		})
		return nil, err
	}
	if _, err := s.gateway.Capture(ctx, charge.Reference, balance); err != nil {
		return nil, fmt.Errorf("capturing payment: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := s.RecordPayment(ctx, tx, Payment{
		InvoiceID: &inv.ID, ReservationID: &inv.ReservationID, CustomerID: inv.CustomerID,
		Kind: "capture", AmountCents: balance, Status: "succeeded",
		Gateway: s.gateway.Name(), GatewayRef: charge.Reference,
		CardBrand: charge.Brand, CardLast4: charge.Last4,
	}); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE invoices
		SET amount_paid_cents = amount_paid_cents + $2,
		    status = CASE WHEN amount_paid_cents + $2 >= total_cents THEN 'paid' ELSE status END,
		    updated_at = now()
		WHERE id = $1`, inv.ID, balance); err != nil {
		return nil, fmt.Errorf("settling invoice: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, inv.ID)
}

// RefundInvoice returns money against the most recent successful capture.
func (s *Store) RefundInvoice(ctx context.Context, invoiceID int64, amount money.Cents) (*Invoice, error) {
	inv, err := s.GetInvoice(ctx, invoiceID)
	if err != nil || inv == nil {
		return nil, err
	}
	if amount <= 0 || amount > inv.AmountPaidCents {
		return nil, fmt.Errorf("refund must be between $0.01 and %s", inv.AmountPaidCents)
	}

	var ref string
	for _, p := range inv.Payments {
		if p.Kind == "capture" && p.Status == "succeeded" && p.GatewayRef != "" {
			ref = p.GatewayRef
			break
		}
	}
	if ref == "" {
		return nil, fmt.Errorf("there is no captured payment to refund")
	}
	charge, err := s.gateway.Refund(ctx, ref, amount)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := s.RecordPayment(ctx, tx, Payment{
		InvoiceID: &inv.ID, ReservationID: &inv.ReservationID, CustomerID: inv.CustomerID,
		Kind: "refund", AmountCents: amount, Status: "succeeded",
		Gateway: s.gateway.Name(), GatewayRef: charge.Reference,
		CardBrand: charge.Brand, CardLast4: charge.Last4,
	}); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE invoices
		SET amount_paid_cents = amount_paid_cents - $2,
		    status = CASE WHEN amount_paid_cents - $2 < total_cents AND status = 'paid'
		                  THEN 'issued' ELSE status END,
		    updated_at = now()
		WHERE id = $1`, inv.ID, amount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, inv.ID)
}
