import type { ReactNode } from 'react'

export function StatusMessage({ kind, children }: { kind: 'pending' | 'success' | 'error'; children: ReactNode }) {
  return <p className={`status-message ${kind}`} role={kind === 'error' ? 'alert' : 'status'}>{children}</p>
}
