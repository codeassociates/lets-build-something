import { useState } from 'react'
import { Page, PageHead } from '../../components/Layout'
import { AdminTabs } from '../../components/AdminTabs'
import { Empty, ErrorNote, Field, Modal, Pill, SkeletonRows } from '../../components/ui'
import { api, qs } from '../../api/client'
import type { Role, User } from '../../api/types'
import { useApi, useAction, useDebounced } from '../../hooks/useApi'
import { useAuth } from '../../auth'
import { formatDate, titleCase } from '../../format'

export function AdminUsers() {
  const { user: me } = useAuth()
  const [search, setSearch] = useState('')
  const [role, setRole] = useState('')
  const [editing, setEditing] = useState<User | 'new' | null>(null)
  const [resetting, setResetting] = useState<User | null>(null)
  const query = useDebounced(search)

  const users = useApi(
    () => api.get<{ users: User[]; total: number }>('/admin/users' + qs({ q: query, role })),
    [query, role])

  return (
    <Page>
      <PageHead title="People"
        subtitle="Customers, counter staff and administrators."
        actions={<button className="btn" onClick={() => setEditing('new')}>Add a person</button>} />
      <AdminTabs />

      <div className="card card-pad" style={{ marginBottom: 18 }}>
        <div className="filters">
          <div className="field grow">
            <label>Search</label>
            <input type="search" value={search} placeholder="Name, email or company…"
              onChange={e => setSearch(e.target.value)} />
          </div>
          <div className="field">
            <label>Role</label>
            <select value={role} onChange={e => setRole(e.target.value)}>
              <option value="">Everyone</option>
              <option value="customer">Customers</option>
              <option value="staff">Counter staff</option>
              <option value="admin">Administrators</option>
            </select>
          </div>
        </div>
      </div>

      <ErrorNote error={users.error} />

      <div className="card">
        {users.loading && !users.data && <SkeletonRows />}
        {users.data?.users.length === 0 && <Empty title="Nobody matches that search" />}
        {users.data && users.data.users.length > 0 && (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>Name</th><th>Email</th><th>Role</th><th>Company</th>
                  <th>Joined</th><th></th>
                </tr>
              </thead>
              <tbody>
                {users.data.users.map(u => (
                  <tr key={u.id}>
                    <td>
                      <div className="strong">
                        {u.full_name}
                        {u.id === me?.id && <span className="muted tiny"> (you)</span>}
                      </div>
                      {u.phone && <div className="tiny muted">{u.phone}</div>}
                    </td>
                    <td className="small">{u.email}</td>
                    <td>
                      <Pill tone={u.role === 'admin' ? 'pill-brand'
                        : u.role === 'staff' ? 'pill-info' : ''}>
                        {titleCase(u.role)}
                      </Pill>
                      {!u.active && <Pill tone="pill-alert">Deactivated</Pill>}
                    </td>
                    <td className="small dim">{u.company || '—'}</td>
                    <td className="small dim nowrap">{formatDate(u.created_at)}</td>
                    <td className="right nowrap">
                      <button className="btn btn-sm btn-ghost" onClick={() => setResetting(u)}>
                        Reset password
                      </button>
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
        <UserDialog user={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); users.reload() }} />
      )}
      {resetting && (
        <ResetDialog user={resetting} onClose={() => setResetting(null)} />
      )}
    </Page>
  )
}

function UserDialog({ user, onClose, onSaved }:
  { user: User | null; onClose: () => void; onSaved: () => void }) {
  const { user: me } = useAuth()
  const [form, setForm] = useState(() => ({
    full_name: user?.full_name ?? '',
    email: user?.email ?? '',
    role: (user?.role ?? 'customer') as Role,
    phone: user?.phone ?? '',
    company: user?.company ?? '',
    active: user?.active ?? true,
  }))
  const [temporary, setTemporary] = useState<string | null>(null)

  const save = useAction(async () => {
    if (user) {
      await api.patch(`/admin/users/${user.id}`, {
        full_name: form.full_name, phone: form.phone, company: form.company,
        role: form.role, active: form.active,
      })
      onSaved()
    } else {
      const result = await api.post<{ user: User; temporary_password?: string }>(
        '/admin/users', {
          email: form.email, full_name: form.full_name, role: form.role,
          phone: form.phone, company: form.company, password: '',
        })
      if (result.temporary_password) setTemporary(result.temporary_password)
      else onSaved()
    }
  })

  const isSelf = user?.id === me?.id
  const err = (f: string) => save.error?.fields?.[f]

  if (temporary) {
    return (
      <Modal title="Account created" onClose={onSaved}
        footer={<button className="btn" onClick={onSaved}>Done</button>}>
        <div className="stack">
          <div className="note note-ok">
            <strong>{form.full_name} can now sign in as {form.email}.</strong>
            <div>Pass on this temporary password — it will not be shown again.</div>
          </div>
          <div className="card card-pad center">
            <div className="eyebrow">Temporary password</div>
            <div className="mono" style={{ fontSize: 21, fontWeight: 700, marginTop: 6 }}>
              {temporary}
            </div>
          </div>
        </div>
      </Modal>
    )
  }

  return (
    <Modal title={user ? `Edit ${user.full_name}` : 'Add a person'} onClose={onClose}
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button className="btn" disabled={save.busy} onClick={() => void save.run()}>
          {save.busy ? 'Saving…' : user ? 'Save changes' : 'Create account'}
        </button>
      </>}>
      <div className="stack">
        {save.error && Object.keys(save.error.fields ?? {}).length === 0 &&
          <ErrorNote error={save.error} />}

        {isSelf && (
          <div className="note note-warn">
            This is your own account. You cannot remove your own access or deactivate yourself.
          </div>
        )}

        <div className="form-grid">
          <div className="span-2">
            <Field label="Full name" error={err('full_name')}>
              <input value={form.full_name} autoFocus
                onChange={e => setForm(f => ({ ...f, full_name: e.target.value }))} />
            </Field>
          </div>
          <div className="span-2">
            <Field label="Email" error={err('email')}
              hint={user ? 'The sign-in address cannot be changed here.' : undefined}>
              <input type="email" value={form.email} disabled={user !== null}
                onChange={e => setForm(f => ({ ...f, email: e.target.value }))} />
            </Field>
          </div>
          <Field label="Role" error={err('role')}>
            <select value={form.role} disabled={isSelf}
              onChange={e => setForm(f => ({ ...f, role: e.target.value as Role }))}>
              <option value="customer">Customer</option>
              <option value="staff">Counter staff</option>
              <option value="admin">Administrator</option>
            </select>
          </Field>
          <Field label="Phone">
            <input value={form.phone}
              onChange={e => setForm(f => ({ ...f, phone: e.target.value }))} />
          </Field>
          <div className="span-2">
            <Field label="Company">
              <input value={form.company}
                onChange={e => setForm(f => ({ ...f, company: e.target.value }))} />
            </Field>
          </div>
          {user && (
            <div className="span-2">
              <label className="checkbox">
                <input type="checkbox" checked={form.active} disabled={isSelf}
                  onChange={e => setForm(f => ({ ...f, active: e.target.checked }))} />
                Account is active — an inactive account cannot sign in
              </label>
            </div>
          )}
        </div>

        {!user && (
          <div className="note note-info">
            A temporary password is generated and shown once the account is created.
          </div>
        )}
      </div>
    </Modal>
  )
}

function ResetDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const [temporary, setTemporary] = useState<string | null>(null)

  const reset = useAction(async () => {
    const result = await api.post<{ temporary_password?: string }>(
      `/admin/users/${user.id}/password`, {})
    setTemporary(result.temporary_password ?? '(set)')
  })

  return (
    <Modal title={`Reset password for ${user.full_name}`} onClose={onClose}
      footer={temporary
        ? <button className="btn" onClick={onClose}>Done</button>
        : <>
            <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
            <button className="btn btn-danger" disabled={reset.busy} onClick={() => void reset.run()}>
              {reset.busy ? 'Resetting…' : 'Reset password'}
            </button>
          </>}>
      <div className="stack">
        <ErrorNote error={reset.error} />
        {temporary ? (
          <>
            <div className="note note-ok">
              Password reset. All of their existing sessions have been signed out.
            </div>
            <div className="card card-pad center">
              <div className="eyebrow">Temporary password</div>
              <div className="mono" style={{ fontSize: 21, fontWeight: 700, marginTop: 6 }}>
                {temporary}
              </div>
            </div>
          </>
        ) : (
          <p className="small dim">
            A new temporary password will be generated and shown once. Signing them out
            everywhere is part of the reset.
          </p>
        )}
      </div>
    </Modal>
  )
}
