import LoginPage from './pages/LoginPage'
import { useEffect, useState } from 'react'
import LayoutPreviewPage from './pages/LayoutPreviewPage'

export default function App() {
  const [hash, setHash] = useState(location.hash)
  useEffect(() => {
    const onHashChange = () => setHash(location.hash)
    addEventListener('hashchange', onHashChange)
    return () => removeEventListener('hashchange', onHashChange)
  }, [])
  useEffect(() => {
    const pageTitle = !hash || hash === '#/login' ? 'Login preview'
      : hash === '#/home' ? 'Home layout preview'
        : hash === '#/recent' ? 'Recent chats layout preview' : 'Preview unavailable'
    document.title = `GovAI | ${pageTitle}`
  }, [hash])
  // Empty URL preserves the original login preview and its regression harness.
  if (!hash || hash === '#/login') return <LoginPage />
  if (hash === '#/home' || hash === '#/recent') return <LayoutPreviewPage page={hash === '#/recent' ? 'recent' : 'home'} />
  return <main className="unknown-preview"><h1>Preview unavailable</h1><p>This destination is not implemented.</p><a href="#/home">Home layout preview</a></main>
}
