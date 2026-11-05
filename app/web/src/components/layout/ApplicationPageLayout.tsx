import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { ApplicationLayout } from './ApplicationLayout'
import type { ApplicationPage } from './Sidebar'

export function ApplicationPageLayout({ page, onLogout, children }: { page: ApplicationPage; onLogout: () => void; children: ReactNode }) {
  useEffect(() => { document.getElementById('application-content')?.focus() }, [])
  return <ApplicationLayout page={page} onLogout={onLogout}>{children}</ApplicationLayout>
}
