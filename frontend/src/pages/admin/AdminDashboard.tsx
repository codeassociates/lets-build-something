import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { ErrorNote, SkeletonRows, Stat, StatusPill } from '../../components/ui'
import { api } from '../../api/client'
import type { Reservation, Stats } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { formatDate, money, moneyShort, plural } from '../../format'

export function AdminDashboard() {
  const stats = useApi(() => api.get<{ stats: Stats }>('/admin/stats'), [])
  const recent = useApi(
    () => api.get<{ reservations: Reservation[] }>('/reservations?limit=12'), [])

  const s = stats.data?.stats

  return (
    <Page>
      <PageHead title="Overview" subtitle="How the yard is doing this month." />
      <AdminTabs />
      <ErrorNote error={stats.error} />

      {stats.loading && !s && <div className="card"><SkeletonRows rows={3} height={70} /></div>}

      {s && (
        <>
          <div className="grid grid-stats" style={{ marginBottom: 16 }}>
            <Stat n={moneyShort(s.revenue_mtd_cents)} k="Taken this month" accent="ok" />
            <Stat n={moneyShort(s.outstanding_cents)} k="Outstanding"
              accent={s.outstanding_cents > 0 ? 'warn' : undefined} />
            <Stat n={s.reservations_mtd} k="Bookings this month" />
            <Stat n={s.customers} k="Active customers" />
          </div>

          <div className="grid grid-stats" style={{ marginBottom: 24 }}>
            <Stat n={s.active_rentals} k="Out on hire now" />
            <Stat n={s.upcoming_pickups} k="Upcoming collections" />
            <Stat n={s.overdue_rentals} k="Overdue"
              accent={s.overdue_rentals > 0 ? 'alert' : undefined} />
            <Stat n={s.units_available} k="Units on the yard" />
            <Stat n={s.units_out} k="Units out" />
            <Stat n={s.units_maintenance} k="In maintenance"
              accent={s.units_maintenance > 0 ? 'warn' : undefined} />
          </div>
        </>
      )}

      <div className="card">
        <div className="card-head">
          <h2>Recent bookings</h2>
          <Link to="/desk" className="btn btn-sm btn-secondary">Service desk</Link>
        </div>
        {recent.loading && !recent.data && <SkeletonRows />}
        {recent.data && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Reference</th><th>Customer</th><th>Dates</th>
                  <th className="right">Total</th><th>Status</th>
                </tr>
              </thead>
              <tbody>
                {recent.data.reservations.map(r => (
                  <tr key={r.id}>
                    <td>
                      <Link to={`/desk/reservations/${r.id}`} className="mono small link-plain">
                        {r.reservation_number}
                      </Link>
                    </td>
                    <td>
                      <div className="strong">{r.customer_name}</div>
                      <div className="tiny muted">
                        {r.items.length} {plural(r.items.length, 'line')}
                      </div>
                    </td>
                    <td className="small nowrap">
                      {formatDate(r.start_date)} – {formatDate(r.end_date)}
                    </td>
                    <td className="right tabular">{money(r.total_cents)}</td>
                    <td><StatusPill status={r.status} overdue={r.is_overdue} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Page>
  )
}
