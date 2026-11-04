import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { browserAuthAdapter } from '../services/browserAuthAdapter'
import { AuthContext } from './AuthContext'
import type { AuthenticationAdapter, AuthState } from './types'

const defaultAdapter = browserAuthAdapter

export function AuthProvider({ children, adapter = defaultAdapter }: { children: ReactNode; adapter?: AuthenticationAdapter }) {
  const [state, setState] = useState<AuthState>({ status: 'checking' })
  const current = useRef<AuthState>(state)
  const generation = useRef(0)
  const mounted = useRef(false)
  const publish = useCallback((next: AuthState) => { current.current = next; if (mounted.current) setState(next) }, [])
  const retry = useCallback(async () => {
    const request = ++generation.current
    publish({ status: 'checking' })
    try {
      const session = await adapter.restore()
      if (!mounted.current || request !== generation.current) return
      publish(session && session.expiresAt > Date.now() ? { status: 'authenticated', session } : { status: 'unauthenticated' })
    } catch { if (mounted.current && request === generation.current) publish({ status: 'error' }) }
  }, [adapter, publish])
  const logout = useCallback(async () => {
    const request = ++generation.current
    // Clear protected state immediately, even when storage cleanup fails.
    publish({ status: 'checking' })
    try {
      await adapter.logout()
      if (mounted.current && request === generation.current) publish({ status: 'unauthenticated' })
    } catch { if (mounted.current && request === generation.current) publish({ status: 'error' }) }
  }, [adapter, publish])
  const login = useCallback(async (identifier: string, password: string) => {
    if (!adapter.login) throw new Error('Login is unavailable.')
    const request = ++generation.current
    publish({ status: 'checking' })
    try {
      const session = await adapter.login(identifier, password)
      if (!mounted.current || request !== generation.current) return false
      publish({ status: 'authenticated', session })
      return true
    } catch (error) {
      if (mounted.current && request === generation.current) publish({ status: 'unauthenticated' })
      throw error
    }
  }, [adapter, publish])
  const startMockSession = useCallback(async () => {
    if (!import.meta.env.DEV || adapter === defaultAdapter) return false
    const request = ++generation.current
    publish({ status: 'checking' })
    try {
      const session = await adapter.createMockSession()
      if (!mounted.current || request !== generation.current) return false
      publish({ status: 'authenticated', session })
      return true
    } catch { if (mounted.current && request === generation.current) publish({ status: 'error' }); return false }
  }, [adapter, publish])
  useEffect(() => {
    mounted.current = true
    void retry()
    function checkExpiry() {
      const snapshot = current.current
      if (snapshot.status === 'authenticated' && snapshot.session.expiresAt <= Date.now()) {
        const request = ++generation.current
        publish({ status: 'unauthenticated', reason: 'expired' })
        void adapter.logout().catch(() => { if (mounted.current && request === generation.current) publish({ status: 'error' }) })
      }
    }
    const interval = setInterval(checkExpiry, 250)
    addEventListener('focus', checkExpiry)
    addEventListener('hashchange', checkExpiry)
    addEventListener('pageshow', checkExpiry)
    document.addEventListener('visibilitychange', checkExpiry)
    const cancel = () => { mounted.current = false; generation.current += 1 }
    return () => {
      cancel(); clearInterval(interval)
      removeEventListener('focus', checkExpiry); removeEventListener('hashchange', checkExpiry); removeEventListener('pageshow', checkExpiry); document.removeEventListener('visibilitychange', checkExpiry)
    }
  }, [adapter, retry, publish])
  useEffect(() => {
    if (state.status !== 'authenticated' || !adapter.refresh) return
    const interval = setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void adapter.refresh!().catch(() => { void retry() })
    }, 120_000)
    const check = () => { void retry() }
    addEventListener('focus', check)
    return () => { clearInterval(interval); removeEventListener('focus', check) }
  }, [state.status, adapter, retry])
  return <AuthContext.Provider value={{ state, retry, login, startMockSession, logout }}>{children}</AuthContext.Provider>
}
