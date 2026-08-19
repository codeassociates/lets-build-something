import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { AccountTabs } from '../../components/AccountTabs'
import { Empty, ErrorNote, SkeletonRows, StatusPill } from '../../components/ui'
import { api } from '../../api/client'
import type { Reservation } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { formatDate, money, plural, relativeDay } from '../../format'

export function MyRentals() {
  const { data, loading, error } = useApi(
    () => api.get<{ reservations: Reservation[] }>('/reservations'), [])

  const all = data?.reservations ?? []
  const active = all.filter(r => r.status === 'confirmed' || r.status === 'picked_up')
  const past = all.filter(r => r.status === 'returned' || r.status === 'cancelled')

  return (
    <Page>
      <PageHead title="My rentals" actions={<Link to="/" className="btn">Hire something</Link>} />
      <AccountTabs />
      <ErrorNote error={error} />

      {loading && !data && <div className="card"><SkeletonRows /></div>}

      {data && all.length === 0 && (
        <div className="card">
          <Empty title="No rentals yet">
            <Link to="/">Browse the catalog</Link> to book your first item.
          </Empty>
        </div>
      )}

      {active.length > 0 && (
        <section style={{ marginBottom: 24 }}>
          <h2 style={{ marginBottom: 10 }}>Current and upcoming</h2>
          <div className="stack">
            {active.map(r => <ReservationRow key={r.id} r={r} />)}
          </div>
        </section>
      )}

      {past.length > 0 && (
        <section>
          <h2 style={{ marginBottom: 10 }}>Past rentals</h2>
          <div className="card table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Reference</th><th>Dates</th><th>Equipment</th>
                  <th className="right">Total</th><th>Status</th>
                </tr>
              </thead>
              <tbody>
                {past.map(r => (
                  <tr key={r.id} className="clickable">
                    <td>
                      <Link to={`/account/reservations/${r.id}`} className="mono link-plain">
                        {r.reservation_number}
                      </Link>
                    </td>
                    <td className="small nowrap">
                      {formatDate(r.start_date)} – {formatDate(r.end_date)}
                    </td>
                    <td className="small dim">{summarise(r)}</td>
                    <td className="right tabular">{money(r.total_cents)}</td>
                    <td><StatusPill status={r.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </Page>
  )
}

function ReservationRow({ r }: { r: Reservation }) {
  return (
    <Link to={`/account/reservations/${r.id}`} className="card card-pad link-plain">
      <div className="spread">
        <div className="grow">
          <div className="row-tight" style={{ marginBottom: 4 }}>
            <span className="mono small dim">{r.reservation_number}</span>
            <StatusPill status={r.status} overdue={r.is_overdue} />
          </div>
          <div className="strong">{summarise(r)}</div>
          <div className="small dim" style={{ marginTop: 3 }}>
            {r.status === 'confirmed'
              ? <>Collect {formatDate(r.start_date)} ({relativeDay(r.start_date)}) · return by {formatDate(r.end_date)}</>
              : <>Out since {formatDate(r.start_date)} · due back {formatDate(r.end_date)} ({relativeDay(r.end_date)})</>}
          </div>
          {r.is_overdue && (
            <div className="small" style={{ color: 'var(--alert)', marginTop: 4 }}>
              {r.days_overdue} {plural(r.days_overdue, 'day')} overdue — late charges are accruing.
            </div>
          )}
        </div>
        <div className="right">
          <div className="tabular strong" style={{ fontSize: 17 }}>{money(r.total_cents)}</div>
          <div className="tiny muted">{r.rental_days} {plural(r.rental_days, 'day')} hire</div>
        </div>
      </div>
    </Link>
  )
}

function summarise(r: Reservation): string {
  if (r.items.length === 0) return '—'
  const first = `${r.items[0].quantity} × ${r.items[0].model_name}`
  return r.items.length === 1
    ? first
    : `${first} and ${r.items.length - 1} more ${plural(r.items.length - 1, 'item')}`
}
