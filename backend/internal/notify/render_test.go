package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/httpx"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

func sampleData() Data {
	start := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	return Data{
		CompanyName:  "Kestrel Equipment Rental",
		BaseURL:      "http://localhost:5173",
		DaysOverdue:  3,
		LateFeeCents: 29250,
		AmountDue:    12500,
		Reservation: &rental.Reservation{
			ID: 42, ReservationNumber: "R-2026-00042",
			CustomerID: 7, CustomerName: "Dana Whitfield", CustomerEmail: "dana@example.com",
			Status:    rental.StatusConfirmed,
			StartDate: httpx.Date(start), EndDate: httpx.Date(start.AddDate(0, 0, 4)),
			SubtotalCents: 52000, TaxCents: 4420, DepositCents: 20000, TotalCents: 56420,
			RentalDays: 5,
			Items: []rental.Item{{
				ID: 1, ModelName: "Bosch 14lb Breaker <Electric>", SKU: "JH-1400",
				Quantity: 2, RateBasis: "weekly", RateCents: 26000,
				BillablePeriods: 1, LineTotalCents: 52000, DailyRateCents: 6500,
			}},
		},
	}
}

func TestEveryTemplateRenders(t *testing.T) {
	names := []string{
		TemplateBookingConfirmation, TemplatePickupReminder, TemplateReturnReminder,
		TemplateOverdueNotice, TemplateReceipt,
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			subject, html, text, err := Render(name, sampleData())
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if subject == "" {
				t.Error("empty subject")
			}
			for label, body := range map[string]string{"html": html, "text": text} {
				if body == "" {
					t.Errorf("empty %s body", label)
				}
				if !strings.Contains(body, "R-2026-00042") && label == "text" {
					// The reservation number belongs in every message.
					t.Errorf("%s body does not mention the reservation number:\n%s", label, body)
				}
				if strings.Contains(body, "%!") {
					t.Errorf("%s body has a formatting error: %s", label, body)
				}
			}
			if !strings.Contains(html, "<!doctype html>") {
				t.Error("html body is not a complete document")
			}
			if !strings.Contains(text, "Dana") {
				t.Error("text body does not greet the customer")
			}
		})
	}
}

func TestRenderEscapesUserContentInHTML(t *testing.T) {
	_, html, _, err := Render(TemplateBookingConfirmation, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<Electric>") {
		t.Error("equipment names are interpolated into HTML without escaping")
	}
	if !strings.Contains(html, "&lt;Electric&gt;") {
		t.Error("expected the escaped form of the equipment name")
	}
}

func TestUnknownTemplateIsAnError(t *testing.T) {
	if _, _, _, err := Render("no_such_template", sampleData()); err == nil {
		t.Error("expected an error for an unknown template")
	}
}

func TestReceiptDropsBalanceCopyWhenNothingOwed(t *testing.T) {
	d := sampleData()
	d.AmountDue, d.LateFeeCents = 0, 0

	_, html, text, err := Render(TemplateReceipt, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{html, text} {
		if strings.Contains(body, "Settle your balance") {
			t.Error("a fully paid rental should not be asked to settle a balance")
		}
		if !strings.Contains(body, "deposit hold has been released") {
			t.Error("a fully paid rental should be told its deposit was released")
		}
	}
}

func TestFirstName(t *testing.T) {
	for in, want := range map[string]string{
		"Dana Whitfield": "Dana", "Cher": "Cher", "": "there",
	} {
		if got := firstName(in); got != want {
			t.Errorf("firstName(%q) = %q, want %q", in, got, want)
		}
	}
}
