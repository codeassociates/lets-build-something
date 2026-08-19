import { Field } from './ui'
import { money } from '../format'

export interface CardDetails {
  number: string
  month: string
  year: string
  cvc: string
  name: string
}

export const emptyCard: CardDetails = {
  number: '4242 4242 4242 4242', month: '11', year: '2030', cvc: '123', name: '',
}

/**
 * Card entry. Prefilled with a working test number because this system runs
 * against a stand-in payment gateway — the card details never leave the
 * deployment. A real integration would replace this with the processor's
 * hosted fields so the number never reaches our servers at all.
 */
export function CardForm({ card, onChange, deposit }:
  { card: CardDetails; onChange: (card: CardDetails) => void; deposit?: number }) {
  const set = (k: keyof CardDetails) => (e: React.ChangeEvent<HTMLInputElement>) =>
    onChange({ ...card, [k]: e.target.value })

  return (
    <div className="stack">
      <div className="form-grid">
        <div className="span-2">
          <Field label="Name on card">
            <input value={card.name} onChange={set('name')} autoComplete="cc-name" />
          </Field>
        </div>
        <div className="span-2">
          <Field label="Card number">
            <input value={card.number} onChange={set('number')} inputMode="numeric"
              autoComplete="cc-number" className="mono" />
          </Field>
        </div>
        <Field label="Expiry month">
          <input value={card.month} onChange={set('month')} inputMode="numeric" placeholder="MM" />
        </Field>
        <Field label="Expiry year">
          <input value={card.year} onChange={set('year')} inputMode="numeric" placeholder="YYYY" />
        </Field>
        <Field label="CVC">
          <input value={card.cvc} onChange={set('cvc')} inputMode="numeric" placeholder="123" />
        </Field>
      </div>

      {deposit !== undefined && deposit > 0 && (
        <div className="note note-info">
          {money(deposit)} of this is a refundable deposit — held, not taken, and released
          when the equipment is returned.
        </div>
      )}

      <div className="note">
        <strong>Test cards</strong>
        <div className="small" style={{ marginTop: 4 }}>
          <code className="mono">4242 4242 4242 4242</code> succeeds ·{' '}
          <code className="mono">4000 0000 0000 0002</code> is declined ·{' '}
          <code className="mono">4000 0000 0000 0069</code> is expired.
        </div>
      </div>
    </div>
  )
}
