import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { DateRange } from '../../components/DateRange'
import { EquipmentIcon } from '../../components/EquipmentIcon'
import { ErrorNote, Pill, Empty } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Category, Model } from '../../api/types'
import { useApi, useDebounced } from '../../hooks/useApi'
import { useCart } from '../../cart'
import { money, plural } from '../../format'

export function Catalog() {
  const cart = useCart()
  const [category, setCategory] = useState('')
  const [search, setSearch] = useState('')
  const query = useDebounced(search)

  const categories = useApi(() => api.get<{ categories: Category[] }>('/categories'), [])
  const models = useApi(
    () => api.get<{ models: Model[]; total: number }>(
      '/models' + qs({ category, q: query, start: cart.startDate, end: cart.endDate })),
    [category, query, cart.startDate, cart.endDate])

  return (
    <Page>
      <PageHead
        title="Hire equipment"
        subtitle="Choose your dates, then pick what you need. Availability updates as you change the window." />

      <div className="card card-pad" style={{ marginBottom: 20 }}>
        <div className="spread" style={{ alignItems: 'flex-end' }}>
          <DateRange start={cart.startDate} end={cart.endDate} onChange={cart.setDates} />
          <div className="field" style={{ minWidth: 240, flex: 1, maxWidth: 340 }}>
            <label htmlFor="cat-search">Search</label>
            <input id="cat-search" type="search" value={search} placeholder="Breaker, pump, sprayer…"
              onChange={e => setSearch(e.target.value)} />
          </div>
        </div>
      </div>

      <div className="tabs">
        <button className={category === '' ? 'active' : ''} onClick={() => setCategory('')}>
          Everything
        </button>
        {categories.data?.categories.map(c => (
          <button key={c.slug} className={category === c.slug ? 'active' : ''}
            onClick={() => setCategory(c.slug)}>
            {c.name}
          </button>
        ))}
      </div>

      <ErrorNote error={models.error} />

      {models.loading && !models.data && (
        <div className="grid grid-cards">
          {Array.from({ length: 8 }, (_, i) => (
            <div key={i} className="card"><div className="skeleton" style={{ height: 210 }} /></div>
          ))}
        </div>
      )}

      {models.data && models.data.models.length === 0 && (
        <div className="card">
          <Empty title="Nothing matches that">
            Try a different search, or clear the category filter.
          </Empty>
        </div>
      )}

      {models.data && models.data.models.length > 0 && (
        <>
          <div className="small dim" style={{ marginBottom: 12 }}>
            {models.data.total} {plural(models.data.total, 'item')} available to hire
          </div>
          <div className="grid grid-cards">
            {models.data.models.map(m => <ItemCard key={m.id} model={m} />)}
          </div>
        </>
      )}
    </Page>
  )
}

function ItemCard({ model }: { model: Model }) {
  const cart = useCart()
  const out = model.available_units <= 0

  return (
    <Link to={`/items/${model.id}`} className="card item-card">
      <div className="item-art">
        <EquipmentIcon category={model.category_slug} />
      </div>
      <div className="item-body">
        <div className="row-tight" style={{ gap: 6 }}>
          <span className="eyebrow">{model.manufacturer || model.category_name}</span>
          {model.requires_license && <Pill tone="pill-warn">Ticket required</Pill>}
        </div>
        <h3>{model.name}</h3>
        <div className="rate">
          {money(model.daily_rate_cents)} <span>/ day</span>
        </div>
        <div className="small dim">
          {money(model.weekly_rate_cents)} weekly · {money(model.deposit_cents)} deposit
        </div>
        <div style={{ marginTop: 'auto', paddingTop: 8 }}>
          {out
            ? <Pill tone="pill-alert" dot>None free on these dates</Pill>
            : <Pill tone="pill-ok" dot>
                {model.available_units} of {model.total_units} available
              </Pill>}
          {cart.has(model.id) && <Pill tone="pill-brand">In basket</Pill>}
        </div>
      </div>
    </Link>
  )
}
