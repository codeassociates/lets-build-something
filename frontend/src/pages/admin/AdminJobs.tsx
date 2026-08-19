import { useState } from 'react'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { Empty, ErrorNote, SkeletonRows, StatusPill } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Job } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { formatDateTime, titleCase } from '../../format'

/**
 * The scheduled work behind the reminders. Mostly it should be empty and
 * boring; when a reminder does not arrive, a failed job here says why.
 */
export function AdminJobs() {
  const [status, setStatus] = useState('')

  const jobs = useApi(
    () => api.get<{ jobs: Job[] }>('/admin/jobs' + qs({ status, limit: 150 })), [status])

  const counts = (jobs.data?.jobs ?? []).reduce<Record<string, number>>((acc, j) => {
    acc[j.status] = (acc[j.status] ?? 0) + 1
    return acc
  }, {})

  return (
    <Page>
      <PageHead title="Scheduled jobs"
        subtitle="Reminder emails and the daily overdue sweep."
        actions={<button className="btn btn-secondary" onClick={jobs.reload}>Refresh</button>} />
      <AdminTabs />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <div className="filters">
          <div className="field">
            <label>Status</label>
            <select value={status} onChange={e => setStatus(e.target.value)}>
              <option value="">All</option>
              <option value="pending">Waiting to run</option>
              <option value="running">Running</option>
              <option value="done">Done</option>
              <option value="failed">Failed</option>
            </select>
          </div>
          <div className="row small dim" style={{ paddingBottom: 8 }}>
            {Object.entries(counts).map(([k, n]) => (
              <span key={k}>{titleCase(k)}: <strong>{n}</strong></span>
            ))}
          </div>
        </div>
      </div>

      <ErrorNote error={jobs.error} />

      <div className="card">
        {jobs.loading && !jobs.data && <SkeletonRows />}
        {jobs.data?.jobs.length === 0 && (
          <Empty title="Nothing scheduled">
            Jobs appear here when a booking is made or a rental falls overdue.
          </Empty>
        )}
        {jobs.data && jobs.data.jobs.length > 0 && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Job</th><th>Runs at</th><th>Status</th>
                  <th className="right">Attempts</th><th>Last error</th>
                </tr>
              </thead>
              <tbody>
                {jobs.data.jobs.map(j => (
                  <tr key={j.id}>
                    <td>
                      <div className="strong">{titleCase(j.kind)}</div>
                      {j.dedupe_key && <div className="tiny muted mono">{j.dedupe_key}</div>}
                    </td>
                    <td className="small nowrap">{formatDateTime(j.run_at)}</td>
                    <td><StatusPill status={j.status} /></td>
                    <td className="right tabular small">{j.attempts}</td>
                    <td className="small" style={{ color: j.last_error ? 'var(--alert)' : undefined,
                      maxWidth: 300 }}>
                      {j.last_error || '—'}
                    </td>
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
