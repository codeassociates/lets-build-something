import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Page } from '../components/Layout'
import { ErrorNote, Field } from '../components/ui'
import { useAction } from '../hooks/useApi'
import { useAuth, homeFor } from '../auth'

export function SignIn() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const submit = useAction(async () => {
    const user = await signIn(email, password)
    const from = (location.state as { from?: string } | null)?.from
    navigate(from ?? homeFor(user), { replace: true })
  })

  return (
    <Page narrow>
      <div style={{ maxWidth: 420, margin: '30px auto' }}>
        <h1 style={{ marginBottom: 6 }}>Sign in</h1>
        <p className="dim small" style={{ marginBottom: 18 }}>
          To book equipment, track your rentals and settle invoices.
        </p>

        <form className="card card-pad stack" onSubmit={e => { e.preventDefault(); void submit.run() }}>
          <ErrorNote error={submit.error} />
          <Field label="Email">
            <input type="email" value={email} autoComplete="username" autoFocus
              onChange={e => setEmail(e.target.value)} required />
          </Field>
          <Field label="Password">
            <input type="password" value={password} autoComplete="current-password"
              onChange={e => setPassword(e.target.value)} required />
          </Field>
          <button className="btn btn-block" disabled={submit.busy}>
            {submit.busy ? 'Signing in…' : 'Sign in'}
          </button>
          <div className="small center dim">
            No account yet? <Link to="/register">Create one</Link>
          </div>
        </form>

        <DemoAccounts onPick={(e, p) => { setEmail(e); setPassword(p) }} />
      </div>
    </Page>
  )
}

/**
 * Shown because this is a demonstration system with generated data. It would
 * not ship with a real deployment, where nobody's password belongs on a page.
 */
function DemoAccounts({ onPick }: { onPick: (email: string, password: string) => void }) {
  const accounts = [
    { role: 'Administrator', email: 'admin@kestrelrental.example', who: 'Alex Rutherford' },
    { role: 'Counter staff', email: 'marisol@kestrelrental.example', who: 'Marisol Vega' },
    { role: 'Customer', email: 'dana.whitfield@example.com', who: 'Dana Whitfield' },
  ]
  return (
    <div className="card card-pad stack" style={{ marginTop: 16 }}>
      <div className="eyebrow">Demonstration accounts</div>
      <div className="small dim" style={{ marginTop: -6 }}>
        Seeded data. Every account uses the password <code className="mono">rentals123</code>.
      </div>
      <div className="stack" style={{ gap: 6 }}>
        {accounts.map(a => (
          <button key={a.email} type="button" className="btn btn-secondary btn-sm"
            style={{ justifyContent: 'flex-start' }}
            onClick={() => onPick(a.email, 'rentals123')}>
            <span className="strong">{a.role}</span>
            <span className="muted tiny">{a.who} · {a.email}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
