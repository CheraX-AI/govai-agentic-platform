import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { useAuth } from './AuthContext'
import { AuthStatusPage } from '../pages/AuthStatusPage'
import { loginDestination, navigate } from '../routing/routes'
import type { ProtectedDestination } from '../routing/routes'

export function ProtectedRoute({ destination, children }: { destination: ProtectedDestination; children: ReactNode }) {
  const { state } = useAuth()
  const redirect = state.status === 'unauthenticated'
  useEffect(() => {
    if (redirect) navigate(loginDestination(destination, state.status === 'unauthenticated' && state.reason === 'expired'), true)
  }, [redirect, destination, state])
  if (state.status === 'error') return <AuthStatusPage error />
  if (state.status !== 'authenticated') return <AuthStatusPage />
  return children
}
