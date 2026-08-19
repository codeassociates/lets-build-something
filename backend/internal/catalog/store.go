// Package catalog holds the rentable inventory: categories, the equipment
// models a customer picks from, the physical units on the yard, and the
// availability calculation that decides whether a booking can be taken.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

type Category struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	ModelCount  int    `json:"model_count"`
}

// Model is a rentable product line — "14 lb electric jackhammer". Customers
// reserve a model; a specific unit is handed over at the counter.
type Model struct {
	ID                    int64             `json:"id"`
	CategoryID            int64             `json:"category_id"`
	CategoryName          string            `json:"category_name"`
	CategorySlug          string            `json:"category_slug"`
	SKU                   string            `json:"sku"`
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	Manufacturer          string            `json:"manufacturer"`
	DailyRateCents        money.Cents       `json:"daily_rate_cents"`
	WeeklyRateCents       money.Cents       `json:"weekly_rate_cents"`
	MonthlyRateCents      money.Cents       `json:"monthly_rate_cents"`
	DepositCents          money.Cents       `json:"deposit_cents"`
	ReplacementValueCents money.Cents       `json:"replacement_value_cents"`
	RequiresLicense       bool              `json:"requires_license"`
	Specs                 map[string]string `json:"specs"`
	ImageURL              string            `json:"image_url"`
	Active                bool              `json:"active"`

	// Fleet counts, populated by List and Get.
	TotalUnits int `json:"total_units"`
	// AvailableUnits is meaningful only when a date window was supplied.
	AvailableUnits int `json:"available_units"`
}

func (m Model) RateCard() money.RateCard {
	return money.RateCard{
		Daily:   m.DailyRateCents,
		Weekly:  m.WeeklyRateCents,
		Monthly: m.MonthlyRateCents,
	}
}

type Unit struct {
	ID             int64      `json:"id"`
	ModelID        int64      `json:"model_id"`
	ModelName      string     `json:"model_name"`
	SKU            string     `json:"sku"`
	AssetTag       string     `json:"asset_tag"`
	SerialNumber   string     `json:"serial_number"`
	Status         string     `json:"status"`
	ConditionNotes string     `json:"condition_notes"`
	MeterHours     float64    `json:"meter_hours"`
	AcquiredOn     *time.Time `json:"acquired_on"`

	// Set when the unit is out: which reservation is holding it.
	ReservationID     *int64  `json:"reservation_id,omitempty"`
	ReservationNumber *string `json:"reservation_number,omitempty"`
}

type Store struct{ pool *db.DB }

func NewStore(pool *db.DB) *Store { return &Store{pool: pool} }

// ---------- categories ----------

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.slug, c.name, c.description, c.sort_order,
		       COUNT(m.id) FILTER (WHERE m.active) AS model_count
		FROM categories c
		LEFT JOIN equipment_models m ON m.category_id = c.id
		GROUP BY c.id
		ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()

	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.SortOrder, &c.ModelCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCategory(ctx context.Context, c Category) (*Category, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO categories (slug, name, description, sort_order)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		c.Slug, c.Name, c.Description, c.SortOrder).Scan(&c.ID)
	if err != nil {
		return nil, fmt.Errorf("creating category: %w", err)
	}
	return &c, nil
}

func (s *Store) UpdateCategory(ctx context.Context, id int64, c Category) (*Category, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE categories SET slug=$2, name=$3, description=$4, sort_order=$5
		WHERE id=$1 RETURNING id`, id, c.Slug, c.Name, c.Description, c.SortOrder).Scan(&c.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("updating category: %w", err)
	}
	return &c, nil
}

// ---------- models ----------

type ModelFilter struct {
	CategorySlug string
	Search       string
	IncludeIdle  bool // admin view: include deactivated models
	Start, End   time.Time
	Limit        int
	Offset       int
}

const modelColumns = `m.id, m.category_id, c.name, c.slug, m.sku, m.name, m.description,
	m.manufacturer, m.daily_rate_cents, m.weekly_rate_cents, m.monthly_rate_cents,
	m.deposit_cents, m.replacement_value_cents, m.requires_license, m.specs,
	m.image_url, m.active`

func scanModel(row pgx.Row, extra ...any) (*Model, error) {
	var m Model
	dest := []any{&m.ID, &m.CategoryID, &m.CategoryName, &m.CategorySlug, &m.SKU, &m.Name,
		&m.Description, &m.Manufacturer, &m.DailyRateCents, &m.WeeklyRateCents,
		&m.MonthlyRateCents, &m.DepositCents, &m.ReplacementValueCents,
		&m.RequiresLicense, &m.Specs, &m.ImageURL, &m.Active}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	if m.Specs == nil {
		m.Specs = map[string]string{}
	}
	return &m, nil
}

// availabilityCTE computes, per model, how many units are rentable and the peak
// number already committed on any single day of the requested window. Peak
// concurrent demand is the correct measure: two back-to-back bookings in the
// same week do not each consume a unit for the whole week.
const availabilityCTE = `
WITH rentable AS (
	SELECT model_id, COUNT(*) AS units
	FROM equipment_units
	WHERE status IN ('available','out')
	GROUP BY model_id
),
daily_demand AS (
	SELECT ri.model_id, d.day, SUM(ri.quantity) AS qty
	FROM generate_series($1::date, $2::date, '1 day') AS d(day)
	JOIN reservations r
	  ON r.start_date <= d.day AND r.end_date >= d.day
	 AND r.status IN ('confirmed','picked_up')
	 AND ($3::bigint IS NULL OR r.id <> $3::bigint)
	JOIN reservation_items ri ON ri.reservation_id = r.id
	GROUP BY ri.model_id, d.day
),
peak AS (
	SELECT model_id, MAX(qty)::int AS committed
	FROM daily_demand GROUP BY model_id
)`

// List returns models, with availability counts when a date window is given.
func (s *Store) List(ctx context.Context, f ModelFilter) ([]Model, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 60
	}
	// With no window, ask about today so the counts still mean something.
	start, end := f.Start, f.End
	if start.IsZero() || end.IsZero() {
		start = time.Now().UTC()
		end = start
	}
	search := ""
	if f.Search != "" {
		search = "%" + strings.ToLower(f.Search) + "%"
	}

	rows, err := s.pool.Query(ctx, availabilityCTE+`
		SELECT `+modelColumns+`,
		       COALESCE(rentable.units, 0)::int AS total_units,
		       GREATEST(COALESCE(rentable.units,0) - COALESCE(peak.committed,0), 0)::int AS available_units,
		       COUNT(*) OVER () AS total
		FROM equipment_models m
		JOIN categories c ON c.id = m.category_id
		LEFT JOIN rentable ON rentable.model_id = m.id
		LEFT JOIN peak ON peak.model_id = m.id
		WHERE ($4 OR m.active)
		  AND ($5 = '' OR c.slug = $5)
		  AND ($6 = '' OR lower(m.name) LIKE $6 OR lower(m.sku) LIKE $6
		       OR lower(m.description) LIKE $6 OR lower(m.manufacturer) LIKE $6)
		ORDER BY c.sort_order, m.name
		LIMIT $7 OFFSET $8`,
		start, end, nil, f.IncludeIdle, f.CategorySlug, search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing models: %w", err)
	}
	defer rows.Close()

	out := []Model{}
	total := 0
	for rows.Next() {
		var totalUnits, availableUnits int
		m, err := scanModel(rows, &totalUnits, &availableUnits, &total)
		if err != nil {
			return nil, 0, err
		}
		m.TotalUnits = totalUnits
		m.AvailableUnits = availableUnits
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *Store) Get(ctx context.Context, id int64, start, end time.Time) (*Model, error) {
	if start.IsZero() || end.IsZero() {
		start = time.Now().UTC()
		end = start
	}
	var totalUnits, availableUnits int
	m, err := scanModel(s.pool.QueryRow(ctx, availabilityCTE+`
		SELECT `+modelColumns+`,
		       COALESCE(rentable.units, 0)::int,
		       GREATEST(COALESCE(rentable.units,0) - COALESCE(peak.committed,0), 0)::int
		FROM equipment_models m
		JOIN categories c ON c.id = m.category_id
		LEFT JOIN rentable ON rentable.model_id = m.id
		LEFT JOIN peak ON peak.model_id = m.id
		WHERE m.id = $4`, start, end, nil, id), &totalUnits, &availableUnits)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading model %d: %w", id, err)
	}
	m.TotalUnits = totalUnits
	m.AvailableUnits = availableUnits
	return m, nil
}

// Available reports how many of a model can still be booked across a window,
// optionally ignoring one reservation so an existing booking can be edited
// without competing with itself.
func (s *Store) Available(ctx context.Context, modelID int64, start, end time.Time, excludeReservation *int64) (int, error) {
	return s.AvailableIn(ctx, s.pool, modelID, start, end, excludeReservation)
}

// AvailableIn is Available against a caller's transaction, so a booking can
// check availability and insert under the same lock.
func (s *Store) AvailableIn(ctx context.Context, q db.Querier, modelID int64, start, end time.Time, excludeReservation *int64) (int, error) {
	var available int
	err := q.QueryRow(ctx, availabilityCTE+`
		SELECT GREATEST(COALESCE(rentable.units,0) - COALESCE(peak.committed,0), 0)::int
		FROM equipment_models m
		LEFT JOIN rentable ON rentable.model_id = m.id
		LEFT JOIN peak ON peak.model_id = m.id
		WHERE m.id = $4`, start, end, excludeReservation, modelID).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("model %d does not exist", modelID)
	}
	if err != nil {
		return 0, fmt.Errorf("checking availability: %w", err)
	}
	return available, nil
}

type ModelInput struct {
	CategoryID            int64             `json:"category_id"`
	SKU                   string            `json:"sku"`
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	Manufacturer          string            `json:"manufacturer"`
	DailyRateCents        money.Cents       `json:"daily_rate_cents"`
	WeeklyRateCents       money.Cents       `json:"weekly_rate_cents"`
	MonthlyRateCents      money.Cents       `json:"monthly_rate_cents"`
	DepositCents          money.Cents       `json:"deposit_cents"`
	ReplacementValueCents money.Cents       `json:"replacement_value_cents"`
	RequiresLicense       bool              `json:"requires_license"`
	Specs                 map[string]string `json:"specs"`
	ImageURL              string            `json:"image_url"`
	Active                bool              `json:"active"`
}

func (s *Store) CreateModel(ctx context.Context, in ModelInput) (int64, error) {
	if in.Specs == nil {
		in.Specs = map[string]string{}
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO equipment_models (category_id, sku, name, description, manufacturer,
			daily_rate_cents, weekly_rate_cents, monthly_rate_cents, deposit_cents,
			replacement_value_cents, requires_license, specs, image_url, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
		in.CategoryID, in.SKU, in.Name, in.Description, in.Manufacturer,
		in.DailyRateCents, in.WeeklyRateCents, in.MonthlyRateCents, in.DepositCents,
		in.ReplacementValueCents, in.RequiresLicense, in.Specs, in.ImageURL, in.Active).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating model: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateModel(ctx context.Context, id int64, in ModelInput) error {
	if in.Specs == nil {
		in.Specs = map[string]string{}
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE equipment_models SET category_id=$2, sku=$3, name=$4, description=$5,
			manufacturer=$6, daily_rate_cents=$7, weekly_rate_cents=$8,
			monthly_rate_cents=$9, deposit_cents=$10, replacement_value_cents=$11,
			requires_license=$12, specs=$13, image_url=$14, active=$15, updated_at=now()
		WHERE id=$1`,
		id, in.CategoryID, in.SKU, in.Name, in.Description, in.Manufacturer,
		in.DailyRateCents, in.WeeklyRateCents, in.MonthlyRateCents, in.DepositCents,
		in.ReplacementValueCents, in.RequiresLicense, in.Specs, in.ImageURL, in.Active)
	if err != nil {
		return fmt.Errorf("updating model: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---------- units ----------

type UnitFilter struct {
	ModelID int64
	Status  string
	Search  string
	Limit   int
	Offset  int
}

func (s *Store) ListUnits(ctx context.Context, f UnitFilter) ([]Unit, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	search := ""
	if f.Search != "" {
		search = "%" + strings.ToLower(f.Search) + "%"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.model_id, m.name, m.sku, u.asset_tag, u.serial_number, u.status,
		       u.condition_notes, u.meter_hours, u.acquired_on,
		       r.id, r.reservation_number,
		       COUNT(*) OVER () AS total
		FROM equipment_units u
		JOIN equipment_models m ON m.id = u.model_id
		LEFT JOIN unit_assignments ua ON ua.unit_id = u.id AND ua.checked_in_at IS NULL
		LEFT JOIN reservation_items ri ON ri.id = ua.reservation_item_id
		LEFT JOIN reservations r ON r.id = ri.reservation_id
		WHERE ($1 = 0 OR u.model_id = $1)
		  AND ($2 = '' OR u.status = $2)
		  AND ($3 = '' OR lower(u.asset_tag) LIKE $3 OR lower(u.serial_number) LIKE $3
		       OR lower(m.name) LIKE $3)
		ORDER BY m.name, u.asset_tag
		LIMIT $4 OFFSET $5`,
		f.ModelID, f.Status, search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing units: %w", err)
	}
	defer rows.Close()

	out := []Unit{}
	total := 0
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.ModelID, &u.ModelName, &u.SKU, &u.AssetTag,
			&u.SerialNumber, &u.Status, &u.ConditionNotes, &u.MeterHours, &u.AcquiredOn,
			&u.ReservationID, &u.ReservationNumber, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// FreeUnits lists units of a model that can be handed over right now: on the
// yard, not already out, not in maintenance.
func (s *Store) FreeUnits(ctx context.Context, modelID int64) ([]Unit, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.model_id, m.name, m.sku, u.asset_tag, u.serial_number, u.status,
		       u.condition_notes, u.meter_hours, u.acquired_on
		FROM equipment_units u
		JOIN equipment_models m ON m.id = u.model_id
		WHERE u.model_id = $1 AND u.status = 'available'
		  AND NOT EXISTS (
		      SELECT 1 FROM unit_assignments ua
		      WHERE ua.unit_id = u.id AND ua.checked_in_at IS NULL)
		ORDER BY u.asset_tag`, modelID)
	if err != nil {
		return nil, fmt.Errorf("listing free units: %w", err)
	}
	defer rows.Close()

	out := []Unit{}
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.ModelID, &u.ModelName, &u.SKU, &u.AssetTag,
			&u.SerialNumber, &u.Status, &u.ConditionNotes, &u.MeterHours, &u.AcquiredOn); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type UnitInput struct {
	ModelID        int64      `json:"model_id"`
	AssetTag       string     `json:"asset_tag"`
	SerialNumber   string     `json:"serial_number"`
	Status         string     `json:"status"`
	ConditionNotes string     `json:"condition_notes"`
	MeterHours     float64    `json:"meter_hours"`
	AcquiredOn     *time.Time `json:"acquired_on"`
}

func (s *Store) CreateUnit(ctx context.Context, in UnitInput) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO equipment_units (model_id, asset_tag, serial_number, status,
			condition_notes, meter_hours, acquired_on)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.ModelID, in.AssetTag, in.SerialNumber, in.Status, in.ConditionNotes,
		in.MeterHours, in.AcquiredOn).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating unit: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateUnit(ctx context.Context, id int64, in UnitInput) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE equipment_units SET model_id=$2, asset_tag=$3, serial_number=$4,
			status=$5, condition_notes=$6, meter_hours=$7, acquired_on=$8, updated_at=now()
		WHERE id=$1`,
		id, in.ModelID, in.AssetTag, in.SerialNumber, in.Status, in.ConditionNotes,
		in.MeterHours, in.AcquiredOn)
	if err != nil {
		return fmt.Errorf("updating unit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
