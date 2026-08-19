// Package money handles currency as integer cents and the rental rate maths.
// Nothing in this system ever holds money in a float.
package money

import (
	"fmt"
	"math"
	"time"
)

// Cents is a monetary amount in the smallest currency unit.
type Cents int64

func (c Cents) String() string {
	neg := c < 0
	v := int64(c)
	if neg {
		v = -v
	}
	s := fmt.Sprintf("$%d.%02d", v/100, v%100)
	if neg {
		return "-" + s
	}
	return s
}

// RateCard is the price list for one equipment model.
type RateCard struct {
	Daily   Cents
	Weekly  Cents
	Monthly Cents
}

// Quote is the chosen billing basis for a rental of a given length.
type Quote struct {
	Basis    string // "daily" | "weekly" | "monthly"
	Rate     Cents  // price of one billable period
	Periods  int    // how many periods are charged
	Subtotal Cents
}

// RentalDays counts inclusive calendar days: picking up and returning the same
// day is one day, not zero.
func RentalDays(start, end time.Time) int {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	days := int(e.Sub(s).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

// BestQuote picks whichever basis bills the customer least for the period, the
// way a rental counter would. A 9-day job is cheaper on 2 weekly periods than on
// 9 daily ones, so that is what it charges.
func BestQuote(rc RateCard, days int) Quote {
	if days < 1 {
		days = 1
	}

	candidates := []Quote{}
	if rc.Daily > 0 {
		candidates = append(candidates, quote("daily", rc.Daily, days))
	}
	if rc.Weekly > 0 {
		candidates = append(candidates, quote("weekly", rc.Weekly, ceilDiv(days, 7)))
	}
	if rc.Monthly > 0 {
		candidates = append(candidates, quote("monthly", rc.Monthly, ceilDiv(days, 30)))
	}
	if len(candidates) == 0 {
		return Quote{Basis: "daily", Rate: 0, Periods: days}
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Subtotal < best.Subtotal {
			best = c
		}
	}
	return best
}

func quote(basis string, rate Cents, periods int) Quote {
	if periods < 1 {
		periods = 1
	}
	return Quote{Basis: basis, Rate: rate, Periods: periods, Subtotal: rate * Cents(periods)}
}

func ceilDiv(a, b int) int {
	if a <= 0 {
		return 1
	}
	return (a + b - 1) / b
}

// Tax rounds half away from zero, matching how a register computes sales tax.
func Tax(subtotal Cents, ratePercent float64) Cents {
	if ratePercent <= 0 {
		return 0
	}
	return Cents(math.Round(float64(subtotal) * ratePercent / 100.0))
}

// LateFee charges each overdue day at the daily rate times a penalty multiple.
func LateFee(dailyRate Cents, overdueDays int, multiple float64) Cents {
	if overdueDays <= 0 {
		return 0
	}
	return Cents(math.Round(float64(dailyRate) * float64(overdueDays) * multiple))
}

// PeriodPhrase describes a billed period in words, for invoice lines and
// emails: "3 days at the daily rate", "1 week at the weekly rate".
func PeriodPhrase(basis string, periods int) string {
	unit := map[string]string{"daily": "day", "weekly": "week", "monthly": "month"}[basis]
	if unit == "" {
		unit = "period"
	}
	if periods != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s at the %s rate", periods, unit, basis)
}
