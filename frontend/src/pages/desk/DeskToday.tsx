import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { Empty, ErrorNote, SkeletonRows, Stat, StatusPill } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { DeskSummary, Reservation } from '../../api/types'
import { useApi, useDebounced } from '../../hooks/useApi'
import { formatDate, money, plural, todayISO } from '../../format'

export function DeskToday() {
  const [date, setDate] = useState(todayISO())
  const [search, setSearch] = useState('')
  const query = useDebounced(search)

  const desk = useApi(
    () => api.get<{ desk: DeskSummary }>('/desk/summary' + qs({ date })), [date])

  const results = useApi(async () => {
    if (query.trim().length < 2) return null
    return api.get<{ reservations: Reservation[] }>('/desk/lookup' + qs({ q: query }))
  }, [query])

  const d = desk.data?.desk

  return (
    <Page>
      <PageHead
        title="Service desk"
        subtitle="Everything happening at the counter today."
        actions={
          <>
            <Link to="/desk/customers" className="btn btn-secondary">Customers</Link>
            <div className="field">
              <input type="date" value={date} onChange={e => setDate(e.target.value)} />
            </div>
          </>
        } />

      <div className="card card-pad" style={{ marginBottom: 20 }}>
        <div className="field">
          <label htmlFor="desk-search">Find a booking</label>
          <input id="desk-search" type="search" value={search} autoFocus
            placeholder="Reservation number, customer name, email or company…"
            onChange={e => setSearch(e.target.value)} />
        </div>
        {results.data && (
          <div style={{ marginTop: 12 }}>
            {results.data.reservations.length === 0
              ? <div className="small muted">Nothing matches “{query}”.</div>
              : <ReservationTable rows={results.data.reservations} />}
          </div>
        )}
      </div>

      <ErrorNote error={desk.error} />

      {desk.loading && !desk.data && <div className="card"><SkeletonRows /></div>}

      {d && (
        <>
          <div className="grid grid-stats" style={{ marginBottom: 22 }}>
            <Stat n={d.pickups_due_count} k="Collections due" />
            <Stat n={d.returns_due_count} k="Returns due" />
            <Stat n={d.overdue_count} k="Overdue" accent={d.overdue_count > 0 ? 'alert' : undefined} />
            <Stat n={d.out_now} k="Units out on hire" />
          </div>

          {d.overdue.length > 0 && (
            <Section title="Overdue" tone="alert"
              subtitle="Chase these — late charges are accruing.">
              <ReservationTable rows={d.overdue} showOverdue />
            </Section>
          )}

          <Section title={`Collections due ${formatDate(d.date)}`}
            subtitle="Confirmed bookings waiting to be handed over.">
            {d.pickups_due.length === 0
              ? <Empty title="No collections booked for this day" />
              : <ReservationTable rows={d.pickups_due} action="Check out" />}
          </Section>

          <Section title={`Returns due ${formatDate(d.date)}`}
            subtitle="Equipment expected back at the yard.">
            {d.returns_due.length === 0
              ? <Empty title="No returns expected for this day" />
              : <ReservationTable rows={d.returns_due} action="Check in" />}
          </Section>
        </>
      )}
    </Page>
  )
}

function Section({ title, subtitle, tone, children }:
  { title: string; subtitle?: string; tone?: string; children: React.ReactNode }) {
  return (
    <section style={{ marginBottom: 22 }}>
      <div className="card">
        <div className="card-head">
          <div className="grow">
            <h2 style={{ color: tone === 'alert' ? 'var(--alert)' : undefined }}>{title}</h2>
            {subtitle && <div className="small dim" style={{ marginTop: 2 }}>{subtitle}</div>}
          </div>
        </div>
        {children}
      </div>
    </section>
  )
}

function ReservationTable({ rows, action, showOverdue }:
  { rows: Reservation[]; action?: string; showOverdue?: boolean }) {
  const navigate = useNavigate()
  return (
    <div className="table-wrap">
      <table className="data">
        <thead>
          <tr>
            <th>Reference</th><th>Customer</th><th>Equipment</th>
            <th>Dates</th><th className="right">Total</th><th>Status</th><th></th>
          </tr>
        </thead>
        <tbody>
          {rows.map(r => (
            <tr key={r.id} className="clickable"
              onClick={() => navigate(`/desk/reservations/${r.id}`)}>
              <td className="mono small nowrap">{r.reservation_number}</td>
              <td>
                <div className="strong">{r.customer_name}</div>
                <div className="tiny muted">{r.customer_phone || r.customer_email}</div>
              </td>
              <td className="small dim">
                {r.items.map(i => `${i.quantity} × ${i.model_name}`).join(', ') || '—'}
              </td>
              <td className="small nowrap">
                {formatDate(r.start_date)} – {formatDate(r.end_date)}
                {showOverdue && (
                  <div className="tiny" style={{ color: 'var(--alert)' }}>
                    {r.days_overdue} {plural(r.days_overdue, 'day')} late
                  </div>
                )}
              </td>
              <td className="right tabular">{money(r.total_cents)}</td>
              <td><StatusPill status={r.status} overdue={r.is_overdue} /></td>
              <td className="right">
                {action && <span className="btn btn-sm btn-secondary">{action}</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
