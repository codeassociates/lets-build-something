import { useEffect, useState } from 'react'
import { ErrorNote, Spinner } from '../../components/ui'
import { api } from '../../api/client'
import type { Reservation, Unit } from '../../api/types'
import { useApi, useAction } from '../../hooks/useApi'
import { plural } from '../../format'

interface LineUnits {
  item_id: number
  model_id: number
  model_name: string
  quantity: number
  units: Unit[]
}

/**
 * The handover. Staff pick which physical machines go out against each line —
 * the yard needs to know exactly which asset left, not just that "a breaker"
 * did, so it can be tracked, metered and charged for if it comes back broken.
 */
export function CheckoutPanel({ reservation, onDone }:
  { reservation: Reservation; onDone: () => void }) {
  const [chosen, setChosen] = useState<Record<number, number[]>>({})
  const [meters, setMeters] = useState<Record<number, string>>({})
  const [notes, setNotes] = useState('')

  const available = useApi(
    () => api.get<{ lines: LineUnits[] }>(`/desk/reservations/${reservation.id}/available-units`),
    [reservation.id])

  // Preselect the first free units for each line: the common case is "whatever
  // is on the shelf", and staff can swap any of them before confirming.
  useEffect(() => {
    if (!available.data) return
    const preset: Record<number, number[]> = {}
    for (const line of available.data.lines) {
      preset[line.item_id] = line.units.slice(0, line.quantity).map(u => u.id)
    }
    setChosen(preset)
  }, [available.data])

  const checkout = useAction(async () => {
    await api.post(`/desk/reservations/${reservation.id}/checkout`, {
      lines: (available.data?.lines ?? []).map(line => ({
        item_id: line.item_id,
        unit_ids: chosen[line.item_id] ?? [],
        meter_out: meters[line.item_id] ? Number(meters[line.item_id]) : null,
      })),
      notes,
    })
    onDone()
  })

  if (available.loading && !available.data) {
    return <div className="card"><Spinner label="Checking the yard…" /></div>
  }

  const lines = available.data?.lines ?? []
  const ready = lines.every(l => (chosen[l.item_id]?.length ?? 0) === l.quantity)

  return (
    <div className="card">
      <div className="card-head">
        <h2>Hand over the equipment</h2>
        <span className="small dim">Choose which units are going out</span>
      </div>

      <div className="card-pad stack">
        <ErrorNote error={available.error ?? checkout.error} />

        {lines.map(line => {
          const picked = chosen[line.item_id] ?? []
          const short = line.units.length < line.quantity
          return (
            <div key={line.item_id} className="stack" style={{ gap: 8 }}>
              <div className="spread">
                <div>
                  <div className="strong">{line.model_name}</div>
                  <div className="small dim">
                    Needs {line.quantity} {plural(line.quantity, 'unit')} ·
                    {' '}{picked.length} selected
                  </div>
                </div>
                {line.units.length > 0 && (
                  <div className="field" style={{ maxWidth: 150 }}>
                    <label>Meter reading out</label>
                    <input type="number" step="0.1" placeholder="hours"
                      value={meters[line.item_id] ?? ''}
                      onChange={e => setMeters(m => ({ ...m, [line.item_id]: e.target.value }))} />
                  </div>
                )}
              </div>

              {short && (
                <div className="note note-alert">
                  Only {line.units.length} {plural(line.units.length, 'unit')} free on the yard,
                  but {line.quantity} were booked. Check whether something is due back, or
                  release a unit from maintenance.
                </div>
              )}

              <div className="row" style={{ gap: 7 }}>
                {line.units.map(unit => {
                  const isPicked = picked.includes(unit.id)
                  const full = picked.length >= line.quantity && !isPicked
                  return (
                    <button key={unit.id} type="button"
                      className={`btn btn-sm ${isPicked ? '' : 'btn-secondary'}`}
                      disabled={full}
                      title={unit.condition_notes || undefined}
                      onClick={() => setChosen(c => ({
                        ...c,
                        [line.item_id]: isPicked
                          ? picked.filter(id => id !== unit.id)
                          : [...picked, unit.id],
                      }))}>
                      <span className="mono">{unit.asset_tag}</span>
                      <span className="tiny" style={{ opacity: .7 }}>
                        {Math.round(unit.meter_hours)}h
                      </span>
                    </button>
                  )
                })}
              </div>
              <div className="divider" />
            </div>
          )
        })}

        <div className="field">
          <label>Counter notes</label>
          <textarea value={notes} onChange={e => setNotes(e.target.value)}
            placeholder="ID checked, accessories included, condition noted…" />
        </div>

        <div className="row">
          <button className="btn" disabled={!ready || checkout.busy}
            onClick={() => void checkout.run()}>
            {checkout.busy ? 'Checking out…' : 'Check out and hand over'}
          </button>
          {!ready && (
            <span className="small muted">
              Select the full quantity for every line first.
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
