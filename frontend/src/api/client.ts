// A thin fetch wrapper. Every call goes through here so error handling,
// credentials and JSON decoding are defined exactly once.

export class ApiError extends Error {
  status: number
  code: string
  /** Per-field messages from a validation failure, keyed by field name. */
  fields: Record<string, string>

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }

  get isAuth() { return this.status === 401 }
  get isForbidden() { return this.status === 403 }
}

const BASE = '/api/v1'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch(BASE + path, {
      method,
      // The session lives in a cookie, so every request must carry it.
      credentials: 'same-origin',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', 'Could not reach the server. Check your connection.')
  }

  if (response.status === 204) return undefined as T

  const text = await response.text()
  let payload: any = null
  if (text) {
    try { payload = JSON.parse(text) } catch {
      if (!response.ok) {
        throw new ApiError(response.status, 'bad_response', text.slice(0, 200))
      }
    }
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      payload?.code ?? 'error',
      payload?.message ?? `Request failed (${response.status}).`,
      payload?.fields ?? {},
    )
  }
  return payload as T
}

/** Builds a query string, dropping empty values so URLs stay readable. */
export function qs(params: Record<string, string | number | boolean | undefined | null>): string {
  const parts = Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== null && v !== '' && v !== false)
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
  return parts.length ? `?${parts.join('&')}` : ''
}

export const api = {
  get:   <T>(path: string) => request<T>('GET', path),
  post:  <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
  del:   <T>(path: string) => request<T>('DELETE', path),
}
