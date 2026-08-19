// Package rental is the heart of the system: quoting a job, taking the booking,
// handing the machine over at the counter, and checking it back in.
package rental

import (
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

// Reservation lifecycle. A booking is confirmed the moment it is taken —
// there is no pending state, because availability is checked before it is
// written, so a reservation that exists is one the yard can honour.
const (
	StatusConfirmed = "confirmed"
	StatusPickedUp  = "picked_up"
	StatusReturned  = "returned"
	StatusCancelled = "cancelled"
)

type Reservation struct {
	ID                int64       `json:"id"`
	ReservationNumber string      `json:"reservation_number"`
	CustomerID        int64       `json:"customer_id"`
	CustomerName      string      `json:"customer_name"`
	CustomerEmail     string      `json:"customer_email"`
	CustomerPhone     string      `json:"customer_phone"`
	Status            string      `json:"status"`
	StartDate         httpx.Date  `json:"start_date"`
	EndDate           httpx.Date  `json:"end_date"`
	PickedUpAt        *time.Time  `json:"picked_up_at"`
	ReturnedAt        *time.Time  `json:"returned_at"`
	SubtotalCents     money.Cents `json:"subtotal_cents"`
	TaxCents          money.Cents `json:"tax_cents"`
	DepositCents      money.Cents `json:"deposit_cents"`
	TotalCents        money.Cents `json:"total_cents"`
	Notes             string      `json:"notes"`
	CreatedAt         time.Time   `json:"created_at"`

	Items []Item `json:"items"`

	// Derived for the UI, so the desk does not recompute overdue in three places.
	RentalDays  int  `json:"rental_days"`
	IsOverdue   bool `json:"is_overdue"`
	DaysOverdue int  `json:"days_overdue"`
}

type Item struct {
	ID              int64       `json:"id"`
	ReservationID   int64       `json:"reservation_id"`
	ModelID         int64       `json:"model_id"`
	ModelName       string      `json:"model_name"`
	SKU             string      `json:"sku"`
	ImageURL        string      `json:"image_url"`
	Quantity        int         `json:"quantity"`
	RateBasis       string      `json:"rate_basis"`
	RateCents       money.Cents `json:"rate_cents"`
	BillablePeriods int         `json:"billable_periods"`
	LineTotalCents  money.Cents `json:"line_total_cents"`
	DailyRateCents  money.Cents `json:"daily_rate_cents"`

	Assignments []Assignment `json:"assignments"`
}

// Assignment records which physical machine went out against a line.
type Assignment struct {
	ID                int64       `json:"id"`
	ReservationItemID int64       `json:"reservation_item_id"`
	UnitID            int64       `json:"unit_id"`
	AssetTag          string      `json:"asset_tag"`
	SerialNumber      string      `json:"serial_number"`
	CheckedOutAt      time.Time   `json:"checked_out_at"`
	CheckedInAt       *time.Time  `json:"checked_in_at"`
	CheckoutNotes     string      `json:"checkout_notes"`
	CheckinNotes      string      `json:"checkin_notes"`
	DamageCents       money.Cents `json:"damage_cents"`
	MeterOut          *float64    `json:"meter_out"`
	MeterIn           *float64    `json:"meter_in"`
}

// ---------- quoting ----------

type QuoteItem struct {
	ModelID  int64 `json:"model_id"`
	Quantity int   `json:"quantity"`
}

type QuoteRequest struct {
	StartDate httpx.Date  `json:"start_date"`
	EndDate   httpx.Date  `json:"end_date"`
	Items     []QuoteItem `json:"items"`
}

type QuoteLine struct {
	ModelID         int64       `json:"model_id"`
	ModelName       string      `json:"model_name"`
	SKU             string      `json:"sku"`
	ImageURL        string      `json:"image_url"`
	Quantity        int         `json:"quantity"`
	RateBasis       string      `json:"rate_basis"`
	RateCents       money.Cents `json:"rate_cents"`
	BillablePeriods int         `json:"billable_periods"`
	LineTotalCents  money.Cents `json:"line_total_cents"`
	DepositCents    money.Cents `json:"deposit_cents"`
	RequiresLicense bool        `json:"requires_license"`

	AvailableUnits int  `json:"available_units"`
	Available      bool `json:"available"`
}

type Quote struct {
	StartDate     httpx.Date  `json:"start_date"`
	EndDate       httpx.Date  `json:"end_date"`
	RentalDays    int         `json:"rental_days"`
	Lines         []QuoteLine `json:"lines"`
	SubtotalCents money.Cents `json:"subtotal_cents"`
	TaxCents      money.Cents `json:"tax_cents"`
	TotalCents    money.Cents `json:"total_cents"`
	DepositCents  money.Cents `json:"deposit_cents"`
	// DueNowCents is what the card is charged today: the rental plus the
	// refundable deposit hold.
	DueNowCents  money.Cents `json:"due_now_cents"`
	AllAvailable bool        `json:"all_available"`
}

// ---------- counter operations ----------

type CheckoutLine struct {
	ItemID   int64    `json:"item_id"`
	UnitIDs  []int64  `json:"unit_ids"`
	Notes    string   `json:"notes"`
	MeterOut *float64 `json:"meter_out"`
}

type CheckoutRequest struct {
	Lines []CheckoutLine `json:"lines"`
	Notes string         `json:"notes"`
}

type CheckinLine struct {
	AssignmentID     int64       `json:"assignment_id"`
	MeterIn          *float64    `json:"meter_in"`
	Notes            string      `json:"notes"`
	DamageCents      money.Cents `json:"damage_cents"`
	NeedsMaintenance bool        `json:"needs_maintenance"`
}

type CheckinRequest struct {
	Lines []CheckinLine `json:"lines"`
	Notes string        `json:"notes"`
}

// CheckinResult tells the desk what to say to the customer: what extra was
// owed, and what happened to the deposit.
type CheckinResult struct {
	Reservation          *Reservation `json:"reservation"`
	DaysOverdue          int          `json:"days_overdue"`
	LateFeeCents         money.Cents  `json:"late_fee_cents"`
	DamageCents          money.Cents  `json:"damage_cents"`
	ExtraInvoiceID       *int64       `json:"extra_invoice_id"`
	DepositTakenCents    money.Cents  `json:"deposit_taken_cents"`
	DepositReleasedCents money.Cents  `json:"deposit_released_cents"`
}

// DeskSummary is the counter's working view of the day.
type DeskSummary struct {
	Date            httpx.Date    `json:"date"`
	PickupsDue      []Reservation `json:"pickups_due"`
	ReturnsDue      []Reservation `json:"returns_due"`
	Overdue         []Reservation `json:"overdue"`
	OutNow          int           `json:"out_now"`
	PickupsDueCount int           `json:"pickups_due_count"`
	ReturnsDueCount int           `json:"returns_due_count"`
	OverdueCount    int           `json:"overdue_count"`
}
