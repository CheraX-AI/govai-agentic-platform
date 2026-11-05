import LoginPage from '../pages/LoginPage'
import { useEffect, useState } from 'react'
import HomePage from '../pages/HomePage'
import RecentPage from '../pages/RecentPage'
import { ApplicationPageLayout } from '../components/layout/ApplicationPageLayout'
import RegistrationPage from '../pages/RegistrationPage'
import ForgotPasswordPage from '../pages/ForgotPasswordPage'
import ResetEmailConfirmationPage from '../pages/ResetEmailConfirmationPage'
import ResetPasswordPage from '../pages/ResetPasswordPage'
import RecoveryRequiredPage from '../pages/RecoveryRequiredPage'
import { useAuth } from '../auth/AuthContext'
import { ProtectedRoute } from '../auth/ProtectedRoute'
import UnavailablePage from '../pages/UnavailablePage'
import { navigate, readRoute, pageTitle } from './routes'
import type { RecoveryReceipt } from '../services/recoveryApi'
import { takeResetLink } from '../services/recoveryApi'

let initialResetLink = takeResetLink()

export default function AppRouter() {
  const [hash, setHash] = useState(location.hash)
  const auth = useAuth()
  const route = readRoute(hash)
  const [receipt, setReceipt] = useState<RecoveryReceipt | null>(null)
  const [resetToken, setResetToken] = useState<string | null>(initialResetLink?.token ?? null)
  const [resetComplete, setResetComplete] = useState(false)
  const [resetVersion, setResetVersion] = useState(0)
  useEffect(() => { initialResetLink = null }, [])
  const logout = async () => { await auth.logout(); navigate('#/login', true) }
  useEffect(() => {
    const onHashChange = () => {
      const incoming = takeResetLink()
      const next = location.hash
      if (incoming) { setResetToken(incoming.token); setResetComplete(false); setResetVersion(value => value + 1) }
      setHash(next)
      if (readRoute(next).path !== '#/check-email') setReceipt(null)
      if (readRoute(next).path !== '#/reset-password') { setResetToken(null); setResetComplete(false) }
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
  if (route.path === '#/login') return <LoginPage onSuccess={() => navigate(route.returnTo, true)} expired={route.expired} />
  if (route.path === '#/verify-code') return <RecoveryRequiredPage />
  if (route.path === '#/reset-password') return resetToken || resetComplete ? <ResetPasswordPage key={resetVersion} token={resetToken ?? ''} onComplete={() => { setResetToken(null); setResetComplete(true) }} /> : <RecoveryRequiredPage />
  if (route.path === '#/register') return <RegistrationPage />
  if (route.path === '#/forgot-password') return <ForgotPasswordPage onReceived={value => { setReceipt(value); navigate('#/check-email') }} />
  if (route.path === '#/check-email') return <ResetEmailConfirmationPage receipt={receipt} onReceived={setReceipt} />
  if (route.path === '#/app/home' || route.path === '#/app/recent') return <ProtectedRoute destination={route.path}><ApplicationPageLayout page={route.path === '#/app/recent' ? 'recent' : 'home'} protectedDemo onDemoLogout={() => void logout()}>{route.path === '#/app/recent' ? <RecentPage protectedDemo /> : <HomePage chatEnabled />}</ApplicationPageLayout></ProtectedRoute>
  if (['#/preview/home', '#/preview/recent', '#/home', '#/recent'].includes(route.path)) {
    if (!import.meta.env.DEV) return <UnavailablePage developmentOnly />
    return <ApplicationPageLayout page={route.path.endsWith('/recent') ? 'recent' : 'home'}>{route.path.endsWith('/recent') ? <RecentPage /> : <HomePage />}</ApplicationPageLayout>
  }
  return <UnavailablePage />
}
