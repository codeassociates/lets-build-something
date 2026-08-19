import type { ReactNode } from 'react'
import { ApiError } from '../api/client'
import { titleCase } from '../format'

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return <div className="empty"><div className="muted small">{label}</div></div>
}

export function SkeletonRows({ rows = 5, height = 44 }: { rows?: number; height?: number }) {
  return (
    <div className="stack" style={{ padding: 14, gap: 8 }}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="skeleton" style={{ height }} />
      ))}
    </div>
  )
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="empty">
      <h3>{title}</h3>
      {children && <div className="small">{children}</div>}
    </div>
  )
}

/**
 * Renders an API failure in the terms the user cares about. A 403 is not the
 * same thing as a broken server, and saying so avoids a support call.
 */
export function ErrorNote({ error }: { error: ApiError | null }) {
  if (!error) return null
  const tone = error.status === 403 || error.status === 401 ? 'note-warn' : 'note-alert'
  return (
    <div className={`note ${tone}`} role="alert">
      <strong>{headline(error)}</strong>
      {error.message && <div style={{ marginTop: 2 }}>{error.message}</div>}
      {Object.entries(error.fields).length > 0 && (
        <ul style={{ margin: '6px 0 0', paddingLeft: 18 }}>
          {Object.entries(error.fields).map(([field, message]) => (
            <li key={field}>{titleCase(field)}: {message}</li>
          ))}
        </ul>
      )}
    </div>
  )
}

function headline(error: ApiError): string {
  if (error.status === 0) return 'No connection'
  if (error.status === 401) return 'Please sign in'
  if (error.status === 403) return 'Not allowed'
  if (error.status === 404) return 'Not found'
  if (error.status === 409) return 'That could not be done'
  if (error.status >= 500) return 'Something went wrong'
  return 'Please check the form'
}

export function Pill({ tone = '', children, dot = false }: { tone?: string; children: ReactNode; dot?: boolean }) {
  return <span className={`pill ${tone} ${dot ? 'pill-dot' : ''}`}>{children}</span>
}

/** Maps a reservation status — plus the overdue flag — onto one badge. */
export function StatusPill({ status, overdue }: { status: string; overdue?: boolean }) {
  if (overdue && status === 'picked_up') return <Pill tone="pill-alert" dot>Overdue</Pill>
  switch (status) {
    case 'confirmed': return <Pill tone="pill-info" dot>Confirmed</Pill>
    case 'picked_up': return <Pill tone="pill-brand" dot>Out on hire</Pill>
    case 'returned':  return <Pill tone="pill-ok" dot>Returned</Pill>
    case 'cancelled': return <Pill tone="" dot>Cancelled</Pill>
    case 'paid':      return <Pill tone="pill-ok">Paid</Pill>
    case 'issued':    return <Pill tone="pill-warn">Unpaid</Pill>
    case 'void':      return <Pill tone="">Void</Pill>
    case 'available': return <Pill tone="pill-ok" dot>Available</Pill>
    case 'out':       return <Pill tone="pill-brand" dot>Out</Pill>
    case 'maintenance': return <Pill tone="pill-warn" dot>Maintenance</Pill>
    case 'retired':   return <Pill tone="" dot>Retired</Pill>
    case 'sent':      return <Pill tone="pill-ok">Sent</Pill>
    case 'failed':    return <Pill tone="pill-alert">Failed</Pill>
    case 'pending':   return <Pill tone="pill-info">Pending</Pill>
    case 'running':   return <Pill tone="pill-warn">Running</Pill>
    case 'done':      return <Pill tone="pill-ok">Done</Pill>
    default:          return <Pill>{titleCase(status)}</Pill>
  }
}

export function Stat({ n, k, accent }: { n: ReactNode; k: string; accent?: 'alert' | 'warn' | 'ok' }) {
  return (
    <div className={`card stat ${accent ? 'accent-' + accent : ''}`}>
      <div className="n tabular">{n}</div>
      <div className="k">{k}</div>
    </div>
  )
}

export function Modal({ title, onClose, children, wide, footer }:
  { title: string; onClose: () => void; children: ReactNode; wide?: boolean; footer?: ReactNode }) {
  return (
    <div className="modal-backdrop" onClick={onClose} role="dialog" aria-modal aria-label={title}>
      <div className={`modal ${wide ? 'wide' : ''}`} onClick={e => e.stopPropagation()}>
        <div className="card-head">
          <h2>{title}</h2>
          <button className="btn btn-ghost btn-sm" onClick={onClose} aria-label="Close">✕</button>
        </div>
        <div className="card-pad">{children}</div>
        {footer && <div className="card-foot">{footer}</div>}
      </div>
    </div>
  )
}

export function Field({ label, error, hint, children }:
  { label: string; error?: string; hint?: string; children: ReactNode }) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
      {hint && !error && <div className="field-hint">{hint}</div>}
      {error && <div className="field-error">{error}</div>}
    </div>
  )
}

/** A money input that speaks dollars to the user and cents to the API. */
export function MoneyInput({ cents, onChange, ...rest }:
  { cents: number; onChange: (cents: number) => void } & Omit<React.InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'value'>) {
  return (
    <input
      type="number" step="0.01" min="0"
      value={cents === 0 ? '' : (cents / 100).toFixed(2)}
      placeholder="0.00"
      onChange={e => onChange(Math.round(parseFloat(e.target.value || '0') * 100) || 0)}
      {...rest}
    />
  )
}
