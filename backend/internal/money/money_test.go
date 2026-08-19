package money

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestRentalDaysIsInclusive(t *testing.T) {
	cases := []struct {
		start, end time.Time
		want       int
	}{
		{day(2026, 3, 1), day(2026, 3, 1), 1},
		{day(2026, 3, 1), day(2026, 3, 2), 2},
		{day(2026, 3, 1), day(2026, 3, 31), 31},
		{day(2026, 3, 2), day(2026, 3, 1), 1}, // reversed input clamps to one day
	}
	for _, c := range cases {
		if got := RentalDays(c.start, c.end); got != c.want {
			t.Errorf("RentalDays(%s, %s) = %d, want %d",
				c.start.Format("2006-01-02"), c.end.Format("2006-01-02"), got, c.want)
		}
	}
}

func TestBestQuotePicksCheapestBasis(t *testing.T) {
	rc := RateCard{Daily: 6500, Weekly: 26000, Monthly: 78000}

	cases := []struct {
		days      int
		wantBasis string
		wantTotal Cents
	}{
		{1, "daily", 6500},
		{3, "daily", 19500},
		{4, "weekly", 26000},   // 4 daily (26000) ties weekly; weekly must not be worse
		{9, "weekly", 52000},   // 2 weeks beats 9 days (58500)
		{25, "monthly", 78000}, // 1 month beats 4 weeks (104000)
		{45, "monthly", 156000},
	}
	for _, c := range cases {
		got := BestQuote(rc, c.days)
		if got.Subtotal != c.wantTotal {
			t.Errorf("BestQuote(%d days) subtotal = %d, want %d", c.days, got.Subtotal, c.wantTotal)
		}
		if got.Subtotal > rc.Daily*Cents(c.days) {
			t.Errorf("BestQuote(%d days) = %d, worse than straight daily %d",
				c.days, got.Subtotal, rc.Daily*Cents(c.days))
		}
	}
}

func TestBestQuoteHandlesSparseRateCard(t *testing.T) {
	only := RateCard{Daily: 5000}
	if q := BestQuote(only, 10); q.Basis != "daily" || q.Subtotal != 50000 {
		t.Errorf("daily-only card: got %+v", q)
	}
	if q := BestQuote(RateCard{}, 5); q.Subtotal != 0 {
		t.Errorf("empty card should quote zero, got %+v", q)
	}
}

func TestTaxRoundsHalfUp(t *testing.T) {
	if got := Tax(10000, 8.5); got != 850 {
		t.Errorf("Tax(10000, 8.5) = %d, want 850", got)
	}
	if got := Tax(1, 50); got != 1 { // 0.5 cents rounds away from zero
		t.Errorf("Tax(1, 50) = %d, want 1", got)
	}
	if got := Tax(10000, 0); got != 0 {
		t.Errorf("zero rate should yield zero tax, got %d", got)
	}
}

func TestLateFee(t *testing.T) {
	if got := LateFee(6500, 3, 1.5); got != 29250 {
		t.Errorf("LateFee = %d, want 29250", got)
	}
	if got := LateFee(6500, 0, 1.5); got != 0 {
		t.Errorf("no overdue days should be free, got %d", got)
	}
	if got := LateFee(6500, -2, 1.5); got != 0 {
		t.Errorf("early return should be free, got %d", got)
	}
}

func TestCentsFormatting(t *testing.T) {
	cases := map[Cents]string{0: "$0.00", 5: "$0.05", 12345: "$123.45", -250: "-$2.50"}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("Cents(%d).String() = %q, want %q", int64(in), got, want)
		}
	}
}

func TestPeriodPhrase(t *testing.T) {
	cases := []struct {
		basis   string
		periods int
		want    string
	}{
		{"daily", 1, "1 day at the daily rate"},
		{"daily", 3, "3 days at the daily rate"},
		{"weekly", 1, "1 week at the weekly rate"},
		{"weekly", 2, "2 weeks at the weekly rate"},
		{"monthly", 1, "1 month at the monthly rate"},
		{"monthly", 4, "4 months at the monthly rate"},
		// An unrecognised basis must still produce a readable line.
		{"fortnightly", 2, "2 periods at the fortnightly rate"},
	}
	for _, c := range cases {
		if got := PeriodPhrase(c.basis, c.periods); got != c.want {
			t.Errorf("PeriodPhrase(%q, %d) = %q, want %q", c.basis, c.periods, got, c.want)
		}
	}
}
