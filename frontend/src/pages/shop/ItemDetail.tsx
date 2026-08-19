import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Page } from '../../components/Layout'
import { DateRange } from '../../components/DateRange'
import { EquipmentIcon } from '../../components/EquipmentIcon'
import { ErrorNote, Pill, Spinner } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Model } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { useCart } from '../../cart'
import { money, plural, rentalDays } from '../../format'

export function ItemDetail() {
  const { id } = useParams<{ id: string }>()
  const cart = useCart()
  const navigate = useNavigate()
  const [quantity, setQuantity] = useState(1)

  const { data, loading, error } = useApi(
    () => api.get<{ model: Model }>(`/models/${id}` + qs({ start: cart.startDate, end: cart.endDate })),
    [id, cart.startDate, cart.endDate])

  if (loading && !data) return <Page><Spinner /></Page>
  if (error) return <Page narrow><ErrorNote error={error} /></Page>
  if (!data) return null

  const model = data.model
  const days = rentalDays(cart.startDate, cart.endDate)
  const enough = model.available_units >= quantity
  const best = bestRate(model, days)

  return (
    <Page>
      <div className="small dim" style={{ marginBottom: 12 }}>
        <Link to="/" className="link-plain">Hire</Link> › {model.category_name} › {model.name}
      </div>

      <div className="cols">
        <div className="stack">
          <div className="card">
            <div className="item-art" style={{ aspectRatio: '16 / 7' }}>
              <EquipmentIcon category={model.category_slug} />
            </div>
            <div className="card-pad stack">
              <div className="row-tight">
                <span className="eyebrow">{model.manufacturer}</span>
                <span className="muted tiny mono">{model.sku}</span>
                {model.requires_license && <Pill tone="pill-warn">Operator ticket required</Pill>}
              </div>
              <h1>{model.name}</h1>
              <p className="dim" style={{ margin: 0 }}>{model.description}</p>
            </div>
          </div>

          {Object.keys(model.specs).length > 0 && (
            <div className="card">
              <div className="card-head"><h2>Specification</h2></div>
              <div className="card-pad">
                <dl className="spec-list">
                  {Object.entries(model.specs).map(([k, v]) => (
                    <div key={k}><dt>{k}</dt><dd>{v}</dd></div>
                  ))}
                </dl>
              </div>
            </div>
          )}

          <div className="card">
            <div className="card-head"><h2>Rates</h2></div>
            <div className="card-pad">
              <dl className="spec-list">
                <div><dt>Daily</dt><dd>{money(model.daily_rate_cents)}</dd></div>
                <div><dt>Weekly</dt><dd>{money(model.weekly_rate_cents)}</dd></div>
                <div><dt>Monthly</dt><dd>{money(model.monthly_rate_cents)}</dd></div>
                <div><dt>Refundable deposit</dt><dd>{money(model.deposit_cents)}</dd></div>
              </dl>
              <div className="note note-info" style={{ marginTop: 12 }}>
                We always charge whichever rate works out cheapest for your dates —
                you never pay more by hiring for longer.
              </div>
            </div>
          </div>
        </div>

        <div className="card" style={{ position: 'sticky', top: 76 }}>
          <div className="card-pad stack">
            <DateRange start={cart.startDate} end={cart.endDate} onChange={cart.setDates} compact />

            <div className="divider" />

            <div className="sum">
              <div className="sum-row">
                <span className="label">
                  {best.periods} × {best.basis} {plural(best.periods, 'rate')}
                </span>
                <span className="tabular">{money(best.rate)}</span>
              </div>
              <div className="sum-row">
                <span className="label">Quantity</span>
                <span>
                  <select value={quantity} onChange={e => setQuantity(Number(e.target.value))}
                    style={{ width: 72 }}>
                    {Array.from({ length: Math.max(model.available_units, 1) }, (_, i) => i + 1)
                      .slice(0, 10)
                      .map(n => <option key={n} value={n}>{n}</option>)}
                  </select>
                </span>
              </div>
              <div className="sum-row total">
                <span>Hire charge</span>
                <span className="tabular">{money(best.total * quantity)}</span>
              </div>
              <div className="sum-row">
                <span className="label">Deposit (refundable)</span>
                <span className="tabular">{money(model.deposit_cents * quantity)}</span>
              </div>
            </div>

            <div>
              {enough
                ? <Pill tone="pill-ok" dot>{model.available_units} available on these dates</Pill>
                : <Pill tone="pill-alert" dot>
                    Only {model.available_units} free — try different dates
                  </Pill>}
            </div>

            <button className="btn btn-block" disabled={!enough}
              onClick={() => {
                cart.add({
                  model_id: model.id, name: model.name, sku: model.sku,
                  category_slug: model.category_slug,
                }, quantity)
                navigate('/checkout')
              }}>
              Add to basket
            </button>
            <div className="tiny muted center">
              Nothing is charged until you confirm the booking.
            </div>
          </div>
        </div>
      </div>
    </Page>
  )
}

/**
 * Mirrors the backend's cheapest-basis rule so the price shown before adding to
 * the basket matches the quote that comes back afterwards.
 */
function bestRate(model: Model, days: number) {
  const options = [
    { basis: 'daily', rate: model.daily_rate_cents, periods: days },
    { basis: 'weekly', rate: model.weekly_rate_cents, periods: Math.ceil(days / 7) },
    { basis: 'monthly', rate: model.monthly_rate_cents, periods: Math.ceil(days / 30) },
  ].filter(o => o.rate > 0)
    .map(o => ({ ...o, total: o.rate * o.periods }))

  return options.length === 0
    ? { basis: 'daily', rate: 0, periods: days, total: 0 }
    : options.reduce((best, o) => (o.total < best.total ? o : best))
}
