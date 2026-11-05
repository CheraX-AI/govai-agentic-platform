export type RecoveryReceipt = { nic: string; maskedEmail?: string }
export async function requestRecovery(nic: string): Promise<RecoveryReceipt> {
  const base = new URL(import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001')
  if (base.protocol !== 'https:' || base.username || base.password || base.search || base.hash || base.pathname !== '/') throw new Error('Recovery configuration is unavailable.')
  const normalized = nic.trim().toUpperCase()
  let response: Response
  try {
    response = await fetch(new URL('/api/v1/auth/forgot-password', base), {
      method: 'POST', credentials: 'omit', cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(35_000),
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ nic: normalized }),
    })
  } catch { throw new Error('Unable to confirm delivery. Check your email before requesting another link.') }
  if (response.status === 429) throw new Error('Please wait before requesting another link.')
  if (response.status === 400 || response.status === 422) throw new Error('Please check your NIC number.')
  if (response.status !== 202) throw new Error('Recovery is temporarily unavailable. Please try again later.')
  const result = await response.json().catch(() => null)
  if (result?.status !== 'received' || (result.masked_email !== undefined && typeof result.masked_email !== 'string')) throw new Error('Unable to confirm the recovery request.')
  return { nic: normalized, maskedEmail: result.masked_email }
}

export function validateResetPasswords(password: string, confirmation: string) {
  return {
    password: [...password].length < 8 || new TextEncoder().encode(password).length > 1024 ? 'Use at least 8 characters and at most 1024 UTF-8 bytes.' : undefined,
    confirmation: !confirmation ? 'Confirm your new password.' : password !== confirmation ? 'Passwords must match exactly.' : undefined,
  }
}
export async function resetPassword(token: string, password: string, confirmation: string): Promise<void> {
  if (!/^[A-Za-z0-9_-]{43}$/.test(token)) throw new Error('This reset link is invalid. Request a new link.')
  const base = new URL(import.meta.env?.VITE_AUTH_API_URL ?? 'https://localhost:5001')
  if (base.protocol !== 'https:' || base.username || base.password || base.search || base.hash || base.pathname !== '/') throw new Error('Recovery configuration is unavailable.')
  let response: Response
  try {
    response = await fetch(new URL('/api/v1/auth/reset-password', base), {
      method: 'POST', credentials: 'omit', cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(35_000),
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token, new_password: password, confirm_password: confirmation }),
    })
  } catch { throw new Error('Unable to confirm the password change. Try logging in or request a new reset link; no automatic retry was made.') }
  if (response.status === 204) return
  const result = await response.json().catch(() => null)
  if (response.status === 400 && result?.code === 'invalid_or_expired_reset') throw new Error('This reset link is invalid, expired or already used. Request a new link.')
  if (response.status === 429) throw new Error('Too many attempts. Please wait before trying again.')
  if (response.status === 400 || response.status === 422) throw new Error('Check your password requirements and confirmation. A new reset link may be needed.')
  throw new Error('Unable to confirm the password change. Try logging in or request a new reset link.')
}
// Remove the bearer fragment before rendering. Only the reset page consumes it;
// callers keep the returned value in memory, never browser storage.
export function takeResetLink(target: Pick<Location, 'pathname' | 'hash'> = location, historyPort: Pick<History, 'replaceState'> = history): { token: string | null } | null {
  if (!target.hash.startsWith('#token=')) return null
  const token = target.hash.slice(7)
  historyPort.replaceState(null, '', target.pathname + '#/reset-password')
  return { token: target.pathname === '/reset-password' && /^[A-Za-z0-9_-]{43}$/.test(token) ? token : null }
}
