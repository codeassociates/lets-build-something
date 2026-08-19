import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Page } from '../../components/Layout'
import { InvoiceCard } from '../../components/InvoiceCard'
import { CheckoutPanel } from './CheckoutPanel'
import { CheckinPanel } from './CheckinPanel'
import { ErrorNote, Spinner, StatusPill } from '../../components/ui'
import { api } from '../../api/client'
import type { Invoice, Reservation } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { formatDate, formatDateTime, money, plural, relativeDay } from '../../format'

export function DeskReservation() {
  const { id } = useParams<{ id: string }>()
  const [flash, setFlash] = useState<string | null>(null)

  const { data, loading, error, reload } = useApi(
    () => api.get<{ reservation: Reservation; invoices: Invoice[] }>(`/reservations/${id}`), [id])

  if (loading && !data) return <Page><Spinner /></Page>
  if (error) return <Page narrow><ErrorNote error={error} /></Page>
  if (!data) return null

  const r = data.reservation
  const outstanding = data.invoices
    .filter(i => i.status !== 'void')
    .reduce((s, i) => s + (i.total_cents - i.amount_paid_cents), 0)

  return (
    <Page>
      <div className="small dim" style={{ marginBottom: 12 }}>
        <Link to="/desk" className="link-plain">Service desk</Link> › {r.reservation_number}
      </div>

      {flash && <div className="note note-ok" style={{ marginBottom: 16 }}>{flash}</div>}

      <div className="spread" style={{ marginBottom: 18 }}>
        <div>
          <div className="row-tight" style={{ marginBottom: 5 }}>
            <span className="mono dim">{r.reservation_number}</span>
            <StatusPill status={r.status} overdue={r.is_overdue} />
          </div>
          <h1>{r.customer_name}</h1>
          <p className="dim small" style={{ marginTop: 5 }}>
            {r.customer_email}
            {r.customer_phone && <> · {r.customer_phone}</>}
          </p>
        </div>
        <div className="right">
          <div className="tabular strong" style={{ fontSize: 20 }}>{money(r.total_cents)}</div>
          {outstanding > 0
            ? <div className="small" style={{ color: 'var(--alert)' }}>
                {money(outstanding)} outstanding
              </div>
            : <div className="small" style={{ color: 'var(--ok)' }}>Paid in full</div>}
        </div>
      </div>

      {r.is_overdue && (
        <div className="note note-alert" style={{ marginBottom: 18 }}>
          <strong>{r.days_overdue} {plural(r.days_overdue, 'day')} overdue.</strong>{' '}
          Late fees are calculated automatically when this is checked in.
        </div>
      )}

      <div className="cols">
        <div className="stack">
          {r.status === 'confirmed' && (
            <CheckoutPanel reservation={r}
              onDone={() => { setFlash('Equipment handed over. The rental is now out on hire.'); reload() }} />
          )}
          {r.status === 'picked_up' && (
            <CheckinPanel reservation={r}
              onDone={summary => { setFlash(summary); reload() }} />
          )}

          <div className="card">
            <div className="card-head"><h2>Equipment on this booking</h2></div>
            <div className="table-wrap">
              <table className="data">
                <thead>
                  <tr><th>Item</th><th>Units</th><th className="right">Line total</th></tr>
                </thead>
                <tbody>
                  {r.items.map(item => (
                    <tr key={item.id}>
                      <td>
                        <div className="strong">{item.quantity} × {item.model_name}</div>
                        <div className="tiny muted mono">{item.sku}</div>
                        <div className="tiny dim">
                          {item.billable_periods} × {item.rate_basis} @ {money(item.rate_cents)}
                        </div>
                      </td>
                      <td className="small">
                        {item.assignments.length === 0
                          ? <span className="muted">Not yet assigned</span>
                          : item.assignments.map(a => (
                              <div key={a.id}>
                                <span className="mono">{a.asset_tag}</span>
                                {a.checked_in_at
                                  ? <span className="muted"> · in {formatDate(a.checked_in_at)}</span>
                                  : <span className="dim"> · out</span>}
                                {a.damage_cents > 0 && (
                                  <span style={{ color: 'var(--alert)' }}>
                                    {' '}· {money(a.damage_cents)} damage
                                  </span>
                                )}
                                {a.checkin_notes && (
                                  <div className="tiny muted">{a.checkin_notes}</div>
                                )}
                              </div>
                            ))}
                      </td>
                      <td className="right tabular">{money(item.line_total_cents)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {data.invoices.map(inv => (
            <InvoiceCard key={inv.id} invoice={inv} onPaid={reload} />
          ))}
        </div>

        <div className="stack">
          <div className="card">
            <div className="card-head"><h3>Booking</h3></div>
            <div className="card-pad">
              <dl className="spec-list">
                <div><dt>Collect</dt>
                  <dd>{formatDate(r.start_date)}<br />
                    <span className="tiny muted">{relativeDay(r.start_date)}</span></dd></div>
                <div><dt>Return by</dt>
                  <dd>{formatDate(r.end_date)}<br />
                    <span className="tiny muted">{relativeDay(r.end_date)}</span></dd></div>
                <div><dt>Duration</dt><dd>{r.rental_days} {plural(r.rental_days, 'day')}</dd></div>
                <div><dt>Deposit held</dt><dd>{money(r.deposit_cents)}</dd></div>
                <div><dt>Booked</dt><dd>{formatDateTime(r.created_at)}</dd></div>
                {r.picked_up_at && (
                  <div><dt>Collected</dt><dd>{formatDateTime(r.picked_up_at)}</dd></div>
                )}
                {r.returned_at && (
                  <div><dt>Returned</dt><dd>{formatDateTime(r.returned_at)}</dd></div>
                )}
              </dl>
            </div>
          </div>

          {r.notes && (
            <div className="card card-pad">
              <div className="eyebrow" style={{ marginBottom: 6 }}>Notes</div>
              <div className="small" style={{ whiteSpace: 'pre-wrap' }}>{r.notes}</div>
            </div>
          )}

          <div className="card card-pad">
            <div className="eyebrow" style={{ marginBottom: 6 }}>Customer</div>
            <div className="small">
              <div className="strong">{r.customer_name}</div>
              <div className="dim">{r.customer_email}</div>
              {r.customer_phone && <div className="dim">{r.customer_phone}</div>}
            </div>
          </div>
        </div>
      </div>
    </Page>
  )
}
