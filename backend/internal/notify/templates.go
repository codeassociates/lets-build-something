package notify

import (
	"embed"
	"fmt"
	"strings"

	"github.com/codeassociates/lets-build-something/backend/internal/money"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

//go:embed templates/*.html
var templateFS embed.FS

// Template names double as job kinds and as the value stored on each sent
// email, so a message in the log can always be traced back to its source.
const (
	TemplateBookingConfirmation = "booking_confirmation"
	TemplatePickupReminder      = "pickup_reminder"
	TemplateReturnReminder      = "return_reminder"
	TemplateOverdueNotice       = "overdue_notice"
	TemplateReceipt             = "rental_receipt"
)

type emailTemplate struct {
	subject func(Data) string
	heading func(Data) string
	blocks  func(Data) []block
}

var templates = map[string]emailTemplate{

	TemplateBookingConfirmation: {
		subject: func(d Data) string {
			return fmt.Sprintf("Booking confirmed — %s", d.Reservation.ReservationNumber)
		},
		heading: func(d Data) string { return "Your equipment is reserved" },
		blocks: func(d Data) []block {
			r := d.Reservation
			return []block{
				para(fmt.Sprintf("Thanks %s — we've put the following aside for you.",
					firstName(r.CustomerName))),
				facts{
					{"Reservation", r.ReservationNumber},
					{"Pickup", fmtDate(r.StartDate.Time())},
					{"Return by", fmtDate(r.EndDate.Time())},
					{"Duration", fmt.Sprintf("%d day%s", r.RentalDays, plural(r.RentalDays))},
				},
				itemList(r.Items),
				facts{
					{"Rental subtotal", r.SubtotalCents.String()},
					{"Tax", r.TaxCents.String()},
					{"Total", r.TotalCents.String()},
					{"Refundable deposit", r.DepositCents.String()},
				},
				para("Please bring photo ID and the card used to book when you collect."),
				button{"View your reservation", reservationURL(d)},
			}
		},
	},

	TemplatePickupReminder: {
		subject: func(d Data) string {
			return fmt.Sprintf("Pickup tomorrow — %s", d.Reservation.ReservationNumber)
		},
		heading: func(d Data) string { return "Your equipment is ready to collect" },
		blocks: func(d Data) []block {
			r := d.Reservation
			return []block{
				para(fmt.Sprintf("Hi %s — a reminder that your rental starts on %s.",
					firstName(r.CustomerName), fmtDate(r.StartDate.Time()))),
				facts{
					{"Reservation", r.ReservationNumber},
					{"Collect from", "1420 Gallatin Road, Bozeman"},
					{"Counter hours", "7:00am – 5:30pm"},
					{"Return by", fmtDate(r.EndDate.Time())},
				},
				itemList(r.Items),
				callout{"warn", "Bring photo ID and the payment card used to book. " +
					"We hold reservations until close of business on the pickup date."},
				button{"View your reservation", reservationURL(d)},
			}
		},
	},

	TemplateReturnReminder: {
		subject: func(d Data) string {
			return fmt.Sprintf("Return due %s — %s",
				d.Reservation.EndDate.Time().Format("2 Jan"), d.Reservation.ReservationNumber)
		},
		heading: func(d Data) string { return "Your rental is due back" },
		blocks: func(d Data) []block {
			r := d.Reservation
			return []block{
				para(fmt.Sprintf("Hi %s — your rental is due back on %s.",
					firstName(r.CustomerName), fmtDate(r.EndDate.Time()))),
				facts{
					{"Reservation", r.ReservationNumber},
					{"Return by", fmtDate(r.EndDate.Time())},
					{"Return to", "1420 Gallatin Road, Bozeman"},
					{"Counter hours", "7:00am – 5:30pm"},
				},
				itemList(r.Items),
				para("Need it for longer? Reply to this email or call the yard before the " +
					"return date and we'll extend it if the equipment is free."),
				callout{"warn", fmt.Sprintf(
					"Late returns are charged at %s per day per item.", "the daily rate plus 50%")},
				button{"View your reservation", reservationURL(d)},
			}
		},
	},

	TemplateOverdueNotice: {
		subject: func(d Data) string {
			return fmt.Sprintf("Overdue: %s — %d day%s late",
				d.Reservation.ReservationNumber, d.DaysOverdue, plural(d.DaysOverdue))
		},
		heading: func(d Data) string { return "This rental is overdue" },
		blocks: func(d Data) []block {
			r := d.Reservation
			return []block{
				callout{"alert", fmt.Sprintf(
					"%s was due back on %s and is now %d day%s overdue.",
					r.ReservationNumber, fmtDate(r.EndDate.Time()),
					d.DaysOverdue, plural(d.DaysOverdue))},
				para(fmt.Sprintf("Hi %s — please return the equipment below as soon as you can, "+
					"or call the yard to arrange an extension.", firstName(r.CustomerName))),
				itemList(r.Items),
				facts{
					{"Late charges so far", d.LateFeeCents.String()},
					{"Accruing", "per day until returned"},
				},
				button{"View your reservation", reservationURL(d)},
			}
		},
	},

	TemplateReceipt: {
		subject: func(d Data) string {
			return fmt.Sprintf("Thanks — %s returned", d.Reservation.ReservationNumber)
		},
		heading: func(d Data) string { return "Rental complete" },
		blocks: func(d Data) []block {
			r := d.Reservation
			bs := []block{
				para(fmt.Sprintf("Thanks %s — everything on %s has been checked back in.",
					firstName(r.CustomerName), r.ReservationNumber)),
				itemList(r.Items),
			}
			f := facts{
				{"Rental total", r.TotalCents.String()},
			}
			if d.LateFeeCents > 0 {
				f = append(f, factRow{
					fmt.Sprintf("Late fees (%d day%s)", d.DaysOverdue, plural(d.DaysOverdue)),
					d.LateFeeCents.String()})
			}
			if d.AmountDue > 0 {
				f = append(f, factRow{"Still outstanding", d.AmountDue.String()})
			}
			bs = append(bs, f)
			if d.AmountDue > 0 {
				bs = append(bs, callout{"warn",
					"There is a balance outstanding on this rental. You can settle it online."})
				bs = append(bs, button{"Settle your balance", d.BaseURL + "/account/invoices"})
			} else {
				bs = append(bs, para("Your deposit hold has been released — depending on your "+
					"bank it may take a few days to disappear from your statement."))
			}
			return append(bs, para("We hope the job went well. See you next time."))
		},
	},
}

func reservationURL(d Data) string {
	return fmt.Sprintf("%s/account/reservations/%d", strings.TrimRight(d.BaseURL, "/"),
		d.Reservation.ID)
}

func firstName(full string) string {
	if i := strings.IndexByte(full, ' '); i > 0 {
		return full[:i]
	}
	if full == "" {
		return "there"
	}
	return full
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

var (
	_ = money.Cents(0)
	_ = rental.Item{}
)
