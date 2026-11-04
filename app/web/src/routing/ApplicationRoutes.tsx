import { useConversation } from '../chat/useConversation'
import { ApplicationPageLayout } from '../components/layout/ApplicationPageLayout'
import ChatPage from '../pages/ChatPage'
import HomePage from '../pages/HomePage'
import RecentPage from '../pages/RecentPage'
import { navigate } from './routes'
import type { ProtectedDestination } from './routes'

// Mounted only inside ProtectedRoute, keyed to one authenticated session.
// Navigation keeps the hook alive; logout/expiry/reload unmount and cancel it.
export default function ApplicationRoutes({ path, onLogout }: { path: ProtectedDestination; onLogout: () => void }) {
  const conversation = useConversation()
  const openChat = () => navigate('#/app/chat')
  const newChat = () => { conversation.reset(); openChat() }
  const page = path === '#/app/chat' ? 'chat' : path === '#/app/recent' ? 'recent' : 'home'
  return <ApplicationPageLayout page={page} protectedDemo onNewChat={newChat} onDemoLogout={onLogout}>
    {page === 'chat' ? <ChatPage conversation={conversation} /> : page === 'recent' ? <RecentPage protectedDemo /> : <HomePage conversation={conversation} onOpenChat={openChat} />}
  </ApplicationPageLayout>
}
