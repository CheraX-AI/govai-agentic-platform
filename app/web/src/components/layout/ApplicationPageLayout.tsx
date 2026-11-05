import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { ApplicationLayout } from './ApplicationLayout'
import type { ApplicationPage } from './Sidebar'

export function ApplicationPageLayout({
  page,
  onLogout,
  children,
  onNewChat,
  conversationAvailable,
}: {
  page: ApplicationPage
  onLogout: () => void
  children: ReactNode
  onNewChat?: () => void
  conversationAvailable?: boolean
}) {

  // Path navigation does not reload the document, so move focus to its new content.

  useEffect(() => {
    document.getElementById('application-content')?.focus()
  }, [page])

  return (
    <ApplicationLayout
      page={page}
      onLogout={onLogout}
      onNewChat={onNewChat}
      conversationAvailable={conversationAvailable}
    >
      {children}
    </ApplicationLayout>
  )
}
