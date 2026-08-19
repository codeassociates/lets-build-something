import { useState } from 'react'
import { Page, PageHead } from '../../components/Layout'
import { AccountTabs } from '../../components/AccountTabs'
import { ErrorNote, Field } from '../../components/ui'
import { api } from '../../api/client'
import type { User } from '../../api/types'
import { useAction } from '../../hooks/useApi'
import { useAuth } from '../../auth'

export function Profile() {
  const { user, refresh } = useAuth()
  const [saved, setSaved] = useState(false)
  const [form, setForm] = useState(() => ({
    full_name: user?.full_name ?? '',
    phone: user?.phone ?? '',
    company: user?.company ?? '',
    address_line1: user?.address_line1 ?? '',
    address_line2: user?.address_line2 ?? '',
    city: user?.city ?? '',
    state: user?.state ?? '',
    postal_code: user?.postal_code ?? '',
    license_number: user?.license_number ?? '',
  }))
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setSaved(false)
    setForm(f => ({ ...f, [k]: e.target.value }))
  }

  const save = useAction(async () => {
    await api.patch<{ user: User }>('/auth/me', form)
    await refresh()
    setSaved(true)
  })

  return (
    <Page narrow>
      <PageHead title="Your details" />
      <AccountTabs />

      <form className="card card-pad stack"
        onSubmit={e => { e.preventDefault(); void save.run() }}>
        <ErrorNote error={save.error} />
        {saved && <div className="note note-ok">Your details have been saved.</div>}

        <div className="form-grid">
          <div className="span-2">
            <Field label="Full name"><input value={form.full_name} onChange={set('full_name')} /></Field>
          </div>
          <Field label="Phone"><input value={form.phone} onChange={set('phone')} /></Field>
          <Field label="Company"><input value={form.company} onChange={set('company')} /></Field>
          <div className="span-2">
            <Field label="Address"><input value={form.address_line1} onChange={set('address_line1')} /></Field>
          </div>
          <div className="span-2">
            <Field label="Address line 2">
              <input value={form.address_line2} onChange={set('address_line2')} />
            </Field>
          </div>
          <Field label="Town or city"><input value={form.city} onChange={set('city')} /></Field>
          <Field label="State"><input value={form.state} onChange={set('state')} /></Field>
          <Field label="ZIP"><input value={form.postal_code} onChange={set('postal_code')} /></Field>
          <div className="span-2">
            <Field label="Driver's licence number"
              hint="Needed for equipment that requires an operator ticket.">
              <input value={form.license_number} onChange={set('license_number')} />
            </Field>
          </div>
        </div>

        <div className="row">
          <button className="btn" disabled={save.busy}>
            {save.busy ? 'Saving…' : 'Save details'}
          </button>
          <span className="small muted">
            Signed in as {user?.email} ({user?.role})
          </span>
        </div>
      </form>

      <PasswordCard />
    </Page>
  )
}

function PasswordCard() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [done, setDone] = useState(false)

  const change = useAction(async () => {
    await api.post('/auth/password', { current_password: current, new_password: next })
    setCurrent(''); setNext(''); setDone(true)
  })

  return (
    <form className="card card-pad stack" style={{ marginTop: 16 }}
      onSubmit={e => { e.preventDefault(); void change.run() }}>
      <h2>Change password</h2>
      <ErrorNote error={change.error} />
      {done && <div className="note note-ok">Your password has been changed.</div>}
      <div className="form-grid">
        <Field label="Current password" error={change.error?.fields?.current_password}>
          <input type="password" value={current} autoComplete="current-password"
            onChange={e => { setDone(false); setCurrent(e.target.value) }} />
        </Field>
        <Field label="New password" error={change.error?.fields?.new_password}
          hint="At least 8 characters.">
          <input type="password" value={next} autoComplete="new-password"
            onChange={e => { setDone(false); setNext(e.target.value) }} />
        </Field>
      </div>
      <div>
        <button className="btn btn-secondary" disabled={change.busy || !current || !next}>
          {change.busy ? 'Changing…' : 'Change password'}
        </button>
      </div>
    </form>
  )
}
