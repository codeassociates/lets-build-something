package rental

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/catalog"
	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/jobs"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

// advisoryNamespace keeps our booking locks from colliding with any other
// advisory lock taken against this database.
const advisoryNamespace = 4711

type Service struct {
	pool    *db.DB
	catalog *catalog.Store
	billing *billing.Store
	queue   *jobs.Queue

	taxRatePercent     float64
	lateFeeMultiple    float64
	pickupReminderLead time.Duration
	returnReminderLead time.Duration
}

type Options struct {
	TaxRatePercent     float64
	LateFeeMultiple    float64
	PickupReminderLead time.Duration
	ReturnReminderLead time.Duration
}

func NewService(pool *db.DB, cat *catalog.Store, bill *billing.Store, q *jobs.Queue, o Options) *Service {
	return &Service{
		pool: pool, catalog: cat, billing: bill, queue: q,
		taxRatePercent:     o.TaxRatePercent,
		lateFeeMultiple:    o.LateFeeMultiple,
		pickupReminderLead: o.PickupReminderLead,
		returnReminderLead: o.ReturnReminderLead,
	}
}

var (
	ErrUnavailable   = errors.New("not enough stock for those dates")
	ErrBadTransition = errors.New("that is not a valid next step for this reservation")
	ErrNotFound      = errors.New("reservation not found")
)

// ---------- quoting ----------

// Quote prices a basket without reserving anything. The shop calls it on every
// change of dates or quantities, so it must not have side effects.
func (s *Service) Quote(ctx context.Context, req QuoteRequest) (*Quote, error) {
	start, end, err := validateWindow(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if len(req.Items) == 0 {
		return nil, httpx.BadRequest("Add at least one item to get a quote.")
	}

	days := money.RentalDays(start, end)
	q := &Quote{
		StartDate: httpx.Date(start), EndDate: httpx.Date(end),
		RentalDays: days, Lines: []QuoteLine{}, AllAvailable: true,
	}

	for _, item := range req.Items {
		if item.Quantity < 1 {
			return nil, httpx.BadRequest("Quantities must be at least 1.")
		}
		model, err := s.catalog.Get(ctx, item.ModelID, start, end)
		if err != nil {
			return nil, err
		}
		if model == nil || !model.Active {
			return nil, httpx.BadRequest(fmt.Sprintf("Item %d is not available to rent.", item.ModelID))
		}

		priced := money.BestQuote(model.RateCard(), days)
		line := QuoteLine{
			ModelID: model.ID, ModelName: model.Name, SKU: model.SKU,
			ImageURL: model.ImageURL, Quantity: item.Quantity,
			RateBasis: priced.Basis, RateCents: priced.Rate,
			BillablePeriods: priced.Periods,
			LineTotalCents:  priced.Subtotal * money.Cents(item.Quantity),
			DepositCents:    model.DepositCents * money.Cents(item.Quantity),
			RequiresLicense: model.RequiresLicense,
			AvailableUnits:  model.AvailableUnits,
			Available:       model.AvailableUnits >= item.Quantity,
		}
		if !line.Available {
			q.AllAvailable = false
		}
		q.SubtotalCents += line.LineTotalCents
		q.DepositCents += line.DepositCents
		q.Lines = append(q.Lines, line)
	}

	q.TaxCents = money.Tax(q.SubtotalCents, s.taxRatePercent)
	q.TotalCents = q.SubtotalCents + q.TaxCents
	q.DueNowCents = q.TotalCents + q.DepositCents
	return q, nil
}

// ---------- booking ----------

type BookRequest struct {
	QuoteRequest
	CustomerID int64
	Notes      string
	CreatedBy  int64
	// Card is optional: staff booking over the phone can take payment later.
	Card *billing.Card
}

type BookResult struct {
	Reservation *Reservation     `json:"reservation"`
	Invoice     *billing.Invoice `json:"invoice"`
	DepositHeld money.Cents      `json:"deposit_held_cents"`
	// PaymentError is set when the booking stands but the card did not go
	// through, so the desk knows to chase payment rather than the booking.
	PaymentError string `json:"payment_error,omitempty"`
}

// Book takes a reservation. Availability is re-checked inside the transaction
// under a per-model advisory lock, so two customers racing for the last
// jackhammer cannot both win.
func (s *Service) Book(ctx context.Context, req BookRequest) (*BookResult, error) {
	quote, err := s.Quote(ctx, req.QuoteRequest)
	if err != nil {
		return nil, err
	}

	start, end := quote.StartDate.Time(), quote.EndDate.Time()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the models in a stable order so concurrent bookings of overlapping
	// baskets cannot deadlock against each other.
	for _, id := range sortedModelIDs(req.Items) {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, $2)`,
			advisoryNamespace, int32(id)); err != nil {
			return nil, fmt.Errorf("locking model %d: %w", id, err)
		}
	}

	for _, line := range quote.Lines {
		available, err := s.catalog.AvailableIn(ctx, tx, line.ModelID, start, end, nil)
		if err != nil {
			return nil, err
		}
		if available < line.Quantity {
			return nil, fmt.Errorf("%w: %s has %d available for %s to %s, not %d",
				ErrUnavailable, line.ModelName, available,
				start.Format("2 Jan"), end.Format("2 Jan"), line.Quantity)
		}
	}

	var res Reservation
	err = tx.QueryRow(ctx, `
		INSERT INTO reservations (reservation_number, customer_id, status, start_date,
			end_date, subtotal_cents, tax_cents, deposit_cents, total_cents, notes, created_by)
		VALUES ('', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`,
		req.CustomerID, StatusConfirmed, start, end, quote.SubtotalCents, quote.TaxCents,
		quote.DepositCents, quote.TotalCents, req.Notes, nullableID(req.CreatedBy)).
		Scan(&res.ID, &res.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating reservation: %w", err)
	}

	number := fmt.Sprintf("R-%s-%05d", start.Format("2006"), res.ID)
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET reservation_number = $2 WHERE id = $1`, res.ID, number); err != nil {
		return nil, fmt.Errorf("numbering reservation: %w", err)
	}

	invoiceLines := make([]billing.InvoiceLine, 0, len(quote.Lines))
	for _, line := range quote.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO reservation_items (reservation_id, model_id, quantity, rate_basis,
				rate_cents, billable_periods, line_total_cents)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			res.ID, line.ModelID, line.Quantity, line.RateBasis, line.RateCents,
			line.BillablePeriods, line.LineTotalCents); err != nil {
			return nil, fmt.Errorf("adding reservation item: %w", err)
		}
		invoiceLines = append(invoiceLines, billing.InvoiceLine{
			Kind: "rental",
			Description: fmt.Sprintf("%s — %s",
				line.ModelName, money.PeriodPhrase(line.RateBasis, line.BillablePeriods)),
			Quantity:        line.Quantity,
			UnitAmountCents: line.RateCents,
			AmountCents:     line.LineTotalCents,
		})
	}

	invoice, err := s.billing.CreateInvoice(ctx, tx, billing.NewInvoice{
		ReservationID: res.ID,
		CustomerID:    req.CustomerID,
		Status:        "issued",
		DueAt:         start,
		TaxCents:      quote.TaxCents,
		Lines:         invoiceLines,
	})
	if err != nil {
		return nil, err
	}

	if err := s.scheduleReminders(ctx, tx, res.ID, start, end); err != nil {
		return nil, err
	}
	if _, err := s.queue.Add(ctx, tx, jobs.Enqueue{
		Kind:      jobs.KindBookingConfirm,
		Payload:   map[string]any{"reservation_id": res.ID},
		DedupeKey: fmt.Sprintf("booking-confirm:%d", res.ID),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing booking: %w", err)
	}

	result := &BookResult{DepositHeld: 0}

	// Payment happens after the booking is durable. A declined card leaves a
	// real reservation with an unpaid invoice, which is exactly what a counter
	// would do — not a lost booking.
	if req.Card != nil {
		if quote.DepositCents > 0 {
			charge, err := s.billing.Gateway().Authorize(ctx, quote.DepositCents, *req.Card)
			if err != nil {
				result.PaymentError = err.Error()
				s.billing.RecordPayment(ctx, s.pool, billing.Payment{
					ReservationID: &res.ID, CustomerID: req.CustomerID, Kind: "authorization",
					AmountCents: quote.DepositCents, Status: "failed",
					Gateway: s.billing.Gateway().Name(), CardBrand: req.Card.Brand(),
					CardLast4: req.Card.Last4(), FailureReason: err.Error(),
				})
			} else {
				result.DepositHeld = quote.DepositCents
				s.billing.RecordPayment(ctx, s.pool, billing.Payment{
					ReservationID: &res.ID, CustomerID: req.CustomerID, Kind: "authorization",
					AmountCents: quote.DepositCents, Status: "succeeded",
					Gateway: s.billing.Gateway().Name(), GatewayRef: charge.Reference,
					CardBrand: charge.Brand, CardLast4: charge.Last4,
				})
			}
		}
		if result.PaymentError == "" && invoice.TotalCents > 0 {
			paid, err := s.billing.PayInvoice(ctx, invoice.ID, *req.Card)
			if err != nil {
				result.PaymentError = err.Error()
			} else {
				invoice = paid
			}
		}
	}

	full, err := s.Get(ctx, res.ID)
	if err != nil {
		return nil, err
	}
	result.Reservation = full
	result.Invoice = invoice
	return result, nil
}

// scheduleReminders queues the pickup and return nudges. Both are deduplicated
// on the reservation, so rescheduling a booking simply moves them.
func (s *Service) scheduleReminders(ctx context.Context, q db.Querier, resID int64, start, end time.Time) error {
	pickupAt := start.Add(-s.pickupReminderLead)
	returnAt := end.Add(-s.returnReminderLead)

	if _, err := s.queue.Add(ctx, q, jobs.Enqueue{
		Kind:      jobs.KindPickupReminder,
		Payload:   map[string]any{"reservation_id": resID},
		RunAt:     pickupAt,
		DedupeKey: fmt.Sprintf("pickup-reminder:%d", resID),
	}); err != nil {
		return err
	}
	_, err := s.queue.Add(ctx, q, jobs.Enqueue{
		Kind:      jobs.KindReturnReminder,
		Payload:   map[string]any{"reservation_id": resID},
		RunAt:     returnAt,
		DedupeKey: fmt.Sprintf("return-reminder:%d", resID),
	})
	return err
}

// ---------- counter: handing the machine over ----------

// Checkout assigns physical units to a reservation and marks it picked up.
func (s *Service) Checkout(ctx context.Context, resID int64, req CheckoutRequest) (*Reservation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`, resID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != StatusConfirmed {
		return nil, fmt.Errorf("%w: it is already %s", ErrBadTransition, status)
	}

	// Every line must be fully assigned; a partial handover is not a pickup.
	wanted := map[int64]int{}
	itemModel := map[int64]int64{}
	rows, err := tx.Query(ctx,
		`SELECT id, quantity, model_id FROM reservation_items WHERE reservation_id = $1`, resID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, modelID int64
		var qty int
		if err := rows.Scan(&id, &qty, &modelID); err != nil {
			rows.Close()
			return nil, err
		}
		wanted[id] = qty
		itemModel[id] = modelID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	given := map[int64]int{}
	for _, line := range req.Lines {
		if _, ok := wanted[line.ItemID]; !ok {
			return nil, httpx.BadRequest(fmt.Sprintf("Line %d is not part of this reservation.", line.ItemID))
		}
		for _, unitID := range line.UnitIDs {
			// Confirm the unit is the right model and genuinely on the yard,
			// locking it so two counters cannot hand out the same machine.
			var unitModel int64
			var unitStatus string
			err := tx.QueryRow(ctx, `
				SELECT u.model_id, u.status FROM equipment_units u
				WHERE u.id = $1 FOR UPDATE`, unitID).Scan(&unitModel, &unitStatus)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, httpx.BadRequest(fmt.Sprintf("Unit %d does not exist.", unitID))
			}
			if err != nil {
				return nil, err
			}

			if unitModel != itemModel[line.ItemID] {
				return nil, httpx.BadRequest("That unit is a different model from the one booked.")
			}
			if unitStatus != "available" {
				return nil, httpx.Conflict(fmt.Sprintf("Unit %d is %s and cannot go out.", unitID, unitStatus))
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO unit_assignments (reservation_item_id, unit_id, checkout_notes, meter_out)
				VALUES ($1,$2,$3,$4)`, line.ItemID, unitID, line.Notes, line.MeterOut); err != nil {
				return nil, fmt.Errorf("assigning unit %d: %w", unitID, err)
			}
			if _, err := tx.Exec(ctx,
				`UPDATE equipment_units SET status='out', updated_at=now() WHERE id=$1`, unitID); err != nil {
				return nil, err
			}
			given[line.ItemID]++
		}
	}

	for itemID, qty := range wanted {
		if given[itemID] != qty {
			return nil, httpx.BadRequest(fmt.Sprintf(
				"Assign exactly %d unit(s) to every line before checking out (line %d has %d).",
				qty, itemID, given[itemID]))
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reservations
		SET status=$2, picked_up_at=now(),
		    notes = CASE WHEN $3 = '' THEN notes
		                 ELSE trim(both E'\n' from notes || E'\n' || $3) END,
		    updated_at=now()
		WHERE id=$1`, resID, StatusPickedUp, req.Notes); err != nil {
		return nil, err
	}

	// The pickup reminder is moot once the machine is in the customer's hands.
	if _, err := tx.Exec(ctx,
		`DELETE FROM jobs WHERE status='pending' AND dedupe_key = $1`,
		fmt.Sprintf("pickup-reminder:%d", resID)); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, resID)
}

// ---------- counter: taking the machine back ----------

// Checkin closes out a rental: units return to the yard, late fees and damage
// are billed, and whatever is left of the deposit hold is released.
func (s *Service) Checkin(ctx context.Context, resID int64, req CheckinRequest) (*CheckinResult, error) {
	res, err := s.Get(ctx, resID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNotFound
	}
	if res.Status != StatusPickedUp {
		return nil, fmt.Errorf("%w: it is %s, not out on rental", ErrBadTransition, res.Status)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var damageTotal money.Cents
	for _, line := range req.Lines {
		if line.DamageCents < 0 {
			return nil, httpx.BadRequest("Damage charges cannot be negative.")
		}
		unitStatus := "available"
		if line.NeedsMaintenance {
			unitStatus = "maintenance"
		}
		tag, err := tx.Exec(ctx, `
			UPDATE unit_assignments
			SET checked_in_at = now(), checkin_notes = $2, damage_cents = $3, meter_in = $4
			WHERE id = $1 AND checked_in_at IS NULL`,
			line.AssignmentID, line.Notes, line.DamageCents, line.MeterIn)
		if err != nil {
			return nil, fmt.Errorf("checking in assignment %d: %w", line.AssignmentID, err)
		}
		if tag.RowsAffected() == 0 {
			return nil, httpx.BadRequest(fmt.Sprintf(
				"Item %d is not currently checked out.", line.AssignmentID))
		}
		if _, err := tx.Exec(ctx, `
			UPDATE equipment_units u
			SET status = $2, meter_hours = COALESCE($3, u.meter_hours),
			    condition_notes = CASE WHEN $4 = '' THEN u.condition_notes ELSE $4 END,
			    updated_at = now()
			FROM unit_assignments ua
			WHERE ua.id = $1 AND u.id = ua.unit_id`,
			line.AssignmentID, unitStatus, line.MeterIn, line.Notes); err != nil {
			return nil, err
		}
		damageTotal += line.DamageCents
	}

	// Anything still out means this was a partial return, which the desk must
	// handle as a separate visit rather than closing the rental.
	var stillOut int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM unit_assignments ua
		JOIN reservation_items ri ON ri.id = ua.reservation_item_id
		WHERE ri.reservation_id = $1 AND ua.checked_in_at IS NULL`, resID).Scan(&stillOut); err != nil {
		return nil, err
	}
	if stillOut > 0 {
		return nil, httpx.BadRequest(fmt.Sprintf(
			"%d item(s) are still out; check every item in to close the rental.", stillOut))
	}

	daysOverdue := overdueDays(res.EndDate.Time(), time.Now().UTC())
	var lateFee money.Cents
	for _, item := range res.Items {
		lateFee += money.LateFee(item.DailyRateCents, daysOverdue, s.lateFeeMultiple) *
			money.Cents(item.Quantity)
	}

	result := &CheckinResult{
		DaysOverdue: daysOverdue, LateFeeCents: lateFee, DamageCents: damageTotal,
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reservations
		SET status=$2, returned_at=now(),
		    notes = CASE WHEN $3 = '' THEN notes
		                 ELSE trim(both E'\n' from notes || E'\n' || $3) END,
		    updated_at=now()
		WHERE id=$1`, resID, StatusReturned, req.Notes); err != nil {
		return nil, err
	}

	// Extras get their own invoice so the original rental invoice stays a clean
	// record of what was agreed at booking.
	var extraInvoice *billing.Invoice
	if lateFee > 0 || damageTotal > 0 {
		lines := []billing.InvoiceLine{}
		if lateFee > 0 {
			lines = append(lines, billing.InvoiceLine{
				Kind: "late_fee",
				Description: fmt.Sprintf("Late return — %d day%s past %s",
					daysOverdue, plural(daysOverdue), res.EndDate.Time().Format("2 Jan 2006")),
				Quantity: daysOverdue, AmountCents: lateFee,
				UnitAmountCents: lateFee / money.Cents(max(daysOverdue, 1)),
			})
		}
		if damageTotal > 0 {
			lines = append(lines, billing.InvoiceLine{
				Kind: "damage", Description: "Damage and cleaning charges",
				Quantity: 1, UnitAmountCents: damageTotal, AmountCents: damageTotal,
			})
		}
		extraInvoice, err = s.billing.CreateInvoice(ctx, tx, billing.NewInvoice{
			ReservationID: resID, CustomerID: res.CustomerID, Status: "issued",
			DueAt:    time.Now().AddDate(0, 0, 14),
			TaxCents: money.Tax(lateFee, s.taxRatePercent), // damage recovery is not taxed
			Lines:    lines,
		})
		if err != nil {
			return nil, err
		}
		result.ExtraInvoiceID = &extraInvoice.ID
	}

	if _, err := s.queue.Add(ctx, tx, jobs.Enqueue{
		Kind:      jobs.KindReceipt,
		Payload:   map[string]any{"reservation_id": resID},
		DedupeKey: fmt.Sprintf("receipt:%d", resID),
	}); err != nil {
		return nil, err
	}
	if err := s.queue.CancelFor(ctx, tx, resID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Settle against the deposit hold outside the transaction, since it talks to
	// the payment gateway.
	taken, released := s.settleDeposit(ctx, res, extraInvoice)
	result.DepositTakenCents = taken
	result.DepositReleasedCents = released

	full, err := s.Get(ctx, resID)
	if err != nil {
		return nil, err
	}
	result.Reservation = full
	return result, nil
}

// settleDeposit takes what is owed out of the hold placed at booking and lets
// the rest go. Failures here are reported but never undo a completed return.
func (s *Service) settleDeposit(ctx context.Context, res *Reservation, extra *billing.Invoice) (taken, released money.Cents) {
	payments, err := s.billing.ListPayments(ctx, billing.PaymentFilter{ReservationID: res.ID})
	if err != nil {
		return 0, 0
	}
	var holdRef string
	var holdAmount money.Cents
	for _, p := range payments {
		if p.Kind == "authorization" && p.Status == "succeeded" {
			holdRef, holdAmount = p.GatewayRef, p.AmountCents
			break
		}
	}
	if holdRef == "" {
		return 0, 0
	}

	owed := money.Cents(0)
	if extra != nil {
		owed = extra.TotalCents
	}
	take := min(owed, holdAmount)

	if take > 0 {
		if charge, err := s.billing.Gateway().Capture(ctx, holdRef, take); err == nil {
			s.billing.RecordPayment(ctx, s.pool, billing.Payment{
				InvoiceID: &extra.ID, ReservationID: &res.ID, CustomerID: res.CustomerID,
				Kind: "capture", AmountCents: take, Status: "succeeded",
				Gateway: s.billing.Gateway().Name(), GatewayRef: charge.Reference,
				CardBrand: charge.Brand, CardLast4: charge.Last4,
			})
			s.pool.Exec(ctx, `
				UPDATE invoices
				SET amount_paid_cents = amount_paid_cents + $2,
				    status = CASE WHEN amount_paid_cents + $2 >= total_cents THEN 'paid' ELSE status END,
				    updated_at = now()
				WHERE id = $1`, extra.ID, take)
			taken = take
		}
	}

	if err := s.billing.Gateway().Release(ctx, holdRef); err == nil {
		released = holdAmount - taken
		s.billing.RecordPayment(ctx, s.pool, billing.Payment{
			ReservationID: &res.ID, CustomerID: res.CustomerID, Kind: "release",
			AmountCents: released, Status: "succeeded",
			Gateway: s.billing.Gateway().Name(), GatewayRef: holdRef,
		})
	}
	return taken, released
}

// Cancel voids a booking that has not been collected.
func (s *Service) Cancel(ctx context.Context, resID int64, reason string) (*Reservation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id=$1 FOR UPDATE`, resID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != StatusConfirmed {
		return nil, fmt.Errorf("%w: a %s reservation cannot be cancelled", ErrBadTransition, status)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reservations SET status=$2,
		    notes = CASE WHEN $3 = '' THEN notes
		                 ELSE trim(both E'\n' from notes || E'\nCancelled: ' || $3) END,
		    updated_at=now()
		WHERE id=$1`, resID, StatusCancelled, reason); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE invoices SET status='void', updated_at=now()
		 WHERE reservation_id=$1 AND status IN ('draft','issued')`, resID); err != nil {
		return nil, err
	}
	if err := s.queue.CancelFor(ctx, tx, resID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Let any deposit hold go now that the rental will not happen.
	if payments, err := s.billing.ListPayments(ctx, billing.PaymentFilter{ReservationID: resID}); err == nil {
		for _, p := range payments {
			if p.Kind == "authorization" && p.Status == "succeeded" && p.GatewayRef != "" {
				s.billing.Gateway().Release(ctx, p.GatewayRef)
				break
			}
		}
	}
	return s.Get(ctx, resID)
}

// ---------- helpers ----------

func validateWindow(startD, endD httpx.Date) (time.Time, time.Time, error) {
	if startD.IsZero() || endD.IsZero() {
		return time.Time{}, time.Time{}, httpx.BadRequest("Choose both a start and an end date.")
	}
	start, end := truncate(startD.Time()), truncate(endD.Time())
	if end.Before(start) {
		return time.Time{}, time.Time{}, httpx.Invalid(map[string]string{
			"end_date": "The return date cannot be before the pickup date.",
		})
	}
	if start.Before(truncate(time.Now().UTC())) {
		return time.Time{}, time.Time{}, httpx.Invalid(map[string]string{
			"start_date": "Pickup cannot be scheduled in the past.",
		})
	}
	if end.Sub(start) > 365*24*time.Hour {
		return time.Time{}, time.Time{}, httpx.Invalid(map[string]string{
			"end_date": "Rentals longer than a year need to be arranged with the office.",
		})
	}
	return start, end, nil
}

func truncate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// overdueDays counts whole days a machine is late; returning early is zero.
func overdueDays(due, now time.Time) int {
	d := int(truncate(now).Sub(truncate(due)).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func sortedModelIDs(items []QuoteItem) []int64 {
	ids := make([]int64, 0, len(items))
	seen := map[int64]bool{}
	for _, it := range items {
		if !seen[it.ModelID] {
			seen[it.ModelID] = true
			ids = append(ids, it.ModelID)
		}
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func nullableID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
