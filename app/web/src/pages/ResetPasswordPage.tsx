import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { PasswordField } from '../components/PasswordField'
import { Button } from '../components/Button'
import { StatusMessage } from '../components/StatusMessage'
import { validateDemoPasswords } from '../validation/registrationValidation'
import { submitMockPasswordReset } from '../services/mockPasswordResetService'
import type { MockOutcome } from '../services/mockLoginService'

export default function ResetPasswordPage({ onComplete }: { onComplete: () => void }) {
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [touched, setTouched] = useState({ password: false, confirmation: false })
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const mounted = useRef(false)
  const inFlight = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const errors = validateDemoPasswords(password, confirmation)
  const pending = status === 'pending'
  const complete = status === 'success'
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (inFlight.current || complete) return
    setStatus('idle'); setTouched({ password: true, confirmation: true })
    if (errors.password || errors.confirmation) { document.getElementById(errors.password ? 'reset-password' : 'reset-confirmation')?.focus(); return }
    inFlight.current = true; setStatus('pending')
    try {
      const result = await submitMockPasswordReset(outcome)
      if (!mounted.current) return
      setStatus(result)
      if (result === 'success') onComplete()
    } catch { if (mounted.current) setStatus('error') }
    finally {
      if (mounted.current) { setPassword(''); setConfirmation(''); setTouched({ password: false, confirmation: false }) }
      inFlight.current = false
    }
  }
  return <AuthLayout headingId="reset-heading" formProps={{ noValidate: true, onSubmit: submit, 'aria-busy': pending }} footerControls={<details className="demo-options"><summary>Demo reset controls</summary><p>Provisional UI adaptation. The registration demo policy is reused and must be aligned with the real system in GA-54. Passwords are preserved exactly and never sent or stored.</p><label htmlFor="reset-outcome">Simulated reset result</label><select id="reset-outcome" value={outcome} disabled={pending || complete} onChange={event => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select></details>}>
    <div className="heading"><h1 id="reset-heading" tabIndex={-1} ref={heading}>Choose a new password</h1><p>Provisional password-reset interface preview.</p></div>
    <p className="status-message">Demo only - no password will be changed.</p>
    <PasswordField id="reset-password" label="New password" required autoComplete="new-password" aria-describedby="reset-policy" value={password} disabled={pending || complete} error={touched.password ? errors.password : undefined} onChange={event => { setPassword(event.target.value); setStatus('idle') }} onBlur={() => setTouched(previous => ({ ...previous, password: true }))} />
    <PasswordField id="reset-confirmation" label="Confirm new password" required autoComplete="new-password" value={confirmation} disabled={pending || complete} error={touched.confirmation ? errors.confirmation : undefined} onChange={event => { setConfirmation(event.target.value); setStatus('idle') }} onBlur={() => setTouched(previous => ({ ...previous, confirmation: true }))} />
    <p className="create-account" id="reset-policy">8+ Unicode code points, an ASCII digit (0-9), and punctuation/symbol (not whitespace). Confirmation must match exactly.</p>
    <Button type="submit" pending={pending} disabled={complete}>Simulate password reset</Button>
    {pending && <StatusMessage kind="pending">Simulating reset only...</StatusMessage>}
    {complete && <><StatusMessage kind="success">Demo reset successful. No password was changed.</StatusMessage><a className="primary-button" href="#/login">Go to login</a></>}
    {status === 'error' && <StatusMessage kind="error">Demo reset failed. Please try again. No password was changed.</StatusMessage>}
    <a href="#/forgot-password">Start recovery again</a>
    <a href="#/login">Back to login</a>
  </AuthLayout>
}
