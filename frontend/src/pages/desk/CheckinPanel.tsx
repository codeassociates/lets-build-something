import { useState } from 'react'
import { ErrorNote, MoneyInput } from '../../components/ui'
import { api } from '../../api/client'
import type { CheckinResult, Reservation } from '../../api/types'
import { useAction } from '../../hooks/useApi'
import { money, plural } from '../../format'

interface LineState {
  meter_in: string
  notes: string
  damage_cents: number
  needs_maintenance: boolean
}

/**
 * Taking the equipment back. Each unit is inspected on its own, because damage
 * and servicing are per-machine facts — and because the late fee, which is
 * calculated by the server from the return date, has to be explained to the
 * customer standing at the counter.
 */
export function CheckinPanel({ reservation, onDone }:
  { reservation: Reservation; onDone: (summary: string) => void }) {
  const open = reservation.items.flatMap(item =>
    item.assignments
      .filter(a => a.checked_in_at === null)
      .map(a => ({ ...a, model_name: item.model_name })))

  const [state, setState] = useState<Record<number, LineState>>(() =>
    Object.fromEntries(open.map(a => [a.id, {
      meter_in: a.meter_out !== null ? String(a.meter_out) : '',
      notes: '', damage_cents: 0, needs_maintenance: false,
    }])))
  const [notes, setNotes] = useState('')

  const set = (id: number, patch: Partial<LineState>) =>
    setState(s => ({ ...s, [id]: { ...s[id], ...patch } }))

  const checkin = useAction(async () => {
    const result = await api.post<CheckinResult>(
      `/desk/reservations/${reservation.id}/checkin`, {
        lines: open.map(a => ({
          assignment_id: a.id,
          meter_in: state[a.id].meter_in ? Number(state[a.id].meter_in) : null,
          notes: state[a.id].notes,
          damage_cents: state[a.id].damage_cents,
          needs_maintenance: state[a.id].needs_maintenance,
        })),
        notes,
      })
    onDone(describe(result))
  })

  const damageTotal = Object.values(state).reduce((n, l) => n + l.damage_cents, 0)

  return (
    <div className="card">
      <div className="card-head">
        <h2>Take the equipment back</h2>
        <span className="small dim">
          {open.length} {plural(open.length, 'unit')} still out
        </span>
      </div>

      <div className="card-pad stack">
        <ErrorNote error={checkin.error} />

        {reservation.is_overdue && (
          <div className="note note-warn">
            This rental is {reservation.days_overdue} {plural(reservation.days_overdue, 'day')} late.
            A late fee is added automatically at the daily rate plus 50% per item, taken from
            the deposit where possible.
          </div>
        )}

        {open.map(a => {
          const line = state[a.id]
          return (
            <div key={a.id} className="stack" style={{ gap: 9 }}>
              <div className="spread">
                <div>
                  <div className="strong">
                    <span className="mono">{a.asset_tag}</span> — {a.model_name}
                  </div>
                  <div className="tiny muted">
                    Out at meter {a.meter_out ?? '—'}
                  </div>
                </div>
                <label className="checkbox">
                  <input type="checkbox" checked={line.needs_maintenance}
                    onChange={e => set(a.id, { needs_maintenance: e.target.checked })} />
                  Send to maintenance
                </label>
              </div>

              <div className="form-grid">
                <div className="field">
                  <label>Meter reading in</label>
                  <input type="number" step="0.1" value={line.meter_in}
                    onChange={e => set(a.id, { meter_in: e.target.value })} />
                </div>
                <div className="field">
                  <label>Damage or cleaning charge</label>
                  <MoneyInput cents={line.damage_cents}
                    onChange={cents => set(a.id, { damage_cents: cents })} />
                </div>
                <div className="span-2 field">
                  <label>Condition notes</label>
                  <input value={line.notes} placeholder="Returned clean, no damage."
                    onChange={e => set(a.id, { notes: e.target.value })} />
                </div>
              </div>
              <div className="divider" />
            </div>
          )
        })}

        <div className="field">
          <label>Counter notes</label>
          <textarea value={notes} onChange={e => setNotes(e.target.value)} />
        </div>

        {damageTotal > 0 && (
          <div className="note note-warn">
            {money(damageTotal)} of damage and cleaning charges will be invoiced and taken
            from the deposit where it covers them.
          </div>
        )}

        <div>
          <button className="btn" disabled={checkin.busy || open.length === 0}
            onClick={() => void checkin.run()}>
            {checkin.busy ? 'Checking in…' : 'Check in and close the rental'}
          </button>
        </div>
      </div>
    </div>
  )
}

/** Turns the server's settlement into a sentence staff can read out. */
function describe(r: CheckinResult): string {
  const parts = ['Rental closed.']
  if (r.late_fee_cents > 0) {
    parts.push(`${money(r.late_fee_cents)} in late fees for ${r.days_overdue} ${plural(r.days_overdue, 'day')}.`)
  }
  if (r.damage_cents > 0) parts.push(`${money(r.damage_cents)} charged for damage.`)
  if (r.deposit_taken_cents > 0) {
    parts.push(`${money(r.deposit_taken_cents)} taken from the deposit.`)
  }
  if (r.deposit_released_cents > 0) {
    parts.push(`${money(r.deposit_released_cents)} of deposit released back to the customer.`)
  }
  if (r.late_fee_cents === 0 && r.damage_cents === 0) {
    parts.push('Nothing further owed.')
  }
  return parts.join(' ')
}
