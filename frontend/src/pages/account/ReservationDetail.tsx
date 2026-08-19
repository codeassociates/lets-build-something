import { useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { Page } from '../../components/Layout'
import { EquipmentIcon } from '../../components/EquipmentIcon'
import { InvoiceCard } from '../../components/InvoiceCard'
import { ErrorNote, Modal, Spinner, StatusPill } from '../../components/ui'
import { api } from '../../api/client'
import type { Invoice, Reservation } from '../../api/types'
import { useApi, useAction } from '../../hooks/useApi'
import { useAuth } from '../../auth'
import { formatDate, formatDateTime, money, plural, relativeDay } from '../../format'

export function ReservationDetail() {
  const { id } = useParams<{ id: string }>()
  const { can } = useAuth()
  const location = useLocation()
  const state = location.state as { justBooked?: boolean; paymentError?: string } | null
  const [cancelling, setCancelling] = useState(false)

  const { data, loading, error, reload } = useApi(
    () => api.get<{ reservation: Reservation; invoices: Invoice[] }>(`/reservations/${id}`), [id])

  const cancel = useAction(async (reason: string) => {
    await api.post(`/reservations/${id}/cancel`, { reason })
    setCancelling(false)
    reload()
  })

  if (loading && !data) return <Page><Spinner /></Page>
  if (error) return <Page narrow><ErrorNote error={error} /></Page>
  if (!data) return null

  const r = data.reservation
  const outstanding = data.invoices
    .filter(i => i.status !== 'void')
    .reduce((sum, i) => sum + (i.total_cents - i.amount_paid_cents), 0)

  return (
    <Page>
      {state?.justBooked && (
        <div className={`note ${state.paymentError ? 'note-warn' : 'note-ok'}`}
          style={{ marginBottom: 18 }}>
          <strong>{state.paymentError ? 'Booking confirmed — payment did not go through' : 'Booking confirmed'}</strong>
          <div>
            {state.paymentError
              ? <>Your equipment is reserved, but the card was declined: {state.paymentError}.
                  You can settle the invoice below or pay at the counter.</>
              : <>We have emailed your confirmation. Bring photo ID and the card you paid with
                  when you collect.</>}
          </div>
        </div>
      )}

      <div className="spread" style={{ marginBottom: 18 }}>
        <div>
          <div className="row-tight" style={{ marginBottom: 5 }}>
            <span className="mono dim">{r.reservation_number}</span>
            <StatusPill status={r.status} overdue={r.is_overdue} />
          </div>
          <h1>{r.rental_days} {plural(r.rental_days, 'day')} hire</h1>
          <p className="dim small" style={{ marginTop: 5 }}>
            Collect {formatDate(r.start_date)} · return by {formatDate(r.end_date)}
            {r.status === 'confirmed' && <> ({relativeDay(r.start_date)})</>}
          </p>
        </div>
        <div className="row">
          {can('staff') && (
            <Link to={`/desk/reservations/${r.id}`} className="btn btn-secondary">
              Open at the desk
            </Link>
          )}
          {r.status === 'confirmed' && (
            <button className="btn btn-secondary" onClick={() => setCancelling(true)}>
              Cancel booking
            </button>
          )}
        </div>
      </div>

      {r.is_overdue && (
        <div className="note note-alert" style={{ marginBottom: 18 }}>
          <strong>This rental is {r.days_overdue} {plural(r.days_overdue, 'day')} overdue.</strong>
          <div>
            Please return the equipment as soon as you can — late charges accrue daily at the
            daily rate plus 50%. Call the yard on (406) 555-0134 if you need an extension.
          </div>
        </div>
      )}

      <div className="cols">
        <div className="stack">
          <div className="card">
            <div className="card-head"><h2>Equipment</h2></div>
            <div className="table-wrap">
              <table className="data">
                <thead>
                  <tr>
                    <th>Item</th><th>Rate</th><th className="right">Line total</th>
                  </tr>
                </thead>
                <tbody>
                  {r.items.map(item => (
                    <tr key={item.id}>
                      <td>
                        <div className="row-tight">
                          <span style={{ width: 34, height: 34, flexShrink: 0 }}>
                            <EquipmentIcon category={categoryOf(item.sku)} />
                          </span>
                          <span>
                            <div className="strong">{item.quantity} × {item.model_name}</div>
                            <div className="tiny muted mono">{item.sku}</div>
                            {item.assignments.length > 0 && (
                              <div className="tiny dim" style={{ marginTop: 3 }}>
                                {item.assignments.map(a => (
                                  <div key={a.id}>
                                    Unit <span className="mono">{a.asset_tag}</span>
                                    {a.checked_in_at
                                      ? <> · returned {formatDate(a.checked_in_at)}</>
                                      : <> · out since {formatDate(a.checked_out_at)}</>}
                                  </div>
                                ))}
                              </div>
                            )}
                          </span>
                        </div>
                      </td>
                      <td className="small dim nowrap">
                        {item.billable_periods} × {item.rate_basis}<br />
                        <span className="tabular">{money(item.rate_cents)}</span>
                      </td>
                      <td className="right tabular strong">{money(item.line_total_cents)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {data.invoices.map(inv => (
            <InvoiceCard key={inv.id} invoice={inv} onPaid={reload} />
          ))}

          {r.notes && (
            <div className="card card-pad">
              <div className="eyebrow" style={{ marginBottom: 6 }}>Notes</div>
              <div className="small" style={{ whiteSpace: 'pre-wrap' }}>{r.notes}</div>
            </div>
          )}
        </div>

        <div className="stack">
          <div className="card">
            <div className="card-head"><h3>Summary</h3></div>
            <div className="card-pad">
              <div className="sum">
                <div className="sum-row">
                  <span className="label">Hire</span>
                  <span className="tabular">{money(r.subtotal_cents)}</span>
                </div>
                <div className="sum-row">
                  <span className="label">Tax</span>
                  <span className="tabular">{money(r.tax_cents)}</span>
                </div>
                <div className="sum-row total">
                  <span>Total</span>
                  <span className="tabular">{money(r.total_cents)}</span>
                </div>
                <div className="sum-row">
                  <span className="label">Deposit held</span>
                  <span className="tabular">{money(r.deposit_cents)}</span>
                </div>
                {outstanding > 0 && (
                  <div className="sum-row total" style={{ color: 'var(--alert)' }}>
                    <span>Outstanding</span>
                    <span className="tabular">{money(outstanding)}</span>
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-head"><h3>Timeline</h3></div>
            <div className="card-pad">
              <dl className="spec-list">
                <div><dt>Booked</dt><dd>{formatDateTime(r.created_at)}</dd></div>
                <div><dt>Collect</dt><dd>{formatDate(r.start_date)}</dd></div>
                <div><dt>Return by</dt><dd>{formatDate(r.end_date)}</dd></div>
                {r.picked_up_at && (
                  <div><dt>Collected</dt><dd>{formatDateTime(r.picked_up_at)}</dd></div>
                )}
                {r.returned_at && (
                  <div><dt>Returned</dt><dd>{formatDateTime(r.returned_at)}</dd></div>
                )}
              </dl>
            </div>
          </div>

          <div className="card card-pad small dim">
            <div className="eyebrow" style={{ marginBottom: 6 }}>Collection</div>
            1420 Gallatin Road, Bozeman MT<br />
            Counter open 7:00am – 5:30pm<br />
            Bring photo ID and the card used to book.
          </div>
        </div>
      </div>

      {cancelling && (
        <CancelDialog busy={cancel.busy} error={cancel.error}
          onClose={() => setCancelling(false)}
          onConfirm={reason => void cancel.run(reason)} />
      )}
    </Page>
  )
}

function CancelDialog({ onClose, onConfirm, busy, error }: {
  onClose: () => void
  onConfirm: (reason: string) => void
  busy: boolean
  error: ReturnType<typeof useAction>['error']
}) {
  const [reason, setReason] = useState('')
  return (
    <Modal title="Cancel this booking?" onClose={onClose}
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Keep it</button>
        <button className="btn btn-danger" disabled={busy} onClick={() => onConfirm(reason)}>
          {busy ? 'Cancelling…' : 'Cancel booking'}
        </button>
      </>}>
      <div className="stack">
        <ErrorNote error={error} />
        <p className="small dim">
          The equipment goes back into stock and any deposit hold is released.
          The invoice will be voided.
        </p>
        <div className="field">
          <label>Reason (optional)</label>
          <textarea value={reason} onChange={e => setReason(e.target.value)}
            placeholder="Job postponed, weather, changed requirements…" />
        </div>
      </div>
    </Modal>
  )
}

/** SKUs carry the category prefix, so artwork works without another lookup. */
function categoryOf(sku: string): string {
  const map: Record<string, string> = {
    BRK: 'breaking', CON: 'concrete', PMP: 'pumps', PNT: 'painting',
    CMP: 'compaction', PWR: 'power', ACC: 'access', CLM: 'climate',
  }
  return map[sku.slice(0, 3)] ?? ''
}
