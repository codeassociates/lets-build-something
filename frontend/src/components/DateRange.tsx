import { rentalDays, todayISO, plural } from '../format'

/**
 * The hire window. It appears on the catalog, on an item, and in the basket,
 * always driving the same two values, so availability and price on screen
 * always refer to the dates the customer can see.
 */
export function DateRange({ start, end, onChange, compact }:
  { start: string; end: string; onChange: (start: string, end: string) => void; compact?: boolean }) {
  const days = rentalDays(start, end)
  return (
    <div className="row" style={{ gap: 10, alignItems: 'flex-end' }}>
      <div className="field" style={{ minWidth: compact ? 130 : 150 }}>
        <label htmlFor="hire-from">Collect</label>
        <input id="hire-from" type="date" value={start} min={todayISO()}
          onChange={e => onChange(e.target.value, end)} />
      </div>
      <div className="field" style={{ minWidth: compact ? 130 : 150 }}>
        <label htmlFor="hire-to">Return</label>
        <input id="hire-to" type="date" value={end} min={start}
          onChange={e => onChange(start, e.target.value)} />
      </div>
      <div className="pill pill-brand" style={{ marginBottom: 7 }}>
        {days} {plural(days, 'day')}
      </div>
    </div>
  )
}
