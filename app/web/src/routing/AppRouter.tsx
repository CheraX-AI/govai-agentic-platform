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
import { handleLinkClick, navigate, readRoute, pageTitle, upgradeHashLink } from './routes'
import type { RecoveryReceipt } from '../services/recoveryApi'
import { takeResetLink } from '../services/recoveryApi'

upgradeHashLink()
let initialResetLink = takeResetLink()

export default function AppRouter() {
  const [location_, setLocation] = useState({ pathname: location.pathname, search: location.search })
  const auth = useAuth()
  const route = readRoute(location_.pathname, location_.search)
  const [receipt, setReceipt] = useState<RecoveryReceipt | null>(null)
  const [resetToken, setResetToken] = useState<string | null>(initialResetLink?.token ?? null)
  const [resetComplete, setResetComplete] = useState(false)
  const [resetVersion, setResetVersion] = useState(0)
  useEffect(() => { initialResetLink = null }, [])
  const logout = async () => { await auth.logout(); navigate('/login', true) }
  useEffect(() => {
    const onLocationChange = () => {
      // A new reset link opened in the same tab only changes the #token part (hashchange)
      const incoming = takeResetLink()
      if (incoming) { setResetToken(incoming.token); setResetComplete(false); setResetVersion(value => value + 1) }
      const next = { pathname: location.pathname, search: location.search }
      setLocation(next)
      const path = readRoute(next.pathname, next.search).path
      if (path !== '/check-email') setReceipt(null)
      if (path !== '/reset-password') { setResetToken(null); setResetComplete(false) }
    }
    addEventListener('popstate', onLocationChange)
    addEventListener('hashchange', onLocationChange)
    document.addEventListener('click', handleLinkClick)
    return () => {
      removeEventListener('popstate', onLocationChange)
      removeEventListener('hashchange', onLocationChange)
      document.removeEventListener('click', handleLinkClick)
    }
  }, [])
  useEffect(() => {
    // Short links go to the signed-in pages (ProtectedRoute sends guests to login)
    if (route.path === '/home' || route.path === '/recent') navigate(route.path === '/home' ? '/app/home' : '/app/recent', true)
  }, [route.path])
  useEffect(() => {
    document.title = `GovAI | ${pageTitle(route.path)}`
  }, [route.path])
  // The site root opens login.
  if (route.path === '/login') return <LoginPage onSuccess={() => navigate(route.returnTo, true)} expired={route.expired} />
  if (route.path === '/verify-code') return <RecoveryRequiredPage />
  if (route.path === '/reset-password') return resetToken || resetComplete ? <ResetPasswordPage key={resetVersion} token={resetToken ?? ''} onComplete={() => { setResetToken(null); setResetComplete(true) }} /> : <RecoveryRequiredPage />
  if (route.path === '/register') return <RegistrationPage />
  if (route.path === '/forgot-password') return <ForgotPasswordPage onReceived={value => { setReceipt(value); navigate('/check-email') }} />
  if (route.path === '/check-email') return <ResetEmailConfirmationPage receipt={receipt} onReceived={setReceipt} />
  if (route.path === '/app/home' || route.path === '/app/recent') return <ProtectedRoute destination={route.path}><ApplicationPageLayout page={route.path === '/app/recent' ? 'recent' : 'home'} onLogout={() => void logout()}>{route.path === '/app/recent' ? <RecentPage /> : <HomePage />}</ApplicationPageLayout></ProtectedRoute>
  if (route.path === '/home' || route.path === '/recent') return null
  return <UnavailablePage />
}
