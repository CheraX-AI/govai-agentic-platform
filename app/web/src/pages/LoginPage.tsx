import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Icon } from '../components/Icon'
import { AuthLayout } from '../components/auth/AuthLayout'
import { Button } from '../components/Button'
import { LoginCredentials } from '../components/auth/LoginCredentials'
import { LoginMethodSwitch } from '../components/auth/LoginMethodSwitch'
import type { LoginMethod } from '../components/auth/LoginMethodSwitch'
import { Checkbox } from '../components/Checkbox'
import { StatusMessage } from '../components/StatusMessage'
import { validateLogin, validateNicEmailLogin } from '../validation/loginValidation'
import { submitMockLogin } from '../services/mockLoginService'
import type { MockOutcome } from '../services/mockLoginService'

export default function LoginPage({ onDemoSuccess }: { onDemoSuccess?: () => void }) {
  const [method, setMethod] = useState<LoginMethod>('phone')
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [humanCheck, setHumanCheck] = useState(false)
  const [touched, setTouched] = useState({ identifier: false, password: false, humanCheck: false })
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const submitting = useRef(false)
  const form = useRef<HTMLFormElement>(null)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const phoneErrors = validateLogin(identifier, password, humanCheck)
  const errors = method === 'phone' ? { identifier: phoneErrors.phone, password: phoneErrors.password, humanCheck: phoneErrors.humanCheck }
    : validateNicEmailLogin(identifier, password, humanCheck)

  function changeMethod(next: LoginMethod) {
    if (submitting.current || next === method) return
    setMethod(next)
    setIdentifier('')
    setPassword('')
    setHumanCheck(false)
    setTouched({ identifier: false, password: false, humanCheck: false })
    setStatus('idle')
  }
  const pending = status === 'pending'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched({ identifier: true, password: true, humanCheck: true })
    setStatus('idle')
    if (Object.values(errors).some(Boolean)) {
      const firstInvalid = errors.identifier ? (method === 'phone' ? '#phone' : '#identifier') : errors.password ? '#password' : '#human-check'
      form.current?.querySelector<HTMLInputElement>(firstInvalid)?.focus()
      return
    }
    if (method === 'nic-email') setIdentifier(identifier.trim())
    submitting.current = true
    setStatus('pending')
    try {
      const result = await submitMockLogin(outcome)
      if (mounted.current) {
        setStatus(result)
        if (result === 'success') {
          if (import.meta.env.DEV) onDemoSuccess?.()
        }
      }
    } catch {
      if (mounted.current) setStatus('error')
    } finally {
      if (mounted.current) {
        setPassword('')
        setTouched((previous) => ({ ...previous, password: false }))
      }
      submitting.current = false
    }
  }

  return <AuthLayout headingId="login-heading" formProps={{ ref: form, onSubmit: submit, noValidate: true }}
    footerControls={<details className="demo-options"><summary>Demo response controls</summary><label htmlFor="outcome">Simulated result</label><select id="outcome" disabled={pending} value={outcome} onChange={(event) => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select></details>}>
          <div className="heading"><h1 id="login-heading">Welcome back</h1><p>Log in to use government services with GovAI.</p></div>
          <LoginMethodSwitch method={method} disabled={pending} onChange={changeMethod} />
          <LoginCredentials method={method} identifier={identifier} password={password} pending={pending}
            identifierError={touched.identifier ? errors.identifier : undefined} passwordError={touched.password ? errors.password : undefined}
            onIdentifierBlur={() => { if (method === 'nic-email') setIdentifier(identifier.trim()); setTouched(previous => ({ ...previous, identifier: true })) }}
            onIdentifierChange={value => { setIdentifier(value); setStatus('idle') }}
            onPasswordBlur={() => setTouched(previous => ({ ...previous, password: true }))}
            onPasswordChange={value => { setPassword(value); setStatus('idle') }} />
          <div className="options"><Checkbox disabled>Remember me <small>(unavailable)</small></Checkbox><a href="#/forgot-password">Forgot password?</a></div>
          <div className="human-check">
            <Checkbox id="human-check" checked={humanCheck} disabled={pending}
              aria-invalid={touched.humanCheck && errors.humanCheck ? true : undefined}
              aria-describedby={touched.humanCheck && errors.humanCheck ? 'human-note human-error' : 'human-note'}
              onChange={(event) => { setHumanCheck(event.target.checked); setStatus('idle') }}>I’m not a robot</Checkbox>
            <div className="security-badge"><Icon name="shield-check" /><span>Demo check</span></div>
          </div>
          <p className="helper" id="human-note">Simulated checkbox only - no security verification.</p>
          {touched.humanCheck && errors.humanCheck && <p id="human-error" className="field-error">{errors.humanCheck}</p>}
          <Button type="submit" pending={pending}>{pending ? 'Simulating…' : 'Log in'}{!pending && <Icon name="login-arrow-right" />}</Button>
          {status === 'pending' && <StatusMessage kind="pending">Simulating a response…</StatusMessage>}
          {status === 'success' && <StatusMessage kind="success">Demo submission successful. You are not signed in.</StatusMessage>}
          {status === 'error' && <StatusMessage kind="error">Demo login failed. Please try again.</StatusMessage>}
          <p className="create-account">New to GovAI? <a href="#/register">Create an account (demo)</a></p>

  </AuthLayout>
}
