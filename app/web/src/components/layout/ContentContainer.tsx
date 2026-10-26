import type { ReactNode } from 'react'
import type { LayoutPreview } from './Sidebar'

export function ContentContainer({ page, children }: { page: LayoutPreview; children: ReactNode }) {
  return <main id="application-content" tabIndex={-1} className={`application-content ${page}-content`}>
    <p className="layout-preview-label">Layout preview · synthetic details · actions unavailable</p>
    {children}
  </main>
}
