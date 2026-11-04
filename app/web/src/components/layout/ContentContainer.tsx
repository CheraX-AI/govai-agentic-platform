import type { ReactNode } from 'react'
import type { LayoutPreview } from './Sidebar'

export function ContentContainer({ page, children, protectedDemo = false }: { page: LayoutPreview; children: ReactNode; protectedDemo?: boolean }) {
  return <main id="application-content" tabIndex={-1} className={`application-content ${page}-content`}>
    <p className="layout-preview-label">{protectedDemo ? "Protected demonstration - synthetic details - actions unavailable" : "Layout preview - synthetic details - actions unavailable"}</p>
    {children}
  </main>
}
