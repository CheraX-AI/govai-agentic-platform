import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { ApplicationLayout } from './ApplicationLayout'
import type { LayoutPreview } from './Sidebar'

export type ApplicationPageProps = { protectedDemo?: boolean; onDemoLogout?: () => void }
export function ApplicationPageLayout({ page, protectedDemo = false, onDemoLogout, children }: ApplicationPageProps & { page: LayoutPreview; children: ReactNode }) {
  useEffect(() => { if (protectedDemo) document.getElementById('application-content')?.focus() }, [protectedDemo])
  return <ApplicationLayout page={page} protectedDemo={protectedDemo} onDemoLogout={protectedDemo ? onDemoLogout : undefined}>
    {protectedDemo && <div className="demo-session-notice"><p role="status">Mock session - no real authentication. Synthetic user details only.</p><button type="button" onClick={onDemoLogout}>Demo logout</button></div>}
    {children}
  </ApplicationLayout>
}
