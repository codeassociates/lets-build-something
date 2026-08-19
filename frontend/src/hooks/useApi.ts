import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'

interface State<T> {
  data: T | null
  error: ApiError | null
  loading: boolean
}

/**
 * Loads data on mount and whenever a dependency changes. Returns a `reload`
 * so a mutation can refresh the view without a full page navigation.
 *
 * A response that arrives after a newer request was issued is discarded, so
 * fast typing in a search box cannot leave stale results on screen.
 */
export function useApi<T>(fetcher: () => Promise<T>, deps: unknown[] = []): State<T> & { reload: () => void } {
  const [state, setState] = useState<State<T>>({ data: null, error: null, loading: true })
  const [nonce, setNonce] = useState(0)
  const generation = useRef(0)

  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  useEffect(() => {
    const mine = ++generation.current
    setState(s => ({ ...s, loading: true }))

    fetcherRef.current()
      .then(data => {
        if (generation.current === mine) setState({ data, error: null, loading: false })
      })
      .catch((error: ApiError) => {
        if (generation.current === mine) setState({ data: null, error, loading: false })
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  const reload = useCallback(() => setNonce(n => n + 1), [])
  return { ...state, reload }
}

/** Debounces a value — used for search boxes so each keystroke is not a request. */
export function useDebounced<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

/** Tracks an in-flight mutation so buttons can disable themselves. */
export function useAction<A extends unknown[], R>(fn: (...args: A) => Promise<R>) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  const run = useCallback(async (...args: A): Promise<R | undefined> => {
    setBusy(true)
    setError(null)
    try {
      return await fn(...args)
    } catch (e) {
      setError(e as ApiError)
      return undefined
    } finally {
      setBusy(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fn])

  return { run, busy, error, setError }
}
