export type MockSession = { kind: 'demo'; userId: 'synthetic-harshan'; createdAt: number; expiresAt: number }
export type AuthState =
  | { status: 'checking' }
  | { status: 'authenticated'; session: MockSession }
  | { status: 'unauthenticated'; reason?: 'expired' }
  | { status: 'error' }

export interface AuthenticationAdapter {
  restore(): Promise<MockSession | null>
  createMockSession(): Promise<MockSession>
  logout(): Promise<void>
}
