import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { ApplicationLayout } from './ApplicationLayout'
import type { LayoutPreview } from './Sidebar'

export type ApplicationPageProps = { protectedDemo?: boolean; onDemoLogout?: () => void }
export function ApplicationPageLayout({ page, protectedDemo = false, onDemoLogout, children }: ApplicationPageProps & { page: LayoutPreview; children: ReactNode }) {
  useEffect(() => { if (protectedDemo) document.getElementById('application-content')?.focus() }, [protectedDemo])
  return <ApplicationLayout page={page} protectedDemo={protectedDemo} onDemoLogout={protectedDemo ? onDemoLogout : undefined}>
    {protectedDemo && <div className="demo-session-notice"><p role="status">Signed in securely. Chat and profile content remain previews.</p><button type="button" onClick={onDemoLogout}>Log out</button></div>}
    {children}
  </ApplicationLayout>
}
