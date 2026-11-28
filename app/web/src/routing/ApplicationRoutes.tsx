import { useLayoutEffect, useRef } from 'react'
import { useAuth } from '../auth/AuthContext'
import { ProtectedRoute } from '../auth/ProtectedRoute'
import { useConversation } from '../chat/useConversation'
import { ApplicationPageLayout } from '../components/layout/ApplicationPageLayout'
import { NotificationProvider } from '../notifications/NotificationProvider'
import ChatPage from '../pages/ChatPage'
import HomePage from '../pages/HomePage'
import RecentPage from '../pages/RecentPage'
import NotificationsPage from '../pages/NotificationsPage'
import NotificationSettingsPage from '../pages/NotificationSettingsPage'
import { navigate } from './routes'
import type { ProtectedDestination } from './routes'
import '../styles/chat.css'

// Shared conversation ownership stays unchanged across the protected destinations.

export default function ApplicationRoutes({ path, onLogout }: {
  path: ProtectedDestination
  onLogout: () => void
}) {
  const auth = useAuth()
  const conversation = useConversation(undefined, () => { void auth.retry() })
  const { reset } = conversation
  const owner = useRef<string | null>(null)

  useLayoutEffect(() => {
    if (auth.state.status === 'authenticated') {
      if (owner.current !== null && owner.current !== auth.state.session.userId) reset()
      owner.current = auth.state.session.userId
    } else if (auth.state.status !== 'checking') {
      owner.current = null
      reset()
    }
  }, [auth.state, reset])

  const openChat = () => navigate('/app/chat')
  const newChat = () => {
    reset()
    navigate('/app/home')
  }
  const page = path === '/app/notifications/settings' ? 'notification-settings'
    : path === '/app/notifications' ? 'notifications'
      : path === '/app/chat' ? 'chat' : path === '/app/recent' ? 'recent' : 'home'
  const identity = auth.state.status === 'authenticated' ? auth.state.session.userId : 'none'

  return (
    <ProtectedRoute destination={path}>

      {/* Remount on identity changes so one citizen cannot retain another's inbox. */}

      <NotificationProvider key={identity}>
        <ApplicationPageLayout
          page={page}
          onLogout={onLogout}
          onNewChat={newChat}
          conversationAvailable={conversation.messages.length > 0}
        >
          {page === 'notifications' ? <NotificationsPage />
            : page === 'notification-settings' ? <NotificationSettingsPage />
              : page === 'chat' ? <ChatPage conversation={conversation} />
                : page === 'recent' ? (
                  <RecentPage conversationAvailable={conversation.messages.length > 0} />
                ) : <HomePage conversation={conversation} onOpenChat={openChat} />}
        </ApplicationPageLayout>
      </NotificationProvider>
    </ProtectedRoute>
  )
}
