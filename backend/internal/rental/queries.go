package rental

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
)

const reservationColumns = `r.id, r.reservation_number, r.customer_id, u.full_name, u.email,
	u.phone, r.status, r.start_date, r.end_date, r.picked_up_at, r.returned_at,
	r.subtotal_cents, r.tax_cents, r.deposit_cents, r.total_cents, r.notes, r.created_at`

func scanReservation(row pgx.Row, extra ...any) (*Reservation, error) {
	var r Reservation
	var start, end time.Time
	dest := []any{&r.ID, &r.ReservationNumber, &r.CustomerID, &r.CustomerName, &r.CustomerEmail,
		&r.CustomerPhone, &r.Status, &start, &end, &r.PickedUpAt, &r.ReturnedAt,
		&r.SubtotalCents, &r.TaxCents, &r.DepositCents, &r.TotalCents, &r.Notes, &r.CreatedAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	r.StartDate, r.EndDate = httpx.Date(start), httpx.Date(end)
	r.Items = []Item{}
	r.decorate()
	return &r, nil
}

// decorate fills in the values the UI would otherwise have to derive itself.
func (r *Reservation) decorate() {
	r.RentalDays = int(r.EndDate.Time().Sub(r.StartDate.Time()).Hours()/24) + 1
	if r.RentalDays < 1 {
		r.RentalDays = 1
	}
	if r.Status == StatusPickedUp {
		r.DaysOverdue = overdueDays(r.EndDate.Time(), time.Now().UTC())
		r.IsOverdue = r.DaysOverdue > 0
	}
}

func (s *Service) Get(ctx context.Context, id int64) (*Reservation, error) {
	res, err := scanReservation(s.pool.QueryRow(ctx, `
		SELECT `+reservationColumns+`
		FROM reservations r JOIN users u ON u.id = r.customer_id
		WHERE r.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading reservation %d: %w", id, err)
	}
	if err := s.attachItems(ctx, []*Reservation{res}); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) ByNumber(ctx context.Context, number string) (*Reservation, error) {
	res, err := scanReservation(s.pool.QueryRow(ctx, `
		SELECT `+reservationColumns+`
		FROM reservations r JOIN users u ON u.id = r.customer_id
		WHERE r.reservation_number = $1`, strings.ToUpper(strings.TrimSpace(number))))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.attachItems(ctx, []*Reservation{res}); err != nil {
		return nil, err
	}
	return res, nil
}

// attachItems loads lines and unit assignments for a page of reservations in
// two queries rather than two per reservation.
func (s *Service) attachItems(ctx context.Context, list []*Reservation) error {
	if len(list) == 0 {
		return nil
	}
	byID := make(map[int64]*Reservation, len(list))
	ids := make([]int64, 0, len(list))
	for _, r := range list {
		byID[r.ID] = r
		ids = append(ids, r.ID)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT ri.id, ri.reservation_id, ri.model_id, m.name, m.sku, m.image_url,
		       ri.quantity, ri.rate_basis, ri.rate_cents, ri.billable_periods,
		       ri.line_total_cents, m.daily_rate_cents
		FROM reservation_items ri
		JOIN equipment_models m ON m.id = ri.model_id
		WHERE ri.reservation_id = ANY($1)
		ORDER BY ri.id`, ids)
	if err != nil {
		return fmt.Errorf("loading reservation items: %w", err)
	}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ReservationID, &it.ModelID, &it.ModelName, &it.SKU,
			&it.ImageURL, &it.Quantity, &it.RateBasis, &it.RateCents, &it.BillablePeriods,
			&it.LineTotalCents, &it.DailyRateCents); err != nil {
			rows.Close()
			return err
		}
		it.Assignments = []Assignment{}
		res := byID[it.ReservationID]
		res.Items = append(res.Items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Index the items only once every append is done. Taking pointers while the
	// slice is still growing would leave them dangling in a reallocated array.
	itemsByID := map[int64]*Item{}
	for _, res := range list {
		for i := range res.Items {
			itemsByID[res.Items[i].ID] = &res.Items[i]
		}
	}
	if len(itemsByID) == 0 {
		return nil
	}

	assignRows, err := s.pool.Query(ctx, `
		SELECT ua.id, ua.reservation_item_id, ua.unit_id, eu.asset_tag, eu.serial_number,
		       ua.checked_out_at, ua.checked_in_at, ua.checkout_notes, ua.checkin_notes,
		       ua.damage_cents, ua.meter_out, ua.meter_in
		FROM unit_assignments ua
		JOIN equipment_units eu ON eu.id = ua.unit_id
		JOIN reservation_items ri ON ri.id = ua.reservation_item_id
		WHERE ri.reservation_id = ANY($1)
		ORDER BY ua.id`, ids)
	if err != nil {
		return fmt.Errorf("loading unit assignments: %w", err)
	}
	defer assignRows.Close()

	for assignRows.Next() {
		var a Assignment
		if err := assignRows.Scan(&a.ID, &a.ReservationItemID, &a.UnitID, &a.AssetTag,
			&a.SerialNumber, &a.CheckedOutAt, &a.CheckedInAt, &a.CheckoutNotes,
			&a.CheckinNotes, &a.DamageCents, &a.MeterOut, &a.MeterIn); err != nil {
			return err
		}
		if item, ok := itemsByID[a.ReservationItemID]; ok {
			item.Assignments = append(item.Assignments, a)
		}
	}
	return assignRows.Err()
}

type Filter struct {
	CustomerID  int64
	Status      string
	Search      string
	StartsOn    *time.Time
	EndsOn      *time.Time
	OverdueOnly bool
	ActiveOnly  bool // confirmed or picked up: the bookings that still matter
	Limit       int
	Offset      int
}

func (s *Service) List(ctx context.Context, f Filter) ([]Reservation, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	search := ""
	if f.Search != "" {
		search = "%" + strings.ToLower(f.Search) + "%"
	}

	rows, err := s.pool.Query(ctx, `
		SELECT `+reservationColumns+`, COUNT(*) OVER () AS total
		FROM reservations r
		JOIN users u ON u.id = r.customer_id
		WHERE ($1 = 0 OR r.customer_id = $1)
		  AND ($2 = '' OR r.status = $2)
		  AND ($3 = '' OR lower(r.reservation_number) LIKE $3
		       OR lower(u.full_name) LIKE $3 OR lower(u.email) LIKE $3
		       OR lower(u.company) LIKE $3)
		  AND ($4::date IS NULL OR r.start_date = $4::date)
		  AND ($5::date IS NULL OR r.end_date = $5::date)
		  AND (NOT $6 OR (r.status = 'picked_up' AND r.end_date < CURRENT_DATE))
		  AND (NOT $7 OR r.status IN ('confirmed','picked_up'))
		ORDER BY r.start_date DESC, r.id DESC
		LIMIT $8 OFFSET $9`,
		f.CustomerID, f.Status, search, f.StartsOn, f.EndsOn, f.OverdueOnly,
		f.ActiveOnly, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing reservations: %w", err)
	}
	defer rows.Close()

	out := []Reservation{}
	refs := []*Reservation{}
	total := 0
	for rows.Next() {
		res, err := scanReservation(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *res)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for i := range out {
		refs = append(refs, &out[i])
	}
	if err := s.attachItems(ctx, refs); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Desk builds the counter's view of a day: who is collecting, who is bringing
// something back, and who is late.
func (s *Service) Desk(ctx context.Context, day time.Time) (*DeskSummary, error) {
	day = truncate(day)
	summary := &DeskSummary{Date: httpx.Date(day)}

	load := func(f Filter) ([]Reservation, int, error) { return s.List(ctx, f) }

	pickups, pickupCount, err := load(Filter{Status: StatusConfirmed, StartsOn: &day, Limit: 100})
	if err != nil {
		return nil, err
	}
	returns, returnCount, err := load(Filter{Status: StatusPickedUp, EndsOn: &day, Limit: 100})
	if err != nil {
		return nil, err
	}
	overdue, overdueCount, err := load(Filter{OverdueOnly: true, Limit: 100})
	if err != nil {
		return nil, err
	}

	summary.PickupsDue, summary.PickupsDueCount = pickups, pickupCount
	summary.ReturnsDue, summary.ReturnsDueCount = returns, returnCount
	summary.Overdue, summary.OverdueCount = overdue, overdueCount

	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM unit_assignments WHERE checked_in_at IS NULL`).
		Scan(&summary.OutNow); err != nil {
		return nil, err
	}
	return summary, nil
}

// Stats backs the admin dashboard.
type Stats struct {
	ActiveRentals    int   `json:"active_rentals"`
	UpcomingPickups  int   `json:"upcoming_pickups"`
	OverdueRentals   int   `json:"overdue_rentals"`
	UnitsOut         int   `json:"units_out"`
	UnitsAvailable   int   `json:"units_available"`
	UnitsMaintenance int   `json:"units_maintenance"`
	Customers        int   `json:"customers"`
	RevenueMTDCents  int64 `json:"revenue_mtd_cents"`
	OutstandingCents int64 `json:"outstanding_cents"`
	ReservationsMTD  int   `json:"reservations_mtd"`
}

func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	var st Stats
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM reservations WHERE status = 'picked_up'),
			(SELECT COUNT(*) FROM reservations WHERE status = 'confirmed' AND start_date >= CURRENT_DATE),
			(SELECT COUNT(*) FROM reservations WHERE status = 'picked_up' AND end_date < CURRENT_DATE),
			(SELECT COUNT(*) FROM equipment_units WHERE status = 'out'),
			(SELECT COUNT(*) FROM equipment_units WHERE status = 'available'),
			(SELECT COUNT(*) FROM equipment_units WHERE status = 'maintenance'),
			(SELECT COUNT(*) FROM users WHERE role = 'customer' AND active),
			(SELECT COALESCE(SUM(amount_cents), 0) FROM payments
			  WHERE kind = 'capture' AND status = 'succeeded'
			    AND created_at >= date_trunc('month', CURRENT_DATE)),
			(SELECT COALESCE(SUM(total_cents - amount_paid_cents), 0) FROM invoices
			  WHERE status IN ('issued','draft')),
			(SELECT COUNT(*) FROM reservations WHERE created_at >= date_trunc('month', CURRENT_DATE))`).
		Scan(&st.ActiveRentals, &st.UpcomingPickups, &st.OverdueRentals, &st.UnitsOut,
			&st.UnitsAvailable, &st.UnitsMaintenance, &st.Customers, &st.RevenueMTDCents,
			&st.OutstandingCents, &st.ReservationsMTD)
	if err != nil {
		return nil, fmt.Errorf("computing stats: %w", err)
	}
	return &st, nil
}

// DueForOverdueNotice finds rentals that passed their return date, for the
// daily sweep to chase.
func (s *Service) DueForOverdueNotice(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM reservations
		WHERE status = 'picked_up' AND end_date < CURRENT_DATE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
