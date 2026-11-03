import type { AuthenticationAdapter, MockSession } from '../auth/types'

export const mockSessionKey = 'govai.dev.mock-session.v1'
export const mockSessionLifetime = 15 * 60 * 1000
type StoragePort = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

// Only a fixed synthetic identity and bounded timestamps are accepted. Never a token.
export function parseMockSession(raw: string, now = Date.now()): MockSession | null {
  try {
    const value = JSON.parse(raw)
    if (!value || typeof value !== 'object' || Array.isArray(value)) return null
    if (Object.keys(value).sort().join(',') !== 'createdAt,expiresAt,kind,userId,version') return null
    if (value.version !== 1 || value.kind !== 'demo' || value.userId !== 'synthetic-harshan') return null
    if (!Number.isSafeInteger(value.createdAt) || !Number.isSafeInteger(value.expiresAt)) return null
    if (value.createdAt > now || value.createdAt < 0 || value.expiresAt <= now || value.expiresAt <= value.createdAt || value.expiresAt - value.createdAt > mockSessionLifetime) return null
    return { kind: 'demo', userId: 'synthetic-harshan', createdAt: value.createdAt, expiresAt: value.expiresAt }
  } catch { return null }
}

export function createMockAuthAdapter(storage: () => StoragePort, enabled: boolean, now = Date.now): AuthenticationAdapter {
  return {
    async restore() {
      if (!enabled) return null
      await new Promise(resolve => setTimeout(resolve, 200))
      const raw = storage().getItem(mockSessionKey)
      if (raw === null) return null
      const session = parseMockSession(raw, now())
      if (!session) storage().removeItem(mockSessionKey)
      return session
    },
    async createMockSession() {
      if (!enabled) throw Error('Mock sessions disabled')
      const createdAt = now()
      const session: MockSession = { kind: 'demo', userId: 'synthetic-harshan', createdAt, expiresAt: createdAt + mockSessionLifetime }
      storage().setItem(mockSessionKey, JSON.stringify({ version: 1, ...session }))
      return session
    },
    async logout() { if (enabled) storage().removeItem(mockSessionKey) },
  }
}
