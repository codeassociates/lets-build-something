// Package billing turns finished rentals into invoices and moves money.
package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/money"
)

// Card is the payment instrument as the API receives it. Only the brand and the
// last four digits are ever stored.
type Card struct {
	Number      string `json:"number"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
	CVC         string `json:"cvc"`
	Name        string `json:"name"`
}

func (c Card) Last4() string {
	digits := onlyDigits(c.Number)
	if len(digits) < 4 {
		return ""
	}
	return digits[len(digits)-4:]
}

// Brand is inferred from the leading digits, the way a terminal does it.
func (c Card) Brand() string {
	d := onlyDigits(c.Number)
	switch {
	case strings.HasPrefix(d, "4"):
		return "Visa"
	case strings.HasPrefix(d, "5"):
		return "Mastercard"
	case strings.HasPrefix(d, "34"), strings.HasPrefix(d, "37"):
		return "American Express"
	case strings.HasPrefix(d, "6"):
		return "Discover"
	default:
		return "Card"
	}
}

type Charge struct {
	Reference string
	Amount    money.Cents
	Brand     string
	Last4     string
}

var (
	ErrDeclined      = errors.New("the card was declined")
	ErrUnknownCharge = errors.New("no such charge")
)

// Gateway is the whole surface the rest of the system uses to move money.
// A real Stripe adapter implements these four methods and nothing else changes.
type Gateway interface {
	// Authorize places a hold — the deposit at booking time.
	Authorize(ctx context.Context, amount money.Cents, card Card) (*Charge, error)
	// Capture takes money from an existing hold, up to the amount held.
	Capture(ctx context.Context, reference string, amount money.Cents) (*Charge, error)
	// Release lets a hold expire without taking anything.
	Release(ctx context.Context, reference string) error
	// Refund returns money already captured.
	Refund(ctx context.Context, reference string, amount money.Cents) (*Charge, error)
	Name() string
}

// FakeGateway is a working stand-in that behaves like a real processor,
// including declines, so the checkout and refund paths are exercised for real
// during development. Test numbers:
//
//	4242 4242 4242 4242  succeeds
//	4000 0000 0000 0002  always declined
//	4000 0000 0000 0069  expired card
//
// Any other well-formed number succeeds.
type FakeGateway struct {
	mu      sync.Mutex
	charges map[string]*fakeCharge
	// AdoptUnknown makes the gateway accept a reference it has no record of,
	// trusting the amount the caller asks for. Holds placed before this process
	// started — seeded demo data, or a restart mid-rental — would otherwise be
	// unsettleable, since this stand-in keeps its state in memory. A real
	// processor remembers its own charges and needs nothing of the sort.
	AdoptUnknown bool
}

type fakeCharge struct {
	amount    money.Cents
	captured  money.Cents
	refunded  money.Cents
	released  bool
	brand     string
	last4     string
	createdAt time.Time
}

func NewFakeGateway() *FakeGateway {
	return &FakeGateway{charges: map[string]*fakeCharge{}, AdoptUnknown: true}
}

// lookup finds a charge, adopting an unrecognised but plausible reference when
// the gateway is configured to. Callers must hold the mutex.
func (g *FakeGateway) lookup(reference string, assumeAmount money.Cents) (*fakeCharge, bool) {
	if c, ok := g.charges[reference]; ok {
		return c, true
	}
	if !g.AdoptUnknown || !strings.HasPrefix(reference, "auth_") {
		return nil, false
	}
	c := &fakeCharge{amount: assumeAmount, brand: "Card", last4: "0000", createdAt: time.Now()}
	g.charges[reference] = c
	return c, true
}

func (g *FakeGateway) Name() string { return "fake" }

func (g *FakeGateway) Authorize(ctx context.Context, amount money.Cents, card Card) (*Charge, error) {
	if amount < 0 {
		return nil, fmt.Errorf("cannot authorize a negative amount")
	}
	digits := onlyDigits(card.Number)
	switch {
	case len(digits) < 12:
		return nil, fmt.Errorf("%w: the card number looks incomplete", ErrDeclined)
	case strings.HasSuffix(digits, "0002"):
		return nil, fmt.Errorf("%w: insufficient funds", ErrDeclined)
	case strings.HasSuffix(digits, "0069"):
		return nil, fmt.Errorf("%w: the card has expired", ErrDeclined)
	case card.ExpiryYear > 0 && expired(card):
		return nil, fmt.Errorf("%w: the card has expired", ErrDeclined)
	}

	ref := "auth_" + randomHex(12)
	g.mu.Lock()
	g.charges[ref] = &fakeCharge{
		amount: amount, brand: card.Brand(), last4: card.Last4(), createdAt: time.Now(),
	}
	g.mu.Unlock()

	return &Charge{Reference: ref, Amount: amount, Brand: card.Brand(), Last4: card.Last4()}, nil
}

func (g *FakeGateway) Capture(ctx context.Context, reference string, amount money.Cents) (*Charge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.lookup(reference, amount)
	if !ok {
		return nil, ErrUnknownCharge
	}
	if c.released {
		return nil, fmt.Errorf("%w: the hold was already released", ErrDeclined)
	}
	if c.captured+amount > c.amount {
		return nil, fmt.Errorf("%w: only %s remains on this hold",
			ErrDeclined, (c.amount - c.captured))
	}
	c.captured += amount
	return &Charge{Reference: reference, Amount: amount, Brand: c.brand, Last4: c.last4}, nil
}

func (g *FakeGateway) Release(ctx context.Context, reference string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.lookup(reference, 0)
	if !ok {
		return ErrUnknownCharge
	}
	c.released = true
	return nil
}

func (g *FakeGateway) Refund(ctx context.Context, reference string, amount money.Cents) (*Charge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.lookup(reference, 0)
	if !ok {
		return nil, ErrUnknownCharge
	}
	if c.refunded+amount > c.captured {
		return nil, fmt.Errorf("cannot refund %s; only %s was captured",
			amount, c.captured-c.refunded)
	}
	c.refunded += amount
	return &Charge{Reference: reference, Amount: amount, Brand: c.brand, Last4: c.last4}, nil
}

func expired(c Card) bool {
	if c.ExpiryMonth < 1 || c.ExpiryMonth > 12 {
		return true
	}
	// A card is good through the last day of its expiry month.
	end := time.Date(c.ExpiryYear, time.Month(c.ExpiryMonth), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, 1, 0)
	return time.Now().After(end)
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
