package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

// plan describes a batch of reservations to generate in one lifecycle state.
// Building the yard from a plan rather than ad-hoc loops makes it obvious what
// the demo data actually contains.
type plan struct {
	label     string
	count     int
	status    string
	startFrom int // days from today, inclusive
	startTo   int
	minDays   int
	maxDays   int
	// overdueBy, when set, forces the return date this many days into the past.
	overdueBy   int
	startsToday bool
	endsToday   bool
}

func (s *Seeder) seedReservations(ctx context.Context, q db.Querier) error {
	plans := []plan{
		{label: "completed rentals", count: 22, status: "returned",
			startFrom: -90, startTo: -12, minDays: 1, maxDays: 21},
		{label: "cancelled bookings", count: 5, status: "cancelled",
			startFrom: -30, startTo: 20, minDays: 1, maxDays: 7},
		{label: "out on hire", count: 9, status: "picked_up",
			startFrom: -9, startTo: -1, minDays: 5, maxDays: 24},
		{label: "overdue", count: 5, status: "picked_up",
			startFrom: -26, startTo: -12, minDays: 3, maxDays: 8, overdueBy: 1},
		{label: "returns due today", count: 4, status: "picked_up",
			startFrom: -12, startTo: -2, minDays: 1, maxDays: 12, endsToday: true},
		{label: "pickups due today", count: 5, status: "confirmed",
			startsToday: true, minDays: 1, maxDays: 10},
		{label: "upcoming bookings", count: 19, status: "confirmed",
			startFrom: 1, startTo: 40, minDays: 1, maxDays: 16},
	}

	// A unit can only be out once at a time, so open assignments must not reuse
	// one. Returned rentals release their units and are excluded.
	unitsInUse := map[int64]bool{}

	for _, p := range plans {
		for i := 0; i < p.count; i++ {
			if err := s.createReservation(ctx, q, p, unitsInUse); err != nil {
				return fmt.Errorf("%s: %w", p.label, err)
			}
		}
	}
	return nil
}

func (s *Seeder) createReservation(ctx context.Context, q db.Querier, p plan, unitsInUse map[int64]bool) error {
	start, end := s.windowFor(p)
	customerID := s.pickCustomer()
	days := money.RentalDays(start, end)

	// Most rentals are a single machine; a good few are a small kit.
	itemCount := 1
	switch r := s.rng.Intn(10); {
	case r >= 8:
		itemCount = 3
	case r >= 5:
		itemCount = 2
	}
	// An open rental holds its units until it is returned, so it may only use a
	// model that still has one free. A finished rental has released them again.
	var keep func(int64) bool
	if p.status == "picked_up" {
		keep = func(modelID int64) bool { return s.freeUnitCount(modelID, unitsInUse) > 0 }
	}
	modelIDs := s.pickModels(itemCount, keep)
	if len(modelIDs) == 0 {
		// The fleet is fully committed for this state; that is a legitimate
		// outcome, not a failure to seed.
		return nil
	}

	type lineData struct {
		modelID  int64
		quantity int
		basis    string
		rate     money.Cents
		periods  int
		total    money.Cents
		deposit  money.Cents
	}
	lines := make([]lineData, 0, len(modelIDs))
	var subtotal, deposit money.Cents

	for _, modelID := range modelIDs {
		m := s.modelByID[modelID]
		quantity := 1
		// Consumable-ish items (air movers, hoses) go out in multiples.
		if m.Units >= 10 && s.rng.Intn(2) == 0 {
			quantity = 2 + s.rng.Intn(2)
		}
		if p.status == "picked_up" {
			if free := s.freeUnitCount(modelID, unitsInUse); quantity > free {
				quantity = free
			}
		}
		basis, rate, periods, lineTotal := s.quoteFor(modelID, days)

		lines = append(lines, lineData{
			modelID: modelID, quantity: quantity, basis: basis, rate: rate,
			periods: periods, total: lineTotal * money.Cents(quantity),
			deposit: money.Cents(m.Deposit) * money.Cents(quantity),
		})
		subtotal += lineTotal * money.Cents(quantity)
		deposit += money.Cents(m.Deposit) * money.Cents(quantity)
	}

	tax := money.Tax(subtotal, s.taxRatePercent)
	total := subtotal + tax

	createdAt := start.AddDate(0, 0, -s.rng.Intn(9)-1)
	if createdAt.After(s.today) {
		createdAt = s.today
	}

	var pickedUpAt, returnedAt *time.Time
	switch p.status {
	case "picked_up":
		t := start.Add(time.Duration(7+s.rng.Intn(9)) * time.Hour)
		pickedUpAt = &t
	case "returned":
		t := start.Add(time.Duration(7+s.rng.Intn(9)) * time.Hour)
		pickedUpAt = &t
		// Most come back on time; a minority run late.
		back := end.Add(time.Duration(8+s.rng.Intn(9)) * time.Hour)
		if s.rng.Intn(6) == 0 {
			back = back.AddDate(0, 0, 1+s.rng.Intn(4))
		}
		returnedAt = &back
	}

	var resID int64
	err := q.QueryRow(ctx, `
		INSERT INTO reservations (reservation_number, customer_id, status, start_date,
			end_date, picked_up_at, returned_at, subtotal_cents, tax_cents, deposit_cents,
			total_cents, notes, created_by, created_at, updated_at)
		VALUES ('', $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13) RETURNING id`,
		customerID, p.status, start, end, pickedUpAt, returnedAt,
		subtotal, tax, deposit, total, s.pickNote(), s.pickStaffOrNil(), createdAt).Scan(&resID)
	if err != nil {
		return err
	}
	number := fmt.Sprintf("R-%s-%05d", start.Format("2006"), resID)
	if _, err := q.Exec(ctx,
		`UPDATE reservations SET reservation_number=$2 WHERE id=$1`, resID, number); err != nil {
		return err
	}

	var invoiceLines []invoiceLine
	for _, l := range lines {
		m := s.modelByID[l.modelID]
		var itemID int64
		if err := q.QueryRow(ctx, `
			INSERT INTO reservation_items (reservation_id, model_id, quantity, rate_basis,
				rate_cents, billable_periods, line_total_cents)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			resID, l.modelID, l.quantity, l.basis, l.rate, l.periods, l.total).Scan(&itemID); err != nil {
			return err
		}
		invoiceLines = append(invoiceLines, invoiceLine{
			kind: "rental",
			description: fmt.Sprintf("%s — %s", m.Name,
				money.PeriodPhrase(l.basis, l.periods)),
			quantity: l.quantity, unitAmount: l.rate, amount: l.total,
		})

		// Hand over physical units for anything collected.
		if p.status == "picked_up" || p.status == "returned" {
			if err := s.assignUnits(ctx, q, itemID, l.modelID, l.quantity,
				p.status, pickedUpAt, returnedAt, unitsInUse); err != nil {
				return err
			}
		}
	}

	if p.status == "cancelled" {
		if _, err := s.createInvoice(ctx, q, resID, customerID, "void", start, tax, invoiceLines); err != nil {
			return err
		}
		s.logEmail(ctx, q, customerID, resID, "booking_confirmation",
			fmt.Sprintf("Booking confirmed — %s", number), createdAt)
		return nil
	}

	invoiceID, err := s.createInvoice(ctx, q, resID, customerID, "issued", start, tax, invoiceLines)
	if err != nil {
		return err
	}

	// Payment behaviour: most customers pay at booking, a few are on account.
	paid := s.rng.Intn(10) < 8
	if paid {
		if err := s.recordPayment(ctx, q, payment{
			invoiceID: &invoiceID, reservationID: resID, customerID: customerID,
			kind: "capture", amount: total, at: createdAt,
		}); err != nil {
			return err
		}
		if _, err := q.Exec(ctx,
			`UPDATE invoices SET amount_paid_cents=$2, status='paid' WHERE id=$1`,
			invoiceID, total); err != nil {
			return err
		}
	}
	if deposit > 0 {
		if err := s.recordPayment(ctx, q, payment{
			reservationID: resID, customerID: customerID, kind: "authorization",
			amount: deposit, at: createdAt,
		}); err != nil {
			return err
		}
	}

	if err := s.seedEmails(ctx, q, resID, customerID, number, p, start, end, createdAt); err != nil {
		return err
	}

	// A completed rental settles up: late fees where it came back late, and the
	// deposit released.
	if p.status == "returned" && returnedAt != nil {
		lateDays := int(returnedAt.Sub(end).Hours() / 24)
		if lateDays > 0 {
			var lateFee money.Cents
			for _, l := range lines {
				m := s.modelByID[l.modelID]
				lateFee += money.LateFee(money.Cents(m.Daily), lateDays, 1.5) *
					money.Cents(l.quantity)
			}
			extraID, err := s.createInvoice(ctx, q, resID, customerID, "issued",
				*returnedAt, money.Tax(lateFee, s.taxRatePercent),
				[]invoiceLine{{
					kind: "late_fee",
					description: fmt.Sprintf("Late return — %d day%s past %s",
						lateDays, pluralSuffix(lateDays), end.Format("2 Jan 2006")),
					quantity: lateDays, unitAmount: lateFee / money.Cents(lateDays),
					amount: lateFee,
				}})
			if err != nil {
				return err
			}
			// Taken out of the deposit hold, as the system does at check-in.
			settled := lateFee + money.Tax(lateFee, s.taxRatePercent)
			if settled > deposit {
				settled = deposit
			}
			if settled > 0 {
				if err := s.recordPayment(ctx, q, payment{
					invoiceID: &extraID, reservationID: resID, customerID: customerID,
					kind: "capture", amount: settled, at: *returnedAt,
				}); err != nil {
					return err
				}
				if _, err := q.Exec(ctx, `
					UPDATE invoices SET amount_paid_cents=$2,
						status = CASE WHEN $2 >= total_cents THEN 'paid' ELSE status END
					WHERE id=$1`, extraID, settled); err != nil {
					return err
				}
			}
		}
		if deposit > 0 {
			if err := s.recordPayment(ctx, q, payment{
				reservationID: resID, customerID: customerID, kind: "release",
				amount: deposit, at: *returnedAt,
			}); err != nil {
				return err
			}
		}
		s.logEmail(ctx, q, customerID, resID, "rental_receipt",
			fmt.Sprintf("Thanks — %s returned", number), *returnedAt)
	}

	return nil
}

// windowFor turns a plan into concrete dates.
func (s *Seeder) windowFor(p plan) (time.Time, time.Time) {
	length := p.minDays + s.rng.Intn(max(p.maxDays-p.minDays, 1))

	switch {
	case p.startsToday:
		return s.today, s.today.AddDate(0, 0, length-1)
	case p.endsToday:
		return s.today.AddDate(0, 0, -(length - 1)), s.today
	case p.overdueBy > 0:
		// Due back somewhere in the recent past, still not returned.
		end := s.today.AddDate(0, 0, -(1 + s.rng.Intn(9)))
		return end.AddDate(0, 0, -(length - 1)), end
	default:
		offset := p.startFrom + s.rng.Intn(max(p.startTo-p.startFrom, 1))
		start := s.today.AddDate(0, 0, offset)
		end := start.AddDate(0, 0, length-1)
		// An out-on-hire rental must still be running.
		if p.status == "picked_up" && !end.After(s.today) {
			end = s.today.AddDate(0, 0, 1+s.rng.Intn(12))
		}
		return start, end
	}
}

// assignUnits hands physical machines to a line, marking them out where the
// rental is still running.
func (s *Seeder) assignUnits(ctx context.Context, q db.Querier, itemID, modelID int64,
	quantity int, status string, out, in *time.Time, unitsInUse map[int64]bool) error {

	pool := s.unitsByModel[modelID]
	assigned := 0

	for _, unitID := range pool {
		if assigned == quantity {
			break
		}
		// Only an open assignment blocks a unit; returned rentals free it again.
		if status == "picked_up" && unitsInUse[unitID] {
			continue
		}
		var checkedIn any
		damage := money.Cents(0)
		if status == "returned" {
			checkedIn = in
			if s.rng.Intn(12) == 0 {
				damage = money.Cents((s.rng.Intn(18) + 3) * 500)
			}
		}

		meterOut := float64(s.rng.Intn(1800)) + 0.5
		var meterIn any
		if status == "returned" {
			meterIn = meterOut + float64(s.rng.Intn(40)) + 0.5
		}

		if _, err := q.Exec(ctx, `
			INSERT INTO unit_assignments (reservation_item_id, unit_id, checked_out_at,
				checked_in_at, checkout_notes, checkin_notes, damage_cents, meter_out, meter_in)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			itemID, unitID, out, checkedIn, "", checkinNote(damage, s.rng), damage,
			meterOut, meterIn); err != nil {
			return err
		}

		if status == "picked_up" {
			unitsInUse[unitID] = true
			if _, err := q.Exec(ctx,
				`UPDATE equipment_units SET status='out' WHERE id=$1`, unitID); err != nil {
				return err
			}
		}
		assigned++
	}

	if assigned < quantity {
		// The fleet could not cover this line. Shrink the booking rather than
		// leaving a reservation the yard cannot honour.
		if assigned == 0 {
			return fmt.Errorf("no free units for model %d", modelID)
		}
		if _, err := q.Exec(ctx,
			`UPDATE reservation_items SET quantity=$2 WHERE id=$1`, itemID, assigned); err != nil {
			return err
		}
	}
	return nil
}

func checkinNote(damage money.Cents, rng interface{ Intn(int) int }) string {
	if damage > 0 {
		return []string{
			"Returned with a cracked housing — charged for repair.",
			"Missing chisel from the kit; charged at replacement cost.",
			"Heavy concrete residue, extra cleaning required.",
			"Damaged hose end, replaced.",
		}[rng.Intn(4)]
	}
	if rng.Intn(4) == 0 {
		return "Returned clean and running well."
	}
	return ""
}

func (s *Seeder) pickNote() string {
	return jobNotes[s.rng.Intn(len(jobNotes))]
}

func (s *Seeder) pickStaffOrNil() any {
	// Most bookings come in through the website; some are taken at the counter.
	if s.rng.Intn(3) == 0 {
		return s.pickStaff()
	}
	return nil
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// freeUnitCount reports how many units of a model are not already out on
// another open rental.
func (s *Seeder) freeUnitCount(modelID int64, unitsInUse map[int64]bool) int {
	free := 0
	for _, unitID := range s.unitsByModel[modelID] {
		if !unitsInUse[unitID] {
			free++
		}
	}
	return free
}
