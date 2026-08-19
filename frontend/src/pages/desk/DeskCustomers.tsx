import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { Empty, ErrorNote, Field, Modal, SkeletonRows } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { User } from '../../api/types'
import { useApi, useAction, useDebounced } from '../../hooks/useApi'
import { formatDate } from '../../format'

export function DeskCustomers() {
  const [search, setSearch] = useState('')
  const [adding, setAdding] = useState(false)
  const query = useDebounced(search)

  const { data, loading, error, reload } = useApi(
    () => api.get<{ customers: User[]; total: number }>('/customers' + qs({ q: query })), [query])

  return (
    <Page>
      <PageHead title="Customers"
        subtitle="Look someone up, or register a walk-in at the counter."
        actions={
          <>
            <Link to="/desk" className="btn btn-secondary">Back to desk</Link>
            <button className="btn" onClick={() => setAdding(true)}>Register a walk-in</button>
          </>
        } />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <Field label="Search">
          <input type="search" value={search} autoFocus
            placeholder="Name, email or company…"
            onChange={e => setSearch(e.target.value)} />
        </Field>
      </div>

      <ErrorNote error={error} />

      <div className="card">
        {loading && !data && <SkeletonRows />}
        {data && data.customers.length === 0 && <Empty title="No customers match that search" />}
        {data && data.customers.length > 0 && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Name</th><th>Company</th><th>Contact</th>
                  <th>Licence</th><th>Customer since</th>
                </tr>
              </thead>
              <tbody>
                {data.customers.map(c => (
                  <tr key={c.id}>
                    <td className="strong">{c.full_name}</td>
                    <td className="small dim">{c.company || '—'}</td>
                    <td className="small">
                      <div>{c.email}</div>
                      {c.phone && <div className="muted tiny">{c.phone}</div>}
                    </td>
                    <td className="small mono">{c.license_number || '—'}</td>
                    <td className="small dim nowrap">{formatDate(c.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {adding && <WalkInDialog onClose={() => setAdding(false)}
        onCreated={() => { setAdding(false); reload() }} />}
    </Page>
  )
}

function WalkInDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [form, setForm] = useState({
    full_name: '', email: '', phone: '', company: '',
    address_line1: '', city: '', state: 'MT', postal_code: '', license_number: '',
  })
  const [temporary, setTemporary] = useState<string | null>(null)
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(f => ({ ...f, [k]: e.target.value }))

  const create = useAction(async () => {
    const result = await api.post<{ customer: User; temporary_password: string }>('/customers', form)
    setTemporary(result.temporary_password)
  })

  if (temporary) {
    return (
      <Modal title="Customer registered" onClose={() => { onCreated(); onClose() }}
        footer={<button className="btn" onClick={() => { onCreated(); onClose() }}>Done</button>}>
        <div className="stack">
          <div className="note note-ok">
            <strong>{form.full_name} can now sign in.</strong>
            <div>Give them this temporary password — it is not shown again.</div>
          </div>
          <div className="card card-pad center">
            <div className="eyebrow">Temporary password</div>
            <div className="mono" style={{ fontSize: 21, fontWeight: 700, marginTop: 6 }}>
              {temporary}
            </div>
          </div>
          <div className="small dim">
            They should change it under “Your details” after signing in at{' '}
            <span className="mono">{form.email}</span>.
          </div>
        </div>
      </Modal>
    )
  }

  return (
    <Modal title="Register a walk-in customer" onClose={onClose} wide
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button className="btn" disabled={create.busy} onClick={() => void create.run()}>
          {create.busy ? 'Registering…' : 'Register customer'}
        </button>
      </>}>
      <div className="stack">
        <ErrorNote error={create.error} />
        <div className="form-grid">
          <Field label="Full name" error={create.error?.fields?.full_name}>
            <input value={form.full_name} onChange={set('full_name')} autoFocus />
          </Field>
          <Field label="Email" error={create.error?.fields?.email}>
            <input type="email" value={form.email} onChange={set('email')} />
          </Field>
          <Field label="Phone"><input value={form.phone} onChange={set('phone')} /></Field>
          <Field label="Company"><input value={form.company} onChange={set('company')} /></Field>
          <div className="span-2">
            <Field label="Address"><input value={form.address_line1} onChange={set('address_line1')} /></Field>
          </div>
          <Field label="Town or city"><input value={form.city} onChange={set('city')} /></Field>
          <Field label="State"><input value={form.state} onChange={set('state')} /></Field>
          <Field label="ZIP"><input value={form.postal_code} onChange={set('postal_code')} /></Field>
          <div className="span-2">
            <Field label="Driver's licence number"
              hint="Required before hiring anything that needs an operator ticket.">
              <input value={form.license_number} onChange={set('license_number')} />
            </Field>
          </div>
        </div>
      </div>
    </Modal>
  )
}
