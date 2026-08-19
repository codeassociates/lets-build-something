import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { addDays, todayISO } from './format'

export interface CartLine {
  model_id: number
  name: string
  sku: string
  category_slug: string
  quantity: number
}

interface CartState {
  lines: CartLine[]
  startDate: string
  endDate: string
  itemCount: number
  setDates: (start: string, end: string) => void
  add: (line: Omit<CartLine, 'quantity'>, quantity?: number) => void
  setQuantity: (modelId: number, quantity: number) => void
  remove: (modelId: number) => void
  clear: () => void
  has: (modelId: number) => boolean
}

const CartContext = createContext<CartState | null>(null)

// The basket survives a reload: choosing dates and equipment is real work, and
// losing it to an accidental refresh is the kind of thing that loses a booking.
const STORAGE_KEY = 'kestrel.cart.v1'

interface Stored { lines: CartLine[]; startDate: string; endDate: string }

function load(): Stored {
  const fallback: Stored = { lines: [], startDate: todayISO(), endDate: addDays(todayISO(), 2) }
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as Stored
    if (!Array.isArray(parsed.lines)) return fallback
    // A basket left overnight would otherwise hold dates now in the past.
    const start = parsed.startDate < todayISO() ? todayISO() : parsed.startDate
    const end = parsed.endDate < start ? start : parsed.endDate
    return { lines: parsed.lines, startDate: start, endDate: end }
  } catch {
    return fallback
  }
}

export function CartProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<Stored>(load)

  useEffect(() => {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)) } catch { /* private mode */ }
  }, [state])

  const setDates = useCallback((startDate: string, endDate: string) => {
    setState(s => ({ ...s, startDate, endDate: endDate < startDate ? startDate : endDate }))
  }, [])

  const add = useCallback((line: Omit<CartLine, 'quantity'>, quantity = 1) => {
    setState(s => {
      const existing = s.lines.find(l => l.model_id === line.model_id)
      if (existing) {
        return {
          ...s,
          lines: s.lines.map(l =>
            l.model_id === line.model_id ? { ...l, quantity: l.quantity + quantity } : l),
        }
      }
      return { ...s, lines: [...s.lines, { ...line, quantity }] }
    })
  }, [])

  const setQuantity = useCallback((modelId: number, quantity: number) => {
    setState(s => quantity <= 0
      ? { ...s, lines: s.lines.filter(l => l.model_id !== modelId) }
      : { ...s, lines: s.lines.map(l => l.model_id === modelId ? { ...l, quantity } : l) })
  }, [])

  const remove = useCallback((modelId: number) => {
    setState(s => ({ ...s, lines: s.lines.filter(l => l.model_id !== modelId) }))
  }, [])

  const clear = useCallback(() => {
    setState(s => ({ ...s, lines: [] }))
  }, [])

  const value = useMemo<CartState>(() => ({
    lines: state.lines,
    startDate: state.startDate,
    endDate: state.endDate,
    itemCount: state.lines.reduce((n, l) => n + l.quantity, 0),
    setDates, add, setQuantity, remove, clear,
    has: (modelId: number) => state.lines.some(l => l.model_id === modelId),
  }), [state, setDates, add, setQuantity, remove, clear])

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>
}

export function useCart(): CartState {
  const ctx = useContext(CartContext)
  if (!ctx) throw new Error('useCart must be used inside CartProvider')
  return ctx
}
