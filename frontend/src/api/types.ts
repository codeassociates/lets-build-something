// Mirrors the JSON the Go API returns. Kept by hand rather than generated:
// the surface is small, and a hand-written type is the place to document what
// a field means to the UI.

export type Role = 'customer' | 'staff' | 'admin'

export interface User {
  id: number
  email: string
  role: Role
  full_name: string
  phone: string
  company: string
  address_line1: string
  address_line2: string
  city: string
  state: string
  postal_code: string
  license_number: string
  active: boolean
  created_at: string
}

export interface Category {
  id: number
  slug: string
  name: string
  description: string
  sort_order: number
  model_count: number
}

export interface Model {
  id: number
  category_id: number
  category_name: string
  category_slug: string
  sku: string
  name: string
  description: string
  manufacturer: string
  daily_rate_cents: number
  weekly_rate_cents: number
  monthly_rate_cents: number
  deposit_cents: number
  replacement_value_cents: number
  requires_license: boolean
  specs: Record<string, string>
  image_url: string
  active: boolean
  total_units: number
  /** Free units across the requested window; only meaningful with dates. */
  available_units: number
}

export interface Unit {
  id: number
  model_id: number
  model_name: string
  sku: string
  asset_tag: string
  serial_number: string
  status: 'available' | 'out' | 'maintenance' | 'retired'
  condition_notes: string
  meter_hours: number
  acquired_on: string | null
  reservation_id?: number
  reservation_number?: string
}

export type ReservationStatus = 'confirmed' | 'picked_up' | 'returned' | 'cancelled'

export interface Assignment {
  id: number
  reservation_item_id: number
  unit_id: number
  asset_tag: string
  serial_number: string
  checked_out_at: string
  checked_in_at: string | null
  checkout_notes: string
  checkin_notes: string
  damage_cents: number
  meter_out: number | null
  meter_in: number | null
}

export interface ReservationItem {
  id: number
  reservation_id: number
  model_id: number
  model_name: string
  sku: string
  image_url: string
  quantity: number
  rate_basis: 'daily' | 'weekly' | 'monthly'
  rate_cents: number
  billable_periods: number
  line_total_cents: number
  daily_rate_cents: number
  assignments: Assignment[]
}

export interface Reservation {
  id: number
  reservation_number: string
  customer_id: number
  customer_name: string
  customer_email: string
  customer_phone: string
  status: ReservationStatus
  start_date: string
  end_date: string
  picked_up_at: string | null
  returned_at: string | null
  subtotal_cents: number
  tax_cents: number
  deposit_cents: number
  total_cents: number
  notes: string
  created_at: string
  items: ReservationItem[]
  rental_days: number
  is_overdue: boolean
  days_overdue: number
}

export interface QuoteLine {
  model_id: number
  model_name: string
  sku: string
  image_url: string
  quantity: number
  rate_basis: string
  rate_cents: number
  billable_periods: number
  line_total_cents: number
  deposit_cents: number
  requires_license: boolean
  available_units: number
  available: boolean
}

export interface Quote {
  start_date: string
  end_date: string
  rental_days: number
  lines: QuoteLine[]
  subtotal_cents: number
  tax_cents: number
  total_cents: number
  deposit_cents: number
  due_now_cents: number
  all_available: boolean
}

export interface InvoiceLine {
  id: number
  kind: 'rental' | 'late_fee' | 'damage' | 'deposit' | 'discount'
  description: string
  quantity: number
  unit_amount_cents: number
  amount_cents: number
  sort_order: number
}

export interface Payment {
  id: number
  invoice_id: number | null
  reservation_id: number | null
  customer_id: number
  kind: 'authorization' | 'capture' | 'refund' | 'release'
  amount_cents: number
  status: 'pending' | 'succeeded' | 'failed'
  gateway: string
  gateway_ref: string
  card_brand: string
  card_last4: string
  failure_reason: string
  created_at: string
}

export interface Invoice {
  id: number
  invoice_number: string
  reservation_id: number
  reservation_number: string
  customer_id: number
  customer_name: string
  customer_email: string
  status: 'draft' | 'issued' | 'paid' | 'void'
  issued_at: string
  due_at: string
  subtotal_cents: number
  tax_cents: number
  total_cents: number
  amount_paid_cents: number
  lines: InvoiceLine[]
  payments: Payment[]
}

export interface DeskSummary {
  date: string
  pickups_due: Reservation[]
  returns_due: Reservation[]
  overdue: Reservation[]
  out_now: number
  pickups_due_count: number
  returns_due_count: number
  overdue_count: number
}

export interface Stats {
  active_rentals: number
  upcoming_pickups: number
  overdue_rentals: number
  units_out: number
  units_available: number
  units_maintenance: number
  customers: number
  revenue_mtd_cents: number
  outstanding_cents: number
  reservations_mtd: number
}

export interface SentEmail {
  id: number
  to_address: string
  to_name: string
  subject: string
  template: string
  body_text: string
  body_html: string
  reservation_id: number | null
  status: 'sent' | 'failed'
  error: string
  created_at: string
}

export interface Job {
  id: number
  kind: string
  payload: unknown
  dedupe_key: string | null
  run_at: string
  status: 'pending' | 'running' | 'done' | 'failed'
  attempts: number
  last_error: string
  created_at: string
  updated_at: string
}

export interface Card {
  number: string
  expiry_month: number
  expiry_year: number
  cvc: string
  name: string
}

export interface CheckoutLine {
  item_id: number
  unit_ids: number[]
  notes?: string
  meter_out?: number | null
}

export interface CheckinLine {
  assignment_id: number
  meter_in?: number | null
  notes?: string
  damage_cents: number
  needs_maintenance: boolean
}

export interface CheckinResult {
  reservation: Reservation
  days_overdue: number
  late_fee_cents: number
  damage_cents: number
  extra_invoice_id: number | null
  deposit_taken_cents: number
  deposit_released_cents: number
}

export interface BookResult {
  reservation: Reservation
  invoice: Invoice
  deposit_held_cents: number
  payment_error?: string
}
