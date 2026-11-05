export type BrowserSession = { kind: 'server'; userId: string; createdAt: number; expiresAt: number }
export type MockSession = { kind: 'demo'; userId: 'synthetic-harshan'; createdAt: number; expiresAt: number }
export type AuthState =
  | { status: 'checking' }
  | { status: 'authenticated'; session: MockSession | BrowserSession }
  | { status: 'unauthenticated'; reason?: 'expired' }
  | { status: 'error' }

export interface AuthenticationAdapter {
  login?(identifier: string, password: string): Promise<BrowserSession>
  refresh?(): Promise<void>
  restore(): Promise<MockSession | BrowserSession | null>
  createMockSession(): Promise<MockSession>
  logout(): Promise<void>
}
