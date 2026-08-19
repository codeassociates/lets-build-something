package billing

import (
	"context"
	"errors"
	"testing"
)

func goodCard() Card {
	return Card{Number: "4242 4242 4242 4242", ExpiryMonth: 12, ExpiryYear: 2030, CVC: "123", Name: "A Tester"}
}

func TestAuthorizeAndCapture(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()

	charge, err := g.Authorize(ctx, 50000, goodCard())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if charge.Last4 != "4242" || charge.Brand != "Visa" {
		t.Errorf("card details not derived: %+v", charge)
	}

	if _, err := g.Capture(ctx, charge.Reference, 20000); err != nil {
		t.Fatalf("first partial capture: %v", err)
	}
	if _, err := g.Capture(ctx, charge.Reference, 30000); err != nil {
		t.Fatalf("second partial capture: %v", err)
	}
	if _, err := g.Capture(ctx, charge.Reference, 1); err == nil {
		t.Error("capturing beyond the authorized amount should fail")
	}
}

func TestDeclinedCards(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()
	for _, num := range []string{"4000 0000 0000 0002", "4000 0000 0000 0069", "411"} {
		card := goodCard()
		card.Number = num
		if _, err := g.Authorize(ctx, 1000, card); !errors.Is(err, ErrDeclined) {
			t.Errorf("card %q: expected a decline, got %v", num, err)
		}
	}
}

func TestExpiredCardIsDeclined(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()
	card := goodCard()
	card.ExpiryMonth, card.ExpiryYear = 1, 2020
	if _, err := g.Authorize(ctx, 1000, card); !errors.Is(err, ErrDeclined) {
		t.Errorf("expected an expired card to be declined, got %v", err)
	}
}

func TestReleaseBlocksLaterCapture(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()
	charge, _ := g.Authorize(ctx, 10000, goodCard())

	if err := g.Release(ctx, charge.Reference); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := g.Capture(ctx, charge.Reference, 100); err == nil {
		t.Error("capturing a released hold should fail")
	}
}

func TestRefundLimitedToCapturedAmount(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()
	charge, _ := g.Authorize(ctx, 10000, goodCard())
	g.Capture(ctx, charge.Reference, 4000)

	if _, err := g.Refund(ctx, charge.Reference, 4000); err != nil {
		t.Fatalf("refunding the captured amount: %v", err)
	}
	if _, err := g.Refund(ctx, charge.Reference, 1); err == nil {
		t.Error("refunding more than was captured should fail")
	}
}

func TestUnknownChargeReference(t *testing.T) {
	g, ctx := NewFakeGateway(), context.Background()
	g.AdoptUnknown = false
	if _, err := g.Capture(ctx, "auth_nope", 100); !errors.Is(err, ErrUnknownCharge) {
		t.Errorf("expected ErrUnknownCharge, got %v", err)
	}
	if err := g.Release(ctx, "auth_nope"); !errors.Is(err, ErrUnknownCharge) {
		t.Errorf("expected ErrUnknownCharge, got %v", err)
	}
}

func TestCardBrandDetection(t *testing.T) {
	for num, want := range map[string]string{
		"4242424242424242": "Visa",
		"5555555555554444": "Mastercard",
		"378282246310005":  "American Express",
		"6011111111111117": "Discover",
		"9999999999999999": "Card",
	} {
		if got := (Card{Number: num}).Brand(); got != want {
			t.Errorf("Brand(%s) = %q, want %q", num, got, want)
		}
	}
}

func TestAdoptedHoldCanBeSettled(t *testing.T) {
	// A hold placed before this process started — seeded data, or a restart
	// mid-rental — must still be capturable, or a returned rental could never
	// take its late fees out of the deposit.
	g, ctx := NewFakeGateway(), context.Background()

	if _, err := g.Capture(ctx, "auth_from_a_previous_run", 5000); err != nil {
		t.Fatalf("adopting a recorded hold: %v", err)
	}
	if err := g.Release(ctx, "auth_from_a_previous_run"); err != nil {
		t.Errorf("releasing an adopted hold: %v", err)
	}
	if _, err := g.Capture(ctx, "not_one_of_ours", 5000); !errors.Is(err, ErrUnknownCharge) {
		t.Errorf("a reference in another format must not be adopted, got %v", err)
	}
}
