// Package seed fills an empty database with a believable rental yard: a
// catalog, a fleet, customers, and rentals spread across every state the
// system can be in. It exists so the interfaces can be looked at and used
// long before there is any real data.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

// Seed is deterministic: the same seed value always produces the same yard, so
// a screenshot taken today still matches the data tomorrow.
type Seeder struct {
	pool           *db.DB
	rng            *rand.Rand
	taxRatePercent float64
	today          time.Time

	categoryIDs  map[string]int64
	modelIDs     map[string]int64
	modelByID    map[int64]modelSeed
	unitsByModel map[int64][]int64
	customerIDs  []int64
	staffIDs     []int64
}

type Options struct {
	// Reset wipes existing data first. Without it, seeding refuses to run on a
	// database that already has content, so it can never quietly destroy work.
	Reset          bool
	RandomSeed     int64
	TaxRatePercent float64
}

func New(pool *db.DB, o Options) *Seeder {
	if o.RandomSeed == 0 {
		o.RandomSeed = 20260419
	}
	if o.TaxRatePercent == 0 {
		o.TaxRatePercent = 8.5
	}
	now := time.Now().UTC()
	return &Seeder{
		pool:           pool,
		rng:            rand.New(rand.NewSource(o.RandomSeed)),
		taxRatePercent: o.TaxRatePercent,
		today:          time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		categoryIDs:    map[string]int64{},
		modelIDs:       map[string]int64{},
		modelByID:      map[int64]modelSeed{},
		unitsByModel:   map[int64][]int64{},
	}
}

// Run populates the database. Everything happens in one transaction, so a
// failure half way through leaves nothing behind.
func (s *Seeder) Run(ctx context.Context, o Options) error {
	existing, err := s.countUsers(ctx)
	if err != nil {
		return err
	}
	if existing > 0 && !o.Reset {
		slog.Info("database already has data; skipping seed", "users", existing)
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if o.Reset {
		if _, err := tx.Exec(ctx, `
			TRUNCATE emails, jobs, payments, invoice_lines, invoices, unit_assignments,
			         reservation_items, reservations, equipment_units, equipment_models,
			         categories, sessions, users RESTART IDENTITY CASCADE`); err != nil {
			return fmt.Errorf("clearing existing data: %w", err)
		}
		slog.Info("cleared existing data")
	}

	steps := []struct {
		name string
		fn   func(context.Context, db.Querier) error
	}{
		{"categories", s.seedCategories},
		{"equipment models", s.seedModels},
		{"fleet units", s.seedUnits},
		{"people", s.seedPeople},
		{"rentals", s.seedReservations},
	}
	for _, step := range steps {
		if err := step.fn(ctx, tx); err != nil {
			return fmt.Errorf("seeding %s: %w", step.name, err)
		}
		slog.Info("seeded", "step", step.name)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing seed data: %w", err)
	}
	slog.Info("seed complete",
		"models", len(s.modelIDs), "customers", len(s.customerIDs))
	return nil
}

func (s *Seeder) countUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Seeder) seedCategories(ctx context.Context, q db.Querier) error {
	for _, c := range categories {
		var id int64
		if err := q.QueryRow(ctx, `
			INSERT INTO categories (slug, name, description, sort_order)
			VALUES ($1,$2,$3,$4) RETURNING id`,
			c.Slug, c.Name, c.Description, c.Sort).Scan(&id); err != nil {
			return err
		}
		s.categoryIDs[c.Slug] = id
	}
	return nil
}

func (s *Seeder) seedModels(ctx context.Context, q db.Querier) error {
	for _, m := range models {
		categoryID, ok := s.categoryIDs[m.Category]
		if !ok {
			return fmt.Errorf("model %s references unknown category %q", m.SKU, m.Category)
		}
		var id int64
		if err := q.QueryRow(ctx, `
			INSERT INTO equipment_models (category_id, sku, name, description, manufacturer,
				daily_rate_cents, weekly_rate_cents, monthly_rate_cents, deposit_cents,
				replacement_value_cents, requires_license, specs, active)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,true) RETURNING id`,
			categoryID, m.SKU, m.Name, m.Description, m.Manufacturer,
			m.Daily, m.Weekly, m.Monthly, m.Deposit, m.Replacement,
			m.RequiresLicense, m.Specs).Scan(&id); err != nil {
			return err
		}
		s.modelIDs[m.SKU] = id
		s.modelByID[id] = m
	}
	return nil
}

// seedUnits builds the physical fleet. A few units are parked in maintenance
// or retired, because a yard where everything is available looks fake and
// never exercises the availability logic.
func (s *Seeder) seedUnits(ctx context.Context, q db.Querier) error {
	for _, m := range models {
		modelID := s.modelIDs[m.SKU]
		for i := 1; i <= m.Units; i++ {
			status := "available"
			switch {
			case i == m.Units && m.Units >= 5 && s.rng.Intn(3) == 0:
				status = "maintenance"
			case i == m.Units && m.Units >= 8 && s.rng.Intn(4) == 0:
				status = "retired"
			}

			acquired := s.today.AddDate(0, -s.rng.Intn(48)-1, -s.rng.Intn(28))
			meter := float64(s.rng.Intn(2400)) + float64(s.rng.Intn(10))/10

			var unitID int64
			if err := q.QueryRow(ctx, `
				INSERT INTO equipment_units (model_id, asset_tag, serial_number, status,
					condition_notes, meter_hours, acquired_on)
				VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
				modelID,
				fmt.Sprintf("%s-%02d", m.SKU, i),
				fmt.Sprintf("%s%07d", serialPrefix(m.Manufacturer), s.rng.Intn(9999999)),
				status, conditionNote(status, s.rng), meter, acquired).Scan(&unitID); err != nil {
				return err
			}
			if status == "available" {
				s.unitsByModel[modelID] = append(s.unitsByModel[modelID], unitID)
			}
		}
	}
	return nil
}

func serialPrefix(manufacturer string) string {
	if len(manufacturer) >= 2 {
		return string([]byte{manufacturer[0], manufacturer[1]})
	}
	return "XX"
}

func conditionNote(status string, rng *rand.Rand) string {
	switch status {
	case "maintenance":
		return []string{
			"In for service — carburettor rebuild.",
			"Awaiting parts: pull cord assembly.",
			"Annual service and safety inspection.",
			"Hydraulic hose replacement booked.",
		}[rng.Intn(4)]
	case "retired":
		return "Withdrawn from hire — beyond economic repair."
	default:
		if rng.Intn(6) == 0 {
			return []string{
				"Cosmetic scuffing to housing; fully serviceable.",
				"New blade fitted last service.",
				"Runs well. Slight oil weep, monitored.",
			}[rng.Intn(3)]
		}
		return ""
	}
}

func (s *Seeder) seedPeople(ctx context.Context, q db.Querier) error {
	// One known admin, so there is always a way in to a fresh deployment.
	adminID, err := s.insertPerson(ctx, q, personSeed{
		Name: "Alex Rutherford", Email: "admin@kestrelrental.example",
		Phone: "(406) 555-0100", Company: "Kestrel Equipment Rental",
		Address: "1420 Gallatin Road", City: "Bozeman", State: "MT", Postal: "59718",
	}, auth.RoleAdmin, DemoPassword)
	if err != nil {
		return err
	}
	s.staffIDs = append(s.staffIDs, adminID)

	for _, p := range staffMembers {
		id, err := s.insertPerson(ctx, q, p, auth.RoleStaff, DemoPassword)
		if err != nil {
			return err
		}
		s.staffIDs = append(s.staffIDs, id)
	}

	for _, p := range customers {
		id, err := s.insertPerson(ctx, q, p, auth.RoleCustomer, DemoPassword)
		if err != nil {
			return err
		}
		s.customerIDs = append(s.customerIDs, id)
	}
	return nil
}

// DemoPassword is shared by every seeded account. This is demonstration data
// in a development database; it is never a path into a real deployment,
// because a real deployment is not seeded.
const DemoPassword = "rentals123"

func (s *Seeder) insertPerson(ctx context.Context, q db.Querier, p personSeed, role auth.Role, password string) (int64, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, full_name, phone, company,
			address_line1, city, state, postal_code, license_number, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		p.Email, hash, role, p.Name, p.Phone, p.Company,
		p.Address, p.City, p.State, p.Postal, p.License,
		s.today.AddDate(0, 0, -s.rng.Intn(600)-30)).Scan(&id)
	return id, err
}

func (s *Seeder) pickCustomer() int64 {
	return s.customerIDs[s.rng.Intn(len(s.customerIDs))]
}

func (s *Seeder) pickStaff() int64 {
	return s.staffIDs[s.rng.Intn(len(s.staffIDs))]
}

// pickModels chooses n distinct models at random. keep filters to those the
// caller can actually fulfil — an open rental needs a unit nobody else holds.
func (s *Seeder) pickModels(n int, keep func(int64) bool) []int64 {
	ids := make([]int64, 0, len(s.modelIDs))
	for _, id := range s.modelIDs {
		if len(s.unitsByModel[id]) == 0 {
			continue
		}
		if keep != nil && !keep(id) {
			continue
		}
		ids = append(ids, id)
	}
	// Sorting keeps the shuffle reproducible despite map iteration order.
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	s.rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if n > len(ids) {
		n = len(ids)
	}
	return ids[:n]
}

func (s *Seeder) quoteFor(modelID int64, days int) (basis string, rate money.Cents, periods int, total money.Cents) {
	m := s.modelByID[modelID]
	q := money.BestQuote(money.RateCard{
		Daily: money.Cents(m.Daily), Weekly: money.Cents(m.Weekly), Monthly: money.Cents(m.Monthly),
	}, days)
	return q.Basis, q.Rate, q.Periods, q.Subtotal
}
