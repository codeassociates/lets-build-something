/** Formatting helpers. Money is always integer cents until the moment it is shown. */

export function money(cents: number | undefined | null): string {
  const value = (cents ?? 0) / 100
  return value.toLocaleString('en-US', { style: 'currency', currency: 'USD' })
}

/** Drops the cents on round amounts, for headline figures where they add noise. */
export function moneyShort(cents: number | undefined | null): string {
  const value = (cents ?? 0) / 100
  return value % 1 === 0
    ? value.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 })
    : money(cents)
}

/** Parses a YYYY-MM-DD date as local midnight, not UTC, so it never shifts a day. */
export function parseDate(iso: string): Date {
  const [y, m, d] = iso.slice(0, 10).split('-').map(Number)
  return new Date(y, (m ?? 1) - 1, d ?? 1)
}

export function formatDate(iso: string | null | undefined, opts?: Intl.DateTimeFormatOptions): string {
  if (!iso) return '—'
  const date = iso.length <= 10 ? parseDate(iso) : new Date(iso)
  return date.toLocaleDateString('en-US',
    opts ?? { day: 'numeric', month: 'short', year: 'numeric' })
}

export function formatDateShort(iso: string | null | undefined): string {
  return formatDate(iso, { day: 'numeric', month: 'short' })
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('en-US',
    { day: 'numeric', month: 'short', year: 'numeric', hour: 'numeric', minute: '2-digit' })
}

export function todayISO(): string { return toISO(new Date()) }

export function toISO(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export function addDays(iso: string, days: number): string {
  const date = parseDate(iso)
  date.setDate(date.getDate() + days)
  return toISO(date)
}

/** Inclusive day count, matching how the backend prices a rental. */
export function rentalDays(start: string, end: string): number {
  const ms = parseDate(end).getTime() - parseDate(start).getTime()
  return Math.max(1, Math.round(ms / 86400000) + 1)
}

export function relativeDay(iso: string): string {
  const days = Math.round((parseDate(iso).getTime() - parseDate(todayISO()).getTime()) / 86400000)
  if (days === 0) return 'today'
  if (days === 1) return 'tomorrow'
  if (days === -1) return 'yesterday'
  if (days > 0) return `in ${days} days`
  return `${Math.abs(days)} days ago`
}

export function plural(n: number, one: string, many?: string): string {
  return n === 1 ? one : (many ?? one + 's')
}

export function titleCase(s: string): string {
  return s.replace(/[_-]/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

export function initials(name: string): string {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map(p => p[0]?.toUpperCase() ?? '').join('')
}
