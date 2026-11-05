import type { ReactNode } from 'react'
import type { ApplicationPage } from './Sidebar'

export function ContentContainer({ page, children }: { page: ApplicationPage; children: ReactNode }) {
  return <main id="application-content" tabIndex={-1} className={`application-content ${page}-content`}>
    {children}
  </main>
}
