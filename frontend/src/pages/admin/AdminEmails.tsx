import { useState } from 'react'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { Empty, ErrorNote, Modal, SkeletonRows, StatusPill } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { SentEmail } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { formatDateTime, titleCase } from '../../format'

/**
 * Everything the system has sent. When a customer says they never got the
 * reminder, this is the page that answers the question.
 */
export function AdminEmails() {
  const [template, setTemplate] = useState('')
  const [status, setStatus] = useState('')
  const [viewing, setViewing] = useState<number | null>(null)

  const emails = useApi(
    () => api.get<{ emails: SentEmail[]; total: number }>(
      '/admin/emails' + qs({ template, status, limit: 100 })),
    [template, status])

  return (
    <Page>
      <PageHead title="Email log"
        subtitle="Every message the system has sent, and whether it was delivered." />
      <AdminTabs />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <div className="filters">
          <div className="field">
            <label>Type</label>
            <select value={template} onChange={e => setTemplate(e.target.value)}>
              <option value="">All messages</option>
              <option value="booking_confirmation">Booking confirmation</option>
              <option value="pickup_reminder">Pickup reminder</option>
              <option value="return_reminder">Return reminder</option>
              <option value="overdue_notice">Overdue notice</option>
              <option value="rental_receipt">Rental receipt</option>
            </select>
          </div>
          <div className="field">
            <label>Delivery</label>
            <select value={status} onChange={e => setStatus(e.target.value)}>
              <option value="">All</option>
              <option value="sent">Delivered</option>
              <option value="failed">Failed</option>
            </select>
          </div>
        </div>
      </div>

      <ErrorNote error={emails.error} />

      <div className="card">
        {emails.loading && !emails.data && <SkeletonRows />}
        {emails.data?.emails.length === 0 && <Empty title="No messages match" />}
        {emails.data && emails.data.emails.length > 0 && (
          <>
            <div className="card-head">
              <span className="small dim">
                {emails.data.total} messages · newest first
              </span>
            </div>
            <div className="table-wrap">
              <table className="data">
                <thead>
                  <tr>
                    <th>Sent</th><th>To</th><th>Subject</th><th>Type</th><th>Status</th><th></th>
                  </tr>
                </thead>
                <tbody>
                  {emails.data.emails.map(e => (
                    <tr key={e.id} className="clickable" onClick={() => setViewing(e.id)}>
                      <td className="small dim nowrap">{formatDateTime(e.created_at)}</td>
                      <td className="small">
                        <div>{e.to_name}</div>
                        <div className="tiny muted">{e.to_address}</div>
                      </td>
                      <td className="small">{e.subject}</td>
                      <td className="small dim nowrap">{titleCase(e.template)}</td>
                      <td>
                        <StatusPill status={e.status} />
                        {e.error && <div className="tiny" style={{ color: 'var(--alert)' }}>{e.error}</div>}
                      </td>
                      <td className="right">
                        <span className="btn btn-sm btn-secondary">View</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>

      {viewing !== null && <EmailDialog id={viewing} onClose={() => setViewing(null)} />}
    </Page>
  )
}

function EmailDialog({ id, onClose }: { id: number; onClose: () => void }) {
  const { data, loading, error } = useApi(
    () => api.get<{ email: SentEmail }>(`/admin/emails/${id}`), [id])
  const [tab, setTab] = useState<'html' | 'text'>('html')

  const email = data?.email

  return (
    <Modal title={email?.subject ?? 'Message'} onClose={onClose} wide
      footer={<button className="btn btn-secondary" onClick={onClose}>Close</button>}>
      <div className="stack">
        <ErrorNote error={error} />
        {loading && !email && <SkeletonRows rows={4} />}
        {email && (
          <>
            <dl className="spec-list">
              <div><dt>To</dt><dd>{email.to_name} &lt;{email.to_address}&gt;</dd></div>
              <div><dt>Sent</dt><dd>{formatDateTime(email.created_at)}</dd></div>
              <div><dt>Type</dt><dd>{titleCase(email.template)}</dd></div>
              <div><dt>Status</dt><dd><StatusPill status={email.status} /></dd></div>
            </dl>

            {email.error && <div className="note note-alert">{email.error}</div>}

            <div className="tabs" style={{ marginBottom: 0 }}>
              <button className={tab === 'html' ? 'active' : ''} onClick={() => setTab('html')}>
                Rendered
              </button>
              <button className={tab === 'text' ? 'active' : ''} onClick={() => setTab('text')}>
                Plain text
              </button>
            </div>

            {tab === 'html' ? (
              email.body_html ? (
                // Sandboxed: this is our own generated markup, but it is rendered
                // in an isolated frame so a message can never reach this page.
                <iframe title="Email preview" sandbox="" srcDoc={email.body_html}
                  style={{ width: '100%', height: 460, border: '1px solid var(--line)',
                    borderRadius: 'var(--radius-sm)', background: '#fff' }} />
              ) : (
                <div className="note">
                  No rendered copy was stored for this message.
                </div>
              )
            ) : (
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 13, margin: 0,
                background: 'var(--paper-2)', padding: 14, borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--line)', maxHeight: 460, overflow: 'auto' }}>
                {email.body_text || '(no plain-text copy stored)'}
              </pre>
            )}
          </>
        )}
      </div>
    </Modal>
  )
}
