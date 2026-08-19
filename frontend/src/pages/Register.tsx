import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Page } from '../components/Layout'
import { ErrorNote, Field } from '../components/ui'
import { useAction } from '../hooks/useApi'
import { api } from '../api/client'
import { useAuth } from '../auth'
import type { User } from '../api/types'

export function Register() {
  const { refresh } = useAuth()
  const navigate = useNavigate()
  const [form, setForm] = useState({
    full_name: '', email: '', password: '', phone: '', company: '',
    address_line1: '', city: '', state: 'MT', postal_code: '',
  })
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(f => ({ ...f, [k]: e.target.value }))

  const submit = useAction(async () => {
    await api.post<{ user: User }>('/auth/register', form)
    await refresh()
    navigate('/account', { replace: true })
  })
  const fieldError = (name: string) => submit.error?.fields?.[name]

  return (
    <Page narrow>
      <div style={{ maxWidth: 520, margin: '30px auto' }}>
        <h1 style={{ marginBottom: 6 }}>Create an account</h1>
        <p className="dim small" style={{ marginBottom: 18 }}>
          You only need an account to complete a booking — browsing is open to everyone.
        </p>

        <form className="card card-pad stack" onSubmit={e => { e.preventDefault(); void submit.run() }}>
          {submit.error && Object.keys(submit.error.fields ?? {}).length === 0 &&
            <ErrorNote error={submit.error} />}

          <div className="form-grid">
            <div className="span-2">
              <Field label="Full name" error={fieldError('full_name')}>
                <input value={form.full_name} onChange={set('full_name')} autoFocus required />
              </Field>
            </div>
            <div className="span-2">
              <Field label="Email" error={fieldError('email')}>
                <input type="email" value={form.email} onChange={set('email')}
                  autoComplete="username" required />
              </Field>
            </div>
            <div className="span-2">
              <Field label="Password" error={fieldError('password')}
                hint="At least 8 characters.">
                <input type="password" value={form.password} onChange={set('password')}
                  autoComplete="new-password" required />
              </Field>
            </div>
            <Field label="Phone">
              <input value={form.phone} onChange={set('phone')} />
            </Field>
            <Field label="Company" hint="Leave blank if hiring personally.">
              <input value={form.company} onChange={set('company')} />
            </Field>
            <div className="span-2">
              <Field label="Address">
                <input value={form.address_line1} onChange={set('address_line1')} />
              </Field>
            </div>
            <Field label="Town or city">
              <input value={form.city} onChange={set('city')} />
            </Field>
            <Field label="State">
              <input value={form.state} onChange={set('state')} />
            </Field>
            <Field label="ZIP">
              <input value={form.postal_code} onChange={set('postal_code')} />
            </Field>
          </div>

          <button className="btn btn-block" disabled={submit.busy}>
            {submit.busy ? 'Creating…' : 'Create account'}
          </button>
          <div className="small center dim">
            Already registered? <Link to="/signin">Sign in</Link>
          </div>
        </form>
      </div>
    </Page>
  )
}
