import { createContext, useContext } from 'react'
import type { AuthState } from './types'

export type AuthContextValue = { state: AuthState; login: (identifier: string, password: string) => Promise<boolean>; retry: () => Promise<void>; logout: () => Promise<void> }
export const AuthContext = createContext<AuthContextValue | null>(null)
export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw Error('AuthProvider is required')
  return context
}
