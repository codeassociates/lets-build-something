import { useState } from 'react'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { EquipmentIcon } from '../../components/EquipmentIcon'
import { Empty, ErrorNote, Field, Modal, MoneyInput, Pill, SkeletonRows } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Category, Model } from '../../api/types'
import { useApi, useAction, useDebounced } from '../../hooks/useApi'
import { useAuth } from '../../auth'
import { money } from '../../format'

export function AdminItems() {
  const { can } = useAuth()
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('')
  const [editing, setEditing] = useState<Model | 'new' | null>(null)
  const query = useDebounced(search)

  const categories = useApi(() => api.get<{ categories: Category[] }>('/categories'), [])
  const models = useApi(
    () => api.get<{ models: Model[]; total: number }>('/admin/models' + qs({ q: query, category })),
    [query, category])

  return (
    <Page>
      <PageHead title="Catalog"
        subtitle="What the yard offers for hire, and what it costs."
        actions={can('admin') && (
          <button className="btn" onClick={() => setEditing('new')}>Add an item</button>
        )} />
      <AdminTabs />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <div className="filters">
          <div className="field grow">
            <label>Search</label>
            <input type="search" value={search} placeholder="Name, SKU or manufacturer…"
              onChange={e => setSearch(e.target.value)} />
          </div>
          <div className="field">
            <label>Category</label>
            <select value={category} onChange={e => setCategory(e.target.value)}>
              <option value="">All categories</option>
              {categories.data?.categories.map(c => (
                <option key={c.slug} value={c.slug}>{c.name}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <ErrorNote error={models.error} />

      <div className="card">
        {models.loading && !models.data && <SkeletonRows />}
        {models.data?.models.length === 0 && <Empty title="No items match" />}
        {models.data && models.data.models.length > 0 && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Item</th><th>Category</th>
                  <th className="right">Daily</th><th className="right">Weekly</th>
                  <th className="right">Monthly</th><th className="right">Deposit</th>
                  <th className="right">Fleet</th><th></th>
                </tr>
              </thead>
              <tbody>
                {models.data.models.map(m => (
                  <tr key={m.id}>
                    <td>
                      <div className="row-tight">
                        <span style={{ width: 30, height: 30, flexShrink: 0 }}>
                          <EquipmentIcon category={m.category_slug} />
                        </span>
                        <span>
                          <div className="strong">{m.name}</div>
                          <div className="tiny muted mono">{m.sku} · {m.manufacturer}</div>
                        </span>
                      </div>
                      <div className="row-tight" style={{ marginTop: 4 }}>
                        {!m.active && <Pill>Not listed</Pill>}
                        {m.requires_license && <Pill tone="pill-warn">Ticket</Pill>}
                      </div>
                    </td>
                    <td className="small dim">{m.category_name}</td>
                    <td className="right tabular">{money(m.daily_rate_cents)}</td>
                    <td className="right tabular">{money(m.weekly_rate_cents)}</td>
                    <td className="right tabular">{money(m.monthly_rate_cents)}</td>
                    <td className="right tabular dim">{money(m.deposit_cents)}</td>
                    <td className="right tabular">{m.total_units}</td>
                    <td className="right">
                      {can('admin') && (
                        <button className="btn btn-sm btn-secondary" onClick={() => setEditing(m)}>
                          Edit
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {editing && (
        <ItemDialog
          model={editing === 'new' ? null : editing}
          categories={categories.data?.categories ?? []}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); models.reload() }} />
      )}
    </Page>
  )
}

const blank = {
  category_id: 0, sku: '', name: '', description: '', manufacturer: '',
  daily_rate_cents: 0, weekly_rate_cents: 0, monthly_rate_cents: 0,
  deposit_cents: 0, replacement_value_cents: 0,
  requires_license: false, specs: {} as Record<string, string>, image_url: '', active: true,
}

function ItemDialog({ model, categories, onClose, onSaved }:
  { model: Model | null; categories: Category[]; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState(() => model
    ? {
        category_id: model.category_id, sku: model.sku, name: model.name,
        description: model.description, manufacturer: model.manufacturer,
        daily_rate_cents: model.daily_rate_cents,
        weekly_rate_cents: model.weekly_rate_cents,
        monthly_rate_cents: model.monthly_rate_cents,
        deposit_cents: model.deposit_cents,
        replacement_value_cents: model.replacement_value_cents,
        requires_license: model.requires_license, specs: model.specs,
        image_url: model.image_url, active: model.active,
      }
    : { ...blank, category_id: categories[0]?.id ?? 0 })

  const [specs, setSpecs] = useState<[string, string][]>(
    () => Object.entries(model?.specs ?? {}))

  const save = useAction(async () => {
    const payload = {
      ...form,
      specs: Object.fromEntries(specs.filter(([k]) => k.trim() !== '')),
    }
    if (model) await api.patch(`/admin/models/${model.id}`, payload)
    else await api.post('/admin/models', payload)
    onSaved()
  })

  const err = (f: string) => save.error?.fields?.[f]

  return (
    <Modal title={model ? `Edit ${model.name}` : 'Add an item'} onClose={onClose} wide
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button className="btn" disabled={save.busy} onClick={() => void save.run()}>
          {save.busy ? 'Saving…' : model ? 'Save changes' : 'Add item'}
        </button>
      </>}>
      <div className="stack">
        {save.error && Object.keys(save.error.fields ?? {}).length === 0 &&
          <ErrorNote error={save.error} />}

        <div className="form-grid">
          <div className="span-2">
            <Field label="Name" error={err('name')}>
              <input value={form.name} autoFocus
                onChange={e => setForm(f => ({ ...f, name: e.target.value }))} />
            </Field>
          </div>
          <Field label="SKU" error={err('sku')}>
            <input value={form.sku} className="mono"
              onChange={e => setForm(f => ({ ...f, sku: e.target.value }))} />
          </Field>
          <Field label="Manufacturer">
            <input value={form.manufacturer}
              onChange={e => setForm(f => ({ ...f, manufacturer: e.target.value }))} />
          </Field>
          <Field label="Category" error={err('category_id')}>
            <select value={form.category_id}
              onChange={e => setForm(f => ({ ...f, category_id: Number(e.target.value) }))}>
              {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </Field>
          <div className="span-2">
            <Field label="Description">
              <textarea value={form.description}
                onChange={e => setForm(f => ({ ...f, description: e.target.value }))} />
            </Field>
          </div>

          <Field label="Daily rate" error={err('daily_rate_cents')}>
            <MoneyInput cents={form.daily_rate_cents}
              onChange={c => setForm(f => ({ ...f, daily_rate_cents: c }))} />
          </Field>
          <Field label="Weekly rate">
            <MoneyInput cents={form.weekly_rate_cents}
              onChange={c => setForm(f => ({ ...f, weekly_rate_cents: c }))} />
          </Field>
          <Field label="Monthly rate">
            <MoneyInput cents={form.monthly_rate_cents}
              onChange={c => setForm(f => ({ ...f, monthly_rate_cents: c }))} />
          </Field>
          <Field label="Deposit">
            <MoneyInput cents={form.deposit_cents}
              onChange={c => setForm(f => ({ ...f, deposit_cents: c }))} />
          </Field>
          <Field label="Replacement value" hint="Used when equipment is lost or written off.">
            <MoneyInput cents={form.replacement_value_cents}
              onChange={c => setForm(f => ({ ...f, replacement_value_cents: c }))} />
          </Field>

          <div className="span-2 row" style={{ gap: 18 }}>
            <label className="checkbox">
              <input type="checkbox" checked={form.active}
                onChange={e => setForm(f => ({ ...f, active: e.target.checked }))} />
              Listed for hire
            </label>
            <label className="checkbox">
              <input type="checkbox" checked={form.requires_license}
                onChange={e => setForm(f => ({ ...f, requires_license: e.target.checked }))} />
              Requires an operator ticket
            </label>
          </div>
        </div>

        <div>
          <div className="eyebrow" style={{ marginBottom: 6 }}>Specification</div>
          <div className="stack" style={{ gap: 6 }}>
            {specs.map(([k, v], i) => (
              <div key={i} className="row" style={{ gap: 6, flexWrap: 'nowrap' }}>
                <input value={k} placeholder="Weight" style={{ flex: 1 }}
                  onChange={e => setSpecs(s => s.map((row, j) =>
                    j === i ? [e.target.value, row[1]] : row))} />
                <input value={v} placeholder="14 lb" style={{ flex: 1 }}
                  onChange={e => setSpecs(s => s.map((row, j) =>
                    j === i ? [row[0], e.target.value] : row))} />
                <button className="btn btn-ghost btn-sm"
                  onClick={() => setSpecs(s => s.filter((_, j) => j !== i))}>✕</button>
              </div>
            ))}
            <div>
              <button className="btn btn-secondary btn-sm"
                onClick={() => setSpecs(s => [...s, ['', '']])}>
                Add a specification
              </button>
            </div>
          </div>
        </div>
      </div>
    </Modal>
  )
}
