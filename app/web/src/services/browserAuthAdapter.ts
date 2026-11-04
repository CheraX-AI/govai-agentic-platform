import type { AuthenticationAdapter, BrowserSession } from '../auth/types'

export class WebAuthError extends Error {
  status: number
  code: string
  constructor(status: number, code: string) {
    super(status === 401 ? 'Invalid email/NIC or password, or the account is not ready for login.' : status === 429 ? 'Too many attempts. Please wait before trying again.' : status === 409 ? 'An existing session must be logged out before signing in again.' : 'Unable to complete authentication. Please try again later.')
    this.status = status
    this.code = code
  }
}
async function request(path: string, method = 'GET', body?: unknown, csrf?: string) {
  const base = new URL(import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001')
  if (base.protocol !== 'https:' || base.username || base.password || base.search || base.hash || base.pathname !== '/') throw new Error('Authentication configuration unavailable.')
  let response: Response
  try {
    response = await fetch(new URL('/api/v1/web/' + path, base), {
      method, credentials: 'include', cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(15_000),
      headers: { 'X-GovAI-Web': '1', ...(body ? { 'Content-Type': 'application/json' } : {}), ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
      ...(body ? { body: JSON.stringify(body) } : {}),
    })
  } catch { throw new Error('Authentication service is unavailable. Please try again later.') }
  if (!response.ok) {
    const result = await response.json().catch(() => null)
    throw new WebAuthError(response.status, typeof result?.code === 'string' ? result.code : 'request_failed')
  }
  return response.status === 204 ? undefined : response.json()
}
let queue: Promise<unknown> = Promise.resolve()
function exclusive<T>(action: () => Promise<T>): Promise<T> {
  const execute = () => typeof navigator !== 'undefined' && navigator.locks ? navigator.locks.request('govai-web-session', action) : action()
  const result = queue.then(execute, execute)
  queue = result.catch(() => undefined)
  return result
}
async function identity(): Promise<BrowserSession> {
  const value = await request('session')
  if (typeof value?.subject !== 'string' || !value.subject) throw new Error('Invalid session response.')
  return { kind: 'server', userId: value.subject, createdAt: Date.now(), expiresAt: Number.MAX_SAFE_INTEGER }
}
async function csrfToken(): Promise<string> {
  const value = await request('csrf')
  if (typeof value?.csrf_token !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(value.csrf_token)) throw new Error('Invalid session response.')
  return value.csrf_token
}
export const browserAuthAdapter: AuthenticationAdapter = {
  async restore() {
    try { return await identity() } catch (error) {
      if (!(error instanceof WebAuthError) || error.status !== 401) throw error
      return exclusive(async () => {
        try { await request('refresh', 'POST', undefined, await csrfToken()); return await identity() }
        catch (error) { if (error instanceof WebAuthError && error.status === 401) return null; throw error }
      })
    }
  },
  login: (identifier, password) => exclusive(async () => { await request('login', 'POST', { identifier, password }); return identity() }),
  refresh: () => exclusive(async () => { await request('refresh', 'POST', undefined, await csrfToken()) }),
  logout: () => exclusive(async () => {
    try { await request('logout', 'POST', undefined, await csrfToken()) }
    catch (error) { if (error instanceof WebAuthError && (error.status === 401 || error.code === 'provider_revocation_pending')) return; throw error }
  }),
  async createMockSession() { throw new Error('Demo authentication is disabled.') },
}
