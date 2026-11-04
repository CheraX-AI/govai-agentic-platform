import ApplicationRoutes from './ApplicationRoutes'
import '../styles/chat.css'
import LoginPage from '../pages/LoginPage'
import { useEffect, useState } from 'react'
import HomePage from '../pages/HomePage'
import RecentPage from '../pages/RecentPage'
import { ApplicationPageLayout } from '../components/layout/ApplicationPageLayout'
import RegistrationPage from '../pages/RegistrationPage'
import ForgotPasswordPage from '../pages/ForgotPasswordPage'
import ResetEmailConfirmationPage from '../pages/ResetEmailConfirmationPage'
import VerificationCodePage from '../pages/VerificationCodePage'
import ResetPasswordPage from '../pages/ResetPasswordPage'
import RecoveryRequiredPage from '../pages/RecoveryRequiredPage'
import { useAuth } from '../auth/AuthContext'
import { ProtectedRoute } from '../auth/ProtectedRoute'
import UnavailablePage from '../pages/UnavailablePage'
import { navigate, readRoute, pageTitle } from './routes'

export default function AppRouter() {
  const [hash, setHash] = useState(location.hash)
  const auth = useAuth()
  const route = readRoute(hash)
  const [recovery, setRecovery] = useState<'idle' | 'phone' | 'verified' | 'complete'>('idle')
  const startDemoSession = import.meta.env.DEV ? async () => {
    if (await auth.startMockSession()) navigate(route.returnTo, true)
  } : undefined
  const logout = async () => { await auth.logout(); navigate('#/login', true) }
  useEffect(() => {
    const onHashChange = () => {
      const next = location.hash
      setHash(next)
      if (!['#/verify-code', '#/reset-password'].includes(readRoute(next).path)) setRecovery('idle')
    }
    addEventListener('hashchange', onHashChange)
    return () => removeEventListener('hashchange', onHashChange)
  }, [])
  useEffect(() => {
    if (import.meta.env.DEV && (route.path === '#/home' || route.path === '#/recent')) navigate(route.path === '#/home' ? '#/preview/home' : '#/preview/recent', true)
  }, [route.path])
  useEffect(() => {
    document.title = `GovAI | ${pageTitle(route.path)}`
  }, [route.path])
  // Empty URL preserves the original login preview and its regression harness.
  if (route.path === '#/login') return <LoginPage onDemoSuccess={startDemoSession} expired={route.expired} />
  if (route.path === '#/verify-code') return recovery === 'phone' ? <VerificationCodePage onVerified={() => { setRecovery('verified'); location.hash = '#/reset-password'; setHash('#/reset-password') }} /> : <RecoveryRequiredPage />
  if (route.path === '#/reset-password') return recovery === 'verified' || recovery === 'complete' ? <ResetPasswordPage onComplete={() => setRecovery('complete')} /> : <RecoveryRequiredPage />
  if (route.path === '#/register') return <RegistrationPage />
  if (route.path === '#/forgot-password') return <ForgotPasswordPage onPhoneRecovery={() => { setRecovery('phone'); location.hash = '#/verify-code'; setHash('#/verify-code') }} />
  if (route.path === '#/check-email') return <ResetEmailConfirmationPage />
  if (route.path === '#/app/home' || route.path === '#/app/recent' || route.path === '#/app/chat') return <ProtectedRoute destination={route.path}><ApplicationRoutes key={auth.state.status === 'authenticated' ? auth.state.session.createdAt : 'checking'} path={route.path} onLogout={() => void logout()} /></ProtectedRoute>
  if (['#/preview/home', '#/preview/recent', '#/home', '#/recent'].includes(route.path)) {
    if (!import.meta.env.DEV) return <UnavailablePage developmentOnly />
    return <ApplicationPageLayout page={route.path.endsWith('/recent') ? 'recent' : 'home'}>{route.path.endsWith('/recent') ? <RecentPage /> : <HomePage />}</ApplicationPageLayout>
  }
  return <UnavailablePage />
}
