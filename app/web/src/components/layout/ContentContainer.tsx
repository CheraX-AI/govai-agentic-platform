import type { ReactNode } from 'react'
import type { LayoutPreview } from './Sidebar'

export function ContentContainer({ page, children, protectedDemo = false }: { page: LayoutPreview; children: ReactNode; protectedDemo?: boolean }) {
  return <main id="application-content" tabIndex={-1} className={`application-content ${page}-content`}>
    {page !== 'chat' && <p className="layout-preview-label">{protectedDemo ? "Mock application - synthetic details" : "Layout preview - synthetic details - actions unavailable"}</p>}
    {children}
  </main>
}
