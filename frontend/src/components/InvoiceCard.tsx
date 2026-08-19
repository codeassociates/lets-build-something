import { useState } from 'react'
import { ErrorNote, Modal, StatusPill } from './ui'
import { CardForm, type CardDetails, emptyCard } from './CardForm'
import { api } from '../api/client'
import type { Invoice } from '../api/types'
import { useAction } from '../hooks/useApi'
import { formatDate, formatDateTime, money, titleCase } from '../format'

/**
 * One invoice, with its lines, its payment history, and a way to settle it.
 * Used on the customer's reservation page and on the desk's, so both sides of
 * the counter are looking at exactly the same document.
 */
export function InvoiceCard({ invoice, onPaid }: { invoice: Invoice; onPaid: () => void }) {
  const [paying, setPaying] = useState(false)
  const balance = invoice.total_cents - invoice.amount_paid_cents
  const payable = invoice.status !== 'void' && balance > 0

  return (
    <div className="card">
      <div className="card-head">
        <h3>
          Invoice <span className="mono">{invoice.invoice_number}</span>
        </h3>
        <StatusPill status={invoice.status} />
        {payable && (
          <button className="btn btn-sm" onClick={() => setPaying(true)}>
            Pay {money(balance)}
          </button>
        )}
      </div>

      <div className="table-wrap">
        <table className="data">
          <thead>
            <tr><th>Description</th><th className="right">Amount</th></tr>
          </thead>
          <tbody>
            {invoice.lines.map(line => (
              <tr key={line.id}>
                <td>
                  <div>{line.description}</div>
                  {line.kind !== 'rental' && (
                    <div className="tiny muted">{titleCase(line.kind)}</div>
                  )}
                </td>
                <td className="right tabular">{money(line.amount_cents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card-pad">
        <div className="sum">
          <div className="sum-row">
            <span className="label">Subtotal</span>
            <span className="tabular">{money(invoice.subtotal_cents)}</span>
          </div>
          <div className="sum-row">
            <span className="label">Tax</span>
            <span className="tabular">{money(invoice.tax_cents)}</span>
          </div>
          <div className="sum-row total">
            <span>Total</span>
            <span className="tabular">{money(invoice.total_cents)}</span>
          </div>
          {invoice.amount_paid_cents > 0 && (
            <div className="sum-row">
              <span className="label">Paid</span>
              <span className="tabular">−{money(invoice.amount_paid_cents)}</span>
            </div>
          )}
          {balance > 0 && invoice.status !== 'void' && (
            <div className="sum-row total" style={{ color: 'var(--alert)' }}>
              <span>Balance due {formatDate(invoice.due_at)}</span>
              <span className="tabular">{money(balance)}</span>
            </div>
          )}
        </div>

        {invoice.payments.length > 0 && (
          <div style={{ marginTop: 14 }}>
            <div className="eyebrow" style={{ marginBottom: 6 }}>Payment history</div>
            <div className="stack" style={{ gap: 4 }}>
              {invoice.payments.map(p => (
                <div key={p.id} className="spread small">
                  <span className="dim">
                    {titleCase(p.kind)}
                    {p.card_last4 && <> · {p.card_brand} ····{p.card_last4}</>}
                    {p.status === 'failed' && (
                      <span style={{ color: 'var(--alert)' }}> — {p.failure_reason}</span>
                    )}
                  </span>
                  <span className="tabular nowrap dim">
                    {money(p.amount_cents)} · {formatDateTime(p.created_at)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {paying && (
        <PayDialog invoice={invoice} balance={balance}
          onClose={() => setPaying(false)}
          onPaid={() => { setPaying(false); onPaid() }} />
      )}
    </div>
  )
}

function PayDialog({ invoice, balance, onClose, onPaid }:
  { invoice: Invoice; balance: number; onClose: () => void; onPaid: () => void }) {
  const [card, setCard] = useState<CardDetails>(emptyCard)

  const pay = useAction(async () => {
    await api.post(`/invoices/${invoice.id}/pay`, {
      card: {
        number: card.number, expiry_month: Number(card.month),
        expiry_year: Number(card.year), cvc: card.cvc, name: card.name,
      },
    })
    onPaid()
  })

  return (
    <Modal title={`Pay ${money(balance)}`} onClose={onClose}
      footer={<>
        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button className="btn" disabled={pay.busy} onClick={() => void pay.run()}>
          {pay.busy ? 'Taking payment…' : `Pay ${money(balance)}`}
        </button>
      </>}>
      <div className="stack">
        <ErrorNote error={pay.error} />
        <CardForm card={card} onChange={setCard} />
      </div>
    </Modal>
  )
}
