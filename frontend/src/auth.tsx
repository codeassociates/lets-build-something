import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { api } from './api/client'
import type { Role, User } from './api/types'

interface AuthState {
  user: User | null
  loading: boolean
  signIn: (email: string, password: string) => Promise<User>
  signOut: () => Promise<void>
  refresh: () => Promise<void>
  /** True when the signed-in user holds at least the given role. */
  can: (role: Role) => boolean
}

const AuthContext = createContext<AuthState | null>(null)

const RANK: Record<Role, number> = { customer: 1, staff: 2, admin: 3 }

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      const { user } = await api.get<{ user: User | null }>('/auth/me')
      setUser(user)
    } catch {
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  const signIn = useCallback(async (email: string, password: string) => {
    const { user } = await api.post<{ user: User }>('/auth/login', { email, password })
    setUser(user)
    return user
  }, [])

  const signOut = useCallback(async () => {
    await api.post('/auth/logout')
    setUser(null)
  }, [])

  const can = useCallback((role: Role) =>
    user !== null && RANK[user.role] >= RANK[role], [user])

  return (
    <AuthContext.Provider value={{ user, loading, signIn, signOut, refresh, can }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

/** Where a user should land after signing in, based on what they do here. */
export function homeFor(user: User | null): string {
  if (!user) return '/'
  if (user.role === 'admin') return '/admin'
  if (user.role === 'staff') return '/desk'
  return '/account'
}
