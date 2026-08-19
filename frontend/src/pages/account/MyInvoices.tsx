import { Link } from 'react-router-dom'
import { Page, PageHead } from '../../components/Layout'
import { AccountTabs } from '../../components/AccountTabs'
import { InvoiceCard } from '../../components/InvoiceCard'
import { Empty, ErrorNote, SkeletonRows } from '../../components/ui'
import { api } from '../../api/client'
import type { Invoice } from '../../api/types'
import { useApi } from '../../hooks/useApi'
import { money } from '../../format'

export function MyInvoices() {
  const { data, loading, error, reload } = useApi(
    () => api.get<{ invoices: Invoice[] }>('/invoices'), [])

  const invoices = data?.invoices ?? []
  const outstanding = invoices
    .filter(i => i.status !== 'void')
    .reduce((sum, i) => sum + (i.total_cents - i.amount_paid_cents), 0)

  return (
    <Page>
      <PageHead title="Invoices"
        subtitle={outstanding > 0
          ? <>You have <strong>{money(outstanding)}</strong> outstanding.</>
          : 'Everything is settled.'} />
      <AccountTabs />
      <ErrorNote error={error} />

      {loading && !data && <div className="card"><SkeletonRows /></div>}

      {data && invoices.length === 0 && (
        <div className="card">
          <Empty title="No invoices yet">
            Invoices appear here once you have made a booking.{' '}
            <Link to="/">Browse the catalog</Link>.
          </Empty>
        </div>
      )}

      <div className="stack">
        {invoices.map(inv => <InvoiceCard key={inv.id} invoice={inv} onPaid={reload} />)}
      </div>
    </Page>
  )
}
