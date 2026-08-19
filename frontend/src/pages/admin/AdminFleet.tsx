import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { Empty, ErrorNote, Field, Modal, SkeletonRows, StatusPill } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Model, Unit } from '../../api/types'
import { useApi, useAction, useDebounced } from '../../hooks/useApi'
import { useAuth } from '../../auth'
import { formatDate } from '../../format'

/** The physical machines: what exists, where each one is, and its condition. */
export function AdminFleet() {
  const { can } = useAuth()
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [editing, setEditing] = useState<Unit | 'new' | null>(null)
  const query = useDebounced(search)

  const models = useApi(() => api.get<{ models: Model[] }>('/admin/models?limit=200'), [])
  const units = useApi(
    () => api.get<{ units: Unit[]; total: number }>('/admin/units' + qs({ q: query, status })),
    [query, status])

  return (
    <Page>
      <PageHead title="Fleet"
        subtitle="Every physical unit on the yard."
        actions={can('admin') && (
          <button className="btn" onClick={() => setEditing('new')}>Add a unit</button>
        )} />
      <AdminTabs />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <div className="filters">
          <div className="field grow">
            <label>Search</label>
            <input type="search" value={search} placeholder="Asset tag, serial number or item…"
              onChange={e => setSearch(e.target.value)} />
          </div>
          <div className="field">
            <label>Status</label>
            <select value={status} onChange={e => setStatus(e.target.value)}>
              <option value="">All</option>
              <option value="available">On the yard</option>
              <option value="out">Out on hire</option>
              <option value="maintenance">In maintenance</option>
              <option value="retired">Retired</option>
            </select>
          </div>
        </div>
      </div>

      <ErrorNote error={units.error} />

      <div className="card">
        {units.loading && !units.data && <SkeletonRows />}
        {units.data?.units.length === 0 && <Empty title="No units match" />}
        {units.data && units.data.units.length > 0 && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Asset tag</th><th>Item</th><th>Serial</th><th>Status</th>
                  <th className="right">Meter</th><th>Condition</th><th>Acquired</th><th></th>
                </tr>
              </thead>
              <tbody>
                {units.data.units.map(u => (
                  <tr key={u.id}>
                    <td className="mono strong">{u.asset_tag}</td>
                    <td className="small">{u.model_name}</td>
                    <td className="small mono muted">{u.serial_number || '—'}</td>
                    <td>
                      <StatusPill status={u.status} />
                      {u.reservation_number && (
                        <div className="tiny" style={{ marginTop: 3 }}>
                          <Link to={`/desk/reservations/${u.reservation_id}`} className="mono">
                            {u.reservation_number}
                          </Link>
                        </div>
                      )}
                    </td>
                    <td className="right tabular small">{u.meter_hours.toFixed(1)} h</td>
                    <td className="small dim" style={{ maxWidth: 220 }}>
                      {u.condition_notes || '—'}
                    </td>
                    <td className="small dim nowrap">{formatDate(u.acquired_on)}</td>
                    <td className="right">
                      <button className="btn btn-sm btn-secondary" onClick={() => setEditing(u)}>
                        Edit
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {editing && (
        <UnitDialog
          unit={editing === 'new' ? null : editing}
          models={models.data?.models ?? []}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); units.reload() }} />
      )}
    </Page>
  )
}

function UnitDialog({ unit, models, onClose, onSaved }:
  { unit: Unit | null; models: Model[]; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState(() => ({
    model_id: unit?.model_id ?? models[0]?.id ?? 0,
    asset_tag: unit?.asset_tag ?? '',
    serial_number: unit?.serial_number ?? '',
    status: unit?.status ?? 'available',
    condition_notes: unit?.condition_notes ?? '',
    meter_hours: unit?.meter_hours ?? 0,
    acquired_on: unit?.acquired_on ?? null,
  }))

  const save = useAction(async () => {
    const payload = {
      ...form,
      meter_hours: Number(form.meter_hours),
      acquired_on: form.acquired_on ? new Date(form.acquired_on).toISOString() : null,
    }
    if (unit) await api.patch(`/admin/units/${unit.id}`, payload)
    else await api.post('/admin/units', payload)
    onSaved()
  })

  const err = (f: string) => save.error?.fields?.[f]

  return (
    <Modal title={unit ? `Edit ${unit.asset_tag}` : 'Add a unit'} onClose={onClose}
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button className="btn" disabled={save.busy} onClick={() => void save.run()}>
          {save.busy ? 'Saving…' : unit ? 'Save changes' : 'Add unit'}
        </button>
      </>}>
      <div className="stack">
        {save.error && Object.keys(save.error.fields ?? {}).length === 0 &&
          <ErrorNote error={save.error} />}
        <div className="form-grid">
          <div className="span-2">
            <Field label="Item" error={err('model_id')}>
              <select value={form.model_id} disabled={unit !== null}
                onChange={e => setForm(f => ({ ...f, model_id: Number(e.target.value) }))}>
                {models.map(m => <option key={m.id} value={m.id}>{m.name} ({m.sku})</option>)}
              </select>
            </Field>
          </div>
          <Field label="Asset tag" error={err('asset_tag')}>
            <input value={form.asset_tag} className="mono" autoFocus
              onChange={e => setForm(f => ({ ...f, asset_tag: e.target.value }))} />
          </Field>
          <Field label="Serial number">
            <input value={form.serial_number} className="mono"
              onChange={e => setForm(f => ({ ...f, serial_number: e.target.value }))} />
          </Field>
          <Field label="Status" error={err('status')}
            hint={form.status === 'out' ? 'Set by checkout — change with care.' : undefined}>
            <select value={form.status}
              onChange={e => setForm(f => ({ ...f, status: e.target.value as Unit['status'] }))}>
              <option value="available">On the yard</option>
              <option value="out">Out on hire</option>
              <option value="maintenance">In maintenance</option>
              <option value="retired">Retired</option>
            </select>
          </Field>
          <Field label="Meter hours">
            <input type="number" step="0.1" value={form.meter_hours}
              onChange={e => setForm(f => ({ ...f, meter_hours: Number(e.target.value) }))} />
          </Field>
          <div className="span-2">
            <Field label="Condition notes">
              <textarea value={form.condition_notes}
                onChange={e => setForm(f => ({ ...f, condition_notes: e.target.value }))} />
            </Field>
          </div>
        </div>
      </div>
    </Modal>
  )
}
