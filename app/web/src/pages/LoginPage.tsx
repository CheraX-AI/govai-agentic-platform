import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Icon } from '../components/Icon'
import { AuthLayout } from '../components/auth/AuthLayout'
import { Button } from '../components/Button'
import { LoginCredentials } from '../components/auth/LoginCredentials'
import { Checkbox } from '../components/Checkbox'
import { StatusMessage } from '../components/StatusMessage'
import { validateNicEmailLogin } from '../validation/loginValidation'
import { useAuth } from '../auth/AuthContext'

export default function LoginPage({ onSuccess, expired = false }: { onSuccess?: () => void; expired?: boolean }) {
  const auth = useAuth()
  const method = 'nic-email' as const
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [humanCheck, setHumanCheck] = useState(false)
  const [touched, setTouched] = useState({ identifier: false, password: false, humanCheck: false })
  const [failure, setFailure] = useState('')
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const submitting = useRef(false)
  const form = useRef<HTMLFormElement>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const errors = validateNicEmailLogin(identifier, password, true)

  const pending = status === 'pending' || auth.state.status === 'checking'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched({ identifier: true, password: true, humanCheck: true })
    setStatus('idle')
    if (Object.values(errors).some(Boolean)) {
      const firstInvalid = errors.identifier ? '#identifier' : errors.password ? '#password' : '#human-check'
      form.current?.querySelector<HTMLInputElement>(firstInvalid)?.focus()
      return
    }
    if (method === 'nic-email') setIdentifier(identifier.trim())
    submitting.current = true
    setStatus('pending')
    try {
      if (await auth.login(identifier.trim(), password)) {
        if (mounted.current) { setStatus('success'); onSuccess?.() }
      }
    } catch (error) {
      if (mounted.current) { setFailure(error instanceof Error ? error.message : 'Login unavailable.'); setStatus('error') }
    } finally {
      if (mounted.current) {
        setPassword('')
        setTouched((previous) => ({ ...previous, password: false }))
      }
      submitting.current = false
    }
  }

  return <AuthLayout headingId="login-heading" formProps={{ ref: form, onSubmit: submit, noValidate: true }}
    banner="University prototype. Use test details only."
    footerControls={<p>Secure email or NIC login. Phone login is unavailable.</p>}>
          <div className="heading"><h1 id="login-heading" tabIndex={-1} ref={heading}>Welcome back</h1><p>Log in to use government services with GovAI.</p></div>
          {expired && <StatusMessage kind="error">Your session expired. Please log in again.</StatusMessage>}
          {auth.state.status === 'error' && <div><StatusMessage kind="error">Session check failed. No authentication was confirmed.</StatusMessage><button type="button" onClick={() => void auth.retry()}>Retry session check</button></div>}
          <LoginCredentials method={method} identifier={identifier} password={password} pending={pending}
            identifierError={touched.identifier ? errors.identifier : undefined} passwordError={touched.password ? errors.password : undefined}
            onIdentifierBlur={() => { if (method === 'nic-email') setIdentifier(identifier.trim()); setTouched(previous => ({ ...previous, identifier: true })) }}
            onIdentifierChange={value => { setIdentifier(value); setStatus('idle') }}
            onPasswordBlur={() => setTouched(previous => ({ ...previous, password: true }))}
            onPasswordChange={value => { setPassword(value); setStatus('idle') }} />
          <div className="options login-recovery-link"><a href="#/forgot-password">Forgot password?</a></div>
          <div className="human-check">
            <Checkbox id="human-check" checked={humanCheck} disabled
              aria-invalid={touched.humanCheck && errors.humanCheck ? true : undefined}
              aria-describedby={touched.humanCheck && errors.humanCheck ? 'human-note human-error' : 'human-note'}
              onChange={(event) => { setHumanCheck(event.target.checked); setStatus('idle') }}>Human verification unavailable</Checkbox>
            <div className="security-badge"><Icon name="shield-check" /><span>Demo check</span></div>
          </div>
          <p className="helper" id="human-note">Simulated checkbox only - no security verification.</p>
          {touched.humanCheck && errors.humanCheck && <p id="human-error" className="field-error">{errors.humanCheck}</p>}
          <Button type="submit" pending={pending}>{pending ? 'Logging in…' : 'Log in'}{!pending && <Icon name="login-arrow-right" />}</Button>
          {status === 'pending' && <StatusMessage kind="pending">Signing in securely…</StatusMessage>}
          {status === 'success' && <StatusMessage kind="success">Signed in successfully.</StatusMessage>}
          {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
          <p className="create-account">New to GovAI? <a href="#/register">Create an account</a></p>

  </AuthLayout>
}
