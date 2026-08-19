import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { DateRange } from '../../components/DateRange'
import { EquipmentIcon } from '../../components/EquipmentIcon'
import { ErrorNote, Empty, Field, Pill } from '../../components/ui'
import { CardForm, type CardDetails, emptyCard } from '../../components/CardForm'
import { api } from '../../api/client'
import type { BookResult, Quote } from '../../api/types'
import { useApi, useAction } from '../../hooks/useApi'
import { useAuth } from '../../auth'
import { useCart } from '../../cart'
import { money, plural } from '../../format'

export function Checkout() {
  const cart = useCart()
  const { user } = useAuth()
  const navigate = useNavigate()
  const [notes, setNotes] = useState('')
  const [card, setCard] = useState<CardDetails>(emptyCard)
  const [payNow, setPayNow] = useState(true)

  // Re-priced by the server on every change, so what is shown is what is charged.
  const quote = useApi(async () => {
    if (cart.lines.length === 0) return null
    const { quote } = await api.post<{ quote: Quote }>('/quote', {
      start_date: cart.startDate,
      end_date: cart.endDate,
      items: cart.lines.map(l => ({ model_id: l.model_id, quantity: l.quantity })),
    })
    return quote
  }, [cart.lines, cart.startDate, cart.endDate])

  const book = useAction(async () => {
    const result = await api.post<BookResult>('/reservations', {
      start_date: cart.startDate,
      end_date: cart.endDate,
      items: cart.lines.map(l => ({ model_id: l.model_id, quantity: l.quantity })),
      notes,
      card: payNow ? {
        number: card.number, expiry_month: Number(card.month),
        expiry_year: Number(card.year), cvc: card.cvc, name: card.name,
      } : null,
    })
    cart.clear()
    navigate(`/account/reservations/${result.reservation.id}`, {
      state: { justBooked: true, paymentError: result.payment_error },
    })
  })

  if (cart.lines.length === 0) {
    return (
      <Page narrow>
        <PageHead title="Your basket" />
        <div className="card">
          <Empty title="Your basket is empty">
            <Link to="/">Browse the catalog</Link> to add equipment.
          </Empty>
        </div>
      </Page>
    )
  }

  const q = quote.data

  return (
    <Page>
      <PageHead title="Your basket"
        subtitle="Check the dates and quantities, then confirm the booking." />

      <div className="cols">
        <div className="stack">
          <div className="card card-pad">
            <DateRange start={cart.startDate} end={cart.endDate} onChange={cart.setDates} />
          </div>

          <div className="card">
            <div className="card-head"><h2>Equipment</h2></div>
            <div className="table-wrap">
              <table className="data">
                <thead>
                  <tr>
                    <th>Item</th><th>Rate</th><th style={{ width: 110 }}>Quantity</th>
                    <th className="right">Line total</th><th></th>
                  </tr>
                </thead>
                <tbody>
                  {cart.lines.map(line => {
                    const priced = q?.lines.find(l => l.model_id === line.model_id)
                    return (
                      <tr key={line.model_id}>
                        <td>
                          <div className="row-tight">
                            <span style={{ width: 34, height: 34, flexShrink: 0 }}>
                              <EquipmentIcon category={line.category_slug} />
                            </span>
                            <span>
                              <div className="strong">{line.name}</div>
                              <div className="tiny muted mono">{line.sku}</div>
                            </span>
                          </div>
                          {priced && !priced.available && (
                            <div style={{ marginTop: 5 }}>
                              <Pill tone="pill-alert" dot>
                                Only {priced.available_units} free on these dates
                              </Pill>
                            </div>
                          )}
                          {priced?.requires_license && (
                            <div style={{ marginTop: 5 }}>
                              <Pill tone="pill-warn">Bring your operator ticket</Pill>
                            </div>
                          )}
                        </td>
                        <td className="small dim nowrap">
                          {priced
                            ? <>{priced.billable_periods} × {priced.rate_basis}<br />
                                <span className="tabular">{money(priced.rate_cents)}</span></>
                            : '—'}
                        </td>
                        <td>
                          <input type="number" min={1} max={99} value={line.quantity}
                            style={{ width: 74 }}
                            onChange={e => cart.setQuantity(line.model_id, Number(e.target.value))} />
                        </td>
                        <td className="right tabular strong">
                          {priced ? money(priced.line_total_cents) : '—'}
                        </td>
                        <td className="right">
                          <button className="btn btn-ghost btn-sm"
                            onClick={() => cart.remove(line.model_id)} aria-label="Remove">✕</button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>

          <div className="card card-pad">
            <Field label="Notes for the yard" hint="Delivery instructions, job details, anything we should know.">
              <textarea value={notes} onChange={e => setNotes(e.target.value)}
                placeholder="e.g. Collecting at 7am, will need the trailer hitch." />
            </Field>
          </div>

          {user && (
            <div className="card">
              <div className="card-head">
                <h2>Payment</h2>
                <label className="checkbox">
                  <input type="checkbox" checked={payNow} onChange={e => setPayNow(e.target.checked)} />
                  Pay now
                </label>
              </div>
              <div className="card-pad">
                {payNow
                  ? <CardForm card={card} onChange={setCard} deposit={q?.deposit_cents ?? 0} />
                  : <div className="note">
                      The booking will be held and invoiced. Payment is due before collection.
                    </div>}
              </div>
            </div>
          )}
        </div>

        <div className="card" style={{ position: 'sticky', top: 76 }}>
          <div className="card-head"><h2>Summary</h2></div>
          <div className="card-pad stack">
            <ErrorNote error={quote.error ?? book.error} />

            {q && (
              <div className="sum">
                <div className="sum-row">
                  <span className="label">
                    {q.rental_days} {plural(q.rental_days, 'day')} hire
                  </span>
                  <span className="tabular">{money(q.subtotal_cents)}</span>
                </div>
                <div className="sum-row">
                  <span className="label">Tax</span>
                  <span className="tabular">{money(q.tax_cents)}</span>
                </div>
                <div className="sum-row total">
                  <span>Hire total</span>
                  <span className="tabular">{money(q.total_cents)}</span>
                </div>
                <div className="sum-row">
                  <span className="label">Deposit hold (refundable)</span>
                  <span className="tabular">{money(q.deposit_cents)}</span>
                </div>
                <div className="sum-row total">
                  <span>Due today</span>
                  <span className="tabular">{money(q.due_now_cents)}</span>
                </div>
              </div>
            )}

            {q && !q.all_available && (
              <div className="note note-alert">
                Some items do not have enough stock for these dates. Adjust the quantity
                or choose different dates before booking.
              </div>
            )}

            {user ? (
              <button className="btn btn-block"
                disabled={book.busy || !q || !q.all_available}
                onClick={() => void book.run()}>
                {book.busy ? 'Confirming…' : 'Confirm booking'}
              </button>
            ) : (
              <>
                <Link to="/signin" state={{ from: '/checkout' }} className="btn btn-block">
                  Sign in to book
                </Link>
                <div className="tiny muted center">
                  Your basket is saved — you will come straight back here.
                </div>
              </>
            )}

            <div className="tiny muted">
              The deposit is a hold on your card, released when the equipment comes back
              undamaged. Late returns are charged at the daily rate plus 50%.
            </div>
          </div>
        </div>
      </div>
    </Page>
  )
}
