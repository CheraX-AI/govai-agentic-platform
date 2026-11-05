import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { PasswordField } from '../components/PasswordField'
import { Button } from '../components/Button'
import { StatusMessage } from '../components/StatusMessage'
import { resetPassword, validateResetPasswords } from '../services/recoveryApi'

export default function ResetPasswordPage({ token, onComplete }: { token: string; onComplete: () => void }) {
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [touched, setTouched] = useState({ password: false, confirmation: false })
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const [failure, setFailure] = useState('')
  const mounted = useRef(false)
  const inFlight = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const errors = validateResetPasswords(password, confirmation)
  const pending = status === 'pending'
  const complete = status === 'success'
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (inFlight.current || complete) return
    setStatus('idle'); setTouched({ password: true, confirmation: true })
    if (errors.password || errors.confirmation) { document.getElementById(errors.password ? 'reset-password' : 'reset-confirmation')?.focus(); return }
    inFlight.current = true; setStatus('pending')
    try {
      await resetPassword(token, password, confirmation)
      if (!mounted.current) return
      setStatus('success'); onComplete()
    } catch (error) { if (mounted.current) { setFailure(error instanceof Error ? error.message : 'Unable to confirm the password change.'); setStatus('error') } }
    finally {
      if (mounted.current) { setPassword(''); setConfirmation(''); setTouched({ password: false, confirmation: false }) }
      inFlight.current = false
    }
  }
  return <AuthLayout headingId="reset-heading" formProps={{ noValidate: true, onSubmit: submit, 'aria-busy': pending }} banner="University prototype. Use test details only." footerControls={<p>Your password is sent securely to the GovAI backend and managed by Keycloak. It is not stored in this browser.</p>}>
    <div className="heading"><h1 id="reset-heading" tabIndex={-1} ref={heading}>Choose a new password</h1><p>Enter your new password.</p></div>
    <p className="status-message">Use the newest reset link from your email.</p>
    <PasswordField id="reset-password" label="New password" required autoComplete="new-password" aria-describedby="reset-policy" value={password} disabled={pending || complete} error={touched.password ? errors.password : undefined} onChange={event => { setPassword(event.target.value); setStatus('idle') }} onBlur={() => setTouched(previous => ({ ...previous, password: true }))} />
    <PasswordField id="reset-confirmation" label="Confirm new password" required autoComplete="new-password" value={confirmation} disabled={pending || complete} error={touched.confirmation ? errors.confirmation : undefined} onChange={event => { setConfirmation(event.target.value); setStatus('idle') }} onBlur={() => setTouched(previous => ({ ...previous, confirmation: true }))} />
    <p className="create-account" id="reset-policy">At least 8 characters. Confirmation must match exactly.</p>
    <Button type="submit" pending={pending} disabled={complete}>Reset password</Button>
    {pending && <StatusMessage kind="pending">Updating your password...</StatusMessage>}
    {complete && <><StatusMessage kind="success">Password updated. Log in with your new password.</StatusMessage><a className="primary-button" href="/login">Go to login</a></>}
    {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
    <a href="/forgot-password">Start recovery again</a>
    <a href="/login">Back to login</a>
  </AuthLayout>
}
