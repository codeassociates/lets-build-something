package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/money"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

// Data is everything a template may reference. One struct for all templates
// keeps rendering uniform and makes a missing field a compile-time concern
// rather than a runtime surprise in a customer's inbox.
type Data struct {
	CompanyName string
	BaseURL     string
	Reservation *rental.Reservation
	// Extras set by particular templates.
	DaysOverdue  int
	LateFeeCents money.Cents
	AmountDue    money.Cents
	InvoiceURL   string
}

type rendered struct {
	Subject     string
	Heading     string
	Content     template.HTML
	CompanyName string
}

// Render produces the subject, HTML and plain-text bodies for a template.
func Render(name string, d Data) (subject, htmlBody, textBody string, err error) {
	t, ok := templates[name]
	if !ok {
		return "", "", "", fmt.Errorf("unknown email template %q", name)
	}

	subject = t.subject(d)
	blocks := t.blocks(d)

	var content strings.Builder
	for _, b := range blocks {
		content.WriteString(b.html())
	}

	var out bytes.Buffer
	err = layout.ExecuteTemplate(&out, "layout", rendered{
		Subject:     subject,
		Heading:     t.heading(d),
		Content:     template.HTML(content.String()),
		CompanyName: d.CompanyName,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("rendering %s: %w", name, err)
	}

	var text strings.Builder
	text.WriteString(t.heading(d) + "\n" + strings.Repeat("=", len(t.heading(d))) + "\n\n")
	for _, b := range blocks {
		text.WriteString(b.text())
	}
	text.WriteString("\n--\n" + d.CompanyName + " · 1420 Gallatin Road, Bozeman MT · (406) 555-0134\n")

	return subject, out.String(), text.String(), nil
}

var layout = template.Must(template.ParseFS(templateFS, "templates/layout.html"))

// ---------- content blocks ----------

// block is one piece of an email that knows how to render itself in both
// formats, so the HTML and text versions can never drift apart.
type block interface {
	html() string
	text() string
}

type para string

func (p para) html() string {
	return `<p style="margin:0 0 14px;font-size:15px;line-height:1.6;color:#3d3a35;">` +
		template.HTMLEscapeString(string(p)) + `</p>`
}
func (p para) text() string { return string(p) + "\n\n" }

// factRow is a labelled value, used for dates, totals and reference numbers.
type factRow struct{ label, value string }

type facts []factRow

func (f facts) html() string {
	var b strings.Builder
	b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" width="100%" ` +
		`style="margin:0 0 18px;border-collapse:collapse;">`)
	for i, row := range f {
		border := "border-bottom:1px solid #ecE9e3;"
		if i == len(f)-1 {
			border = ""
		}
		fmt.Fprintf(&b, `<tr><td style="padding:9px 0;%s font-size:13px;color:#77726b;">%s</td>`+
			`<td align="right" style="padding:9px 0;%s font-size:14px;font-weight:600;color:#22201d;">%s</td></tr>`,
			border, template.HTMLEscapeString(row.label),
			border, template.HTMLEscapeString(row.value))
	}
	b.WriteString(`</table>`)
	return b.String()
}

func (f facts) text() string {
	// The rendered label carries a trailing colon, so the column must allow for it.
	width := 0
	for _, row := range f {
		if n := len(row.label) + 1; n > width {
			width = n
		}
	}
	var b strings.Builder
	for _, row := range f {
		fmt.Fprintf(&b, "%-*s  %s\n", width, row.label+":", row.value)
	}
	b.WriteString("\n")
	return b.String()
}

// itemList renders the equipment on a reservation.
type itemList []rental.Item

func (l itemList) html() string {
	var b strings.Builder
	b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" width="100%" ` +
		`style="margin:0 0 18px;border-collapse:collapse;background:#faf9f7;border-radius:8px;">`)
	for _, it := range l {
		fmt.Fprintf(&b, `<tr><td style="padding:11px 14px;font-size:14px;color:#22201d;">`+
			`<strong>%d ×</strong> %s<div style="font-size:12px;color:#77726b;margin-top:2px;">%s</div></td>`+
			`<td align="right" style="padding:11px 14px;font-size:14px;font-weight:600;">%s</td></tr>`,
			it.Quantity, template.HTMLEscapeString(it.ModelName),
			template.HTMLEscapeString(fmt.Sprintf("%s · %s",
				it.SKU, money.PeriodPhrase(it.RateBasis, it.BillablePeriods))),
			it.LineTotalCents)
	}
	b.WriteString(`</table>`)
	return b.String()
}

func (l itemList) text() string {
	var b strings.Builder
	b.WriteString("Equipment\n---------\n")
	for _, it := range l {
		fmt.Fprintf(&b, "  %d × %-38s %10s\n", it.Quantity, truncateStr(it.ModelName, 38),
			it.LineTotalCents.String())
	}
	b.WriteString("\n")
	return b.String()
}

// button is a call to action; in text it degrades to a bare URL.
type button struct{ label, url string }

func (c button) html() string {
	return fmt.Sprintf(`<p style="margin:22px 0 6px;"><a href="%s" `+
		`style="display:inline-block;background:#1c3f39;color:#ffffff;text-decoration:none;`+
		`padding:11px 22px;border-radius:7px;font-size:14px;font-weight:600;">%s</a></p>`,
		template.HTMLEscapeString(c.url), template.HTMLEscapeString(c.label))
}

func (c button) text() string { return c.label + ": " + c.url + "\n\n" }

// callout highlights something the customer must not miss.
type callout struct{ tone, body string }

func (c callout) html() string {
	bg, border, fg := "#fdf6e6", "#e8c97a", "#6b5312"
	if c.tone == "alert" {
		bg, border, fg = "#fdefee", "#e5a49c", "#7d2b21"
	}
	return fmt.Sprintf(`<div style="margin:0 0 18px;padding:13px 15px;background:%s;`+
		`border:1px solid %s;border-radius:8px;font-size:14px;line-height:1.55;color:%s;">%s</div>`,
		bg, border, fg, template.HTMLEscapeString(c.body))
}

func (c callout) text() string {
	return "!! " + c.body + "\n\n"
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fmtDate(t time.Time) string { return t.Format("Monday 2 January 2006") }
