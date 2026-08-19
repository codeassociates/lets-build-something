// Package worker drains the job queue: reminder emails, overdue notices, and
// the daily sweep that keeps them scheduled.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/jobs"
	"github.com/codeassociates/lets-build-something/backend/internal/money"
	"github.com/codeassociates/lets-build-something/backend/internal/notify"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

type Worker struct {
	queue    *jobs.Queue
	rentals  *rental.Service
	billing  *billing.Store
	notifier *notify.Notifier
	users    *auth.Store

	pollInterval    time.Duration
	batchSize       int
	lateFeeMultiple float64
}

type Options struct {
	PollInterval    time.Duration
	BatchSize       int
	LateFeeMultiple float64
}

func New(q *jobs.Queue, r *rental.Service, b *billing.Store, n *notify.Notifier, u *auth.Store, o Options) *Worker {
	if o.PollInterval <= 0 {
		o.PollInterval = 15 * time.Second
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 10
	}
	if o.LateFeeMultiple <= 0 {
		o.LateFeeMultiple = 1.5
	}
	return &Worker{
		queue: q, rentals: r, billing: b, notifier: n, users: u,
		pollInterval: o.PollInterval, batchSize: o.BatchSize,
		lateFeeMultiple: o.LateFeeMultiple,
	}
}

// Run polls until the context is cancelled. It also schedules the daily sweep
// on startup, so a fresh deployment starts chasing overdue rentals without
// anyone having to seed the queue by hand.
func (w *Worker) Run(ctx context.Context) error {
	slog.Info("worker started", "poll_interval", w.pollInterval)

	if err := w.ensureSweepScheduled(ctx); err != nil {
		slog.Error("could not schedule the daily sweep", "err", err)
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Jobs abandoned by a worker that died are returned to the queue.
	if n, err := w.queue.ReleaseStale(ctx, 10*time.Minute); err == nil && n > 0 {
		slog.Info("released stale jobs", "count", n)
	}

	for {
		if err := w.tick(ctx); err != nil && ctx.Err() == nil {
			slog.Error("worker tick failed", "err", err)
		}
		select {
		case <-ctx.Done():
			slog.Info("worker stopping")
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	claimed, err := w.queue.Claim(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, job := range claimed {
		w.run(ctx, job)
	}
	return nil
}

// run executes one job. A handler error is never fatal: it is recorded and the
// job is retried with backoff.
func (w *Worker) run(ctx context.Context, job jobs.Job) {
	start := time.Now()
	err := w.dispatch(ctx, job)
	if err != nil {
		slog.Error("job failed", "id", job.ID, "kind", job.Kind,
			"attempt", job.Attempts, "err", err)
		if failErr := w.queue.Fail(ctx, job, err); failErr != nil {
			slog.Error("could not record job failure", "id", job.ID, "err", failErr)
		}
		return
	}
	if err := w.queue.Complete(ctx, job.ID); err != nil {
		slog.Error("could not mark job done", "id", job.ID, "err", err)
		return
	}
	slog.Info("job done", "id", job.ID, "kind", job.Kind, "took", time.Since(start))
}

func (w *Worker) dispatch(ctx context.Context, job jobs.Job) error {
	switch job.Kind {
	case jobs.KindBookingConfirm:
		return w.sendReservationEmail(ctx, job, notify.TemplateBookingConfirmation)
	case jobs.KindPickupReminder:
		return w.sendPickupReminder(ctx, job)
	case jobs.KindReturnReminder:
		return w.sendReturnReminder(ctx, job)
	case jobs.KindOverdueNotice:
		return w.sendOverdueNotice(ctx, job)
	case jobs.KindReceipt:
		return w.sendReceipt(ctx, job)
	case jobs.KindSweep:
		return w.sweep(ctx)
	default:
		return fmt.Errorf("no handler for job kind %q", job.Kind)
	}
}

type reservationPayload struct {
	ReservationID int64 `json:"reservation_id"`
}

func (w *Worker) loadReservation(ctx context.Context, job jobs.Job) (*rental.Reservation, error) {
	var p reservationPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("decoding payload: %w", err)
	}
	res, err := w.rentals.Get(ctx, p.ReservationID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("reservation %d no longer exists", p.ReservationID)
	}
	return res, nil
}

func (w *Worker) sendReservationEmail(ctx context.Context, job jobs.Job, template string) error {
	res, err := w.loadReservation(ctx, job)
	if err != nil {
		return err
	}
	return w.notifier.SendReservationEmail(ctx, template, res, notify.Data{})
}

// A reminder for a booking that has already been collected or cancelled is not
// a failure — the job is simply obsolete.
func (w *Worker) sendPickupReminder(ctx context.Context, job jobs.Job) error {
	res, err := w.loadReservation(ctx, job)
	if err != nil {
		return err
	}
	if res.Status != rental.StatusConfirmed {
		slog.Info("skipping pickup reminder", "reservation", res.ReservationNumber,
			"status", res.Status)
		return nil
	}
	return w.notifier.SendReservationEmail(ctx, notify.TemplatePickupReminder, res, notify.Data{})
}

func (w *Worker) sendReturnReminder(ctx context.Context, job jobs.Job) error {
	res, err := w.loadReservation(ctx, job)
	if err != nil {
		return err
	}
	if res.Status != rental.StatusPickedUp {
		slog.Info("skipping return reminder", "reservation", res.ReservationNumber,
			"status", res.Status)
		return nil
	}
	return w.notifier.SendReservationEmail(ctx, notify.TemplateReturnReminder, res, notify.Data{})
}

func (w *Worker) sendOverdueNotice(ctx context.Context, job jobs.Job) error {
	res, err := w.loadReservation(ctx, job)
	if err != nil {
		return err
	}
	if res.Status != rental.StatusPickedUp || !res.IsOverdue {
		return nil
	}
	return w.notifier.SendReservationEmail(ctx, notify.TemplateOverdueNotice, res, notify.Data{
		DaysOverdue:  res.DaysOverdue,
		LateFeeCents: w.accruedLateFee(res),
	})
}

func (w *Worker) sendReceipt(ctx context.Context, job jobs.Job) error {
	res, err := w.loadReservation(ctx, job)
	if err != nil {
		return err
	}

	invoices, _, err := w.billing.ListInvoices(ctx, billing.InvoiceFilter{
		CustomerID: res.CustomerID, Limit: 200,
	})
	if err != nil {
		return err
	}
	var outstanding, lateFees money.Cents
	for _, inv := range invoices {
		if inv.ReservationID != res.ID || inv.Status == "void" {
			continue
		}
		outstanding += inv.BalanceCents()
		for _, line := range inv.Lines {
			if line.Kind == "late_fee" {
				lateFees += line.AmountCents
			}
		}
	}
	if outstanding < 0 {
		outstanding = 0
	}

	return w.notifier.SendReservationEmail(ctx, notify.TemplateReceipt, res, notify.Data{
		AmountDue:    outstanding,
		LateFeeCents: lateFees,
		DaysOverdue:  res.DaysOverdue,
	})
}

func (w *Worker) accruedLateFee(res *rental.Reservation) money.Cents {
	var total money.Cents
	for _, item := range res.Items {
		total += money.LateFee(item.DailyRateCents, res.DaysOverdue, w.lateFeeMultiple) *
			money.Cents(item.Quantity)
	}
	return total
}

// sweep runs once a day: chase every overdue rental, tidy expired sessions, and
// schedule tomorrow's sweep.
func (w *Worker) sweep(ctx context.Context) error {
	overdue, err := w.rentals.DueForOverdueNotice(ctx)
	if err != nil {
		return err
	}
	for _, id := range overdue {
		// The dedupe key includes the date, so a rental that stays overdue is
		// chased once a day rather than once ever.
		key := fmt.Sprintf("overdue:%d:%s", id, time.Now().UTC().Format("2006-01-02"))
		if _, err := w.queue.Schedule(ctx, jobs.Enqueue{
			Kind:      jobs.KindOverdueNotice,
			Payload:   map[string]any{"reservation_id": id},
			DedupeKey: key,
		}); err != nil {
			slog.Error("could not queue overdue notice", "reservation", id, "err", err)
		}
	}
	if n, err := w.users.PurgeExpiredSessions(ctx); err == nil && n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}

	slog.Info("daily sweep complete", "overdue_chased", len(overdue))
	return w.scheduleNextSweep(ctx)
}

func (w *Worker) ensureSweepScheduled(ctx context.Context) error {
	return w.scheduleNextSweep(ctx)
}

// scheduleNextSweep books the sweep for 07:00 UTC tomorrow, or today if that
// has not passed yet.
func (w *Worker) scheduleNextSweep(ctx context.Context) error {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	_, err := w.queue.Schedule(ctx, jobs.Enqueue{
		Kind:      jobs.KindSweep,
		RunAt:     next,
		DedupeKey: "sweep:" + next.Format("2006-01-02"),
	})
	return err
}
