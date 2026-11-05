export type BrowserSession = { kind: 'server'; userId: string; createdAt: number; expiresAt: number }
export type AuthState =
  | { status: 'checking' }
  | { status: 'authenticated'; session: BrowserSession }
  | { status: 'unauthenticated'; reason?: 'expired' }
  | { status: 'error' }

export interface AuthenticationAdapter {
  login(identifier: string, password: string): Promise<BrowserSession>
  refresh(): Promise<void>
  restore(): Promise<BrowserSession | null>
  logout(): Promise<void>
}
