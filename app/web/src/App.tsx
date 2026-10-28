import LoginPage from './pages/LoginPage'
import { useEffect, useState } from 'react'
import LayoutPreviewPage from './pages/LayoutPreviewPage'
import RegistrationPage from './pages/RegistrationPage'
import ForgotPasswordPage from './pages/ForgotPasswordPage'
import ResetEmailConfirmationPage from './pages/ResetEmailConfirmationPage'
import VerificationCodePage from './pages/VerificationCodePage'
import ResetPasswordPage from './pages/ResetPasswordPage'
import RecoveryRequiredPage from './pages/RecoveryRequiredPage'

export default function App() {
  const [hash, setHash] = useState(location.hash)
  const [demoSession, setDemoSession] = useState(false)
  const [recovery, setRecovery] = useState<'idle' | 'phone' | 'verified' | 'complete'>('idle')
  const startDemoSession = import.meta.env.DEV ? () => { setDemoSession(true); location.hash = '#/home' } : undefined
  useEffect(() => {
    const onHashChange = () => {
      const next = location.hash
      setHash(next)
      if (!['#/verify-code', '#/reset-password'].includes(next)) setRecovery('idle')
    }
    addEventListener('hashchange', onHashChange)
    return () => removeEventListener('hashchange', onHashChange)
  }, [])
  useEffect(() => {
    const pageTitle = !hash || hash === '#/login' ? 'Login preview'
      : hash === '#/forgot-password' ? 'Password recovery preview' : hash === '#/check-email' ? 'Check email preview'
      : hash === '#/reset-password' ? 'Password reset preview' : hash === '#/verify-code' ? 'Recovery code verification preview'
      : hash === '#/register' ? 'Registration preview' : hash === '#/home' ? 'Home layout preview'
        : hash === '#/recent' ? 'Recent chats layout preview' : 'Preview unavailable'
    document.title = `GovAI | ${pageTitle}`
  }, [hash])
  // Empty URL preserves the original login preview and its regression harness.
  if (!hash || hash === '#/login') return <LoginPage onDemoSuccess={startDemoSession} />
  if (hash === '#/verify-code') return recovery === 'phone' ? <VerificationCodePage onVerified={() => { setRecovery('verified'); location.hash = '#/reset-password'; setHash('#/reset-password') }} /> : <RecoveryRequiredPage />
  if (hash === '#/reset-password') return recovery === 'verified' || recovery === 'complete' ? <ResetPasswordPage onComplete={() => setRecovery('complete')} /> : <RecoveryRequiredPage />
  if (hash === '#/register') return <RegistrationPage />
  if (hash === '#/forgot-password') return <ForgotPasswordPage onPhoneRecovery={() => { setRecovery('phone'); location.hash = '#/verify-code'; setHash('#/verify-code') }} />
  if (hash === '#/check-email') return <ResetEmailConfirmationPage />
  if (hash === '#/home' || hash === '#/recent') {
    if (!import.meta.env.DEV) return <main className="unknown-preview"><h1>Development preview unavailable</h1><p>Demo Home access is disabled in production.</p><a href="#/login">Back to Login</a></main>
    return <LayoutPreviewPage page={hash === '#/recent' ? 'recent' : 'home'} demoSession={demoSession} onDemoLogout={() => { setDemoSession(false); location.hash = '#/login' }} />
  }
  return <main className="unknown-preview"><h1>Preview unavailable</h1><p>This destination is not implemented.</p><a href="#/home">Home layout preview</a></main>
}
