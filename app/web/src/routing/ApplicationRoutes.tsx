import { useLayoutEffect, useRef } from 'react'
import { useAuth } from '../auth/AuthContext'
import { ProtectedRoute } from '../auth/ProtectedRoute'
import { useConversation } from '../chat/useConversation'
import { ApplicationPageLayout } from '../components/layout/ApplicationPageLayout'
import ChatPage from '../pages/ChatPage'
import HomePage from '../pages/HomePage'
import RecentPage from '../pages/RecentPage'
import { navigate } from './routes'
import type { ProtectedDestination } from './routes'
import '../styles/chat.css'

// This owner stays mounted across protected pages, so local messages survive navigation.

export default function ApplicationRoutes({ path, onLogout }: {
  path: ProtectedDestination
  onLogout: () => void
}) {
  const auth = useAuth()
  const conversation = useConversation(undefined, () => { void auth.retry() })
  const { reset } = conversation
  const owner = useRef<string | null>(null)

  // Keep context during a check; ProtectedRoute withholds it until confirmation.
  // Clearing before paint prevents a new identity seeing the previous user's messages.

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
  const page = path === '/app/chat' ? 'chat' : path === '/app/recent' ? 'recent' : 'home'

  return (
    <ProtectedRoute destination={path}>
      <ApplicationPageLayout
        page={page}
        onLogout={onLogout}
        onNewChat={newChat}
        conversationAvailable={conversation.messages.length > 0}
      >
        {page === 'chat' ? (
          <ChatPage conversation={conversation} />
        ) : page === 'recent' ? (
          <RecentPage conversationAvailable={conversation.messages.length > 0} />
        ) : (
          <HomePage conversation={conversation} onOpenChat={openChat} />
        )}
      </ApplicationPageLayout>
    </ProtectedRoute>
  )
}
