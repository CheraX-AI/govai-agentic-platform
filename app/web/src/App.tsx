import { lazy, Suspense } from 'react'
import UnavailablePage from './pages/UnavailablePage'
import { AuthProvider } from './auth/AuthProvider'
import AppRouter from './routing/AppRouter'

// Vite eliminates this import and its synthetic service from production builds.

const ChatPreview = import.meta.env.DEV ? lazy(() => import('./dev/ChatPreview')) : null

export default function App() {

  // Bypass session mounting only for the explicitly isolated development preview.

  if (location.pathname === '/preview/chat') {
    return ChatPreview ? (
      <Suspense fallback={<p role="status">Loading chat preview...</p>}>
        <ChatPreview />
      </Suspense>
    ) : <UnavailablePage />
  }
  return <AuthProvider><AppRouter /></AuthProvider>
}
