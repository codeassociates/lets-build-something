import { NavLink, Link, useNavigate } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuth, homeFor } from '../auth'
import { useCart } from '../cart'
import { initials } from '../format'

export function Layout({ children }: { children: ReactNode }) {
  const { user, signOut, can } = useAuth()
  const cart = useCart()
  const navigate = useNavigate()

  return (
    <div className="app">
      <header className="masthead">
        <div className="masthead-inner">
          <Link to="/" className="wordmark">
            <Mark />
            <span>
              Kestrel
              <small>Equipment Rental</small>
            </span>
          </Link>

          <nav className="mast-nav">
            <NavLink to="/" end>Hire</NavLink>
            {user && <NavLink to="/account">My rentals</NavLink>}
            {can('staff') && <NavLink to="/desk">Service desk</NavLink>}
            {can('staff') && <NavLink to="/admin">Admin</NavLink>}
          </nav>

          <div className="mast-right">
            <Link to="/checkout" className="btn btn-sm btn-secondary" title="Your basket">
              Basket{cart.itemCount > 0 && <span className="badge-count">{cart.itemCount}</span>}
            </Link>
            {user ? (
              <>
                <Link to={homeFor(user)} className="wordmark" title={`${user.full_name} (${user.role})`}
                  style={{ gap: 7 }}>
                  <span aria-hidden style={avatarStyle}>{initials(user.full_name)}</span>
                </Link>
                <button className="btn btn-sm btn-ghost" style={{ color: 'rgba(255,255,255,.8)' }}
                  onClick={async () => { await signOut(); navigate('/') }}>
                  Sign out
                </button>
              </>
            ) : (
              <Link to="/signin" className="btn btn-sm btn-secondary">Sign in</Link>
            )}
          </div>
        </div>
      </header>

      {children}

      <footer className="footer">
        <div className="footer-inner">
          <span>Kestrel Equipment Rental · 1420 Gallatin Road, Bozeman MT</span>
          <span>Counter 7:00am–5:30pm · (406) 555-0134</span>
          <span className="muted" style={{ marginLeft: 'auto' }}>
            Demonstration system — no real payments are taken.
          </span>
        </div>
      </footer>
    </div>
  )
}

const avatarStyle: React.CSSProperties = {
  width: 28, height: 28, borderRadius: '50%',
  background: 'rgba(255,255,255,.18)', color: '#fff',
  display: 'grid', placeItems: 'center', fontSize: 11.5, fontWeight: 700,
}

function Mark() {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" aria-hidden>
      <path d="M4 19V7l7 5 7-8v15z" fill="none" stroke="currentColor" strokeWidth="1.8"
        strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  )
}

/** A page container. `narrow` suits forms and single records. */
export function Page({ children, narrow }: { children: ReactNode; narrow?: boolean }) {
  return <main className={`page ${narrow ? 'page-narrow' : ''}`}>{children}</main>
}

export function PageHead({ title, subtitle, actions }:
  { title: string; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="page-head">
      <div className="grow">
        <h1>{title}</h1>
        {subtitle && <p>{subtitle}</p>}
      </div>
      {actions && <div className="row">{actions}</div>}
    </div>
  )
}
