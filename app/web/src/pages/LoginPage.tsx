import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Icon } from '../components/Icon'
import { AuthLayout } from '../components/auth/AuthLayout'
import { Button } from '../components/Button'
import { LoginCredentials } from '../components/auth/LoginCredentials'
import { StatusMessage } from '../components/StatusMessage'
import { validateNicEmailLogin } from '../validation/loginValidation'
import { useAuth } from '../auth/AuthContext'

export default function LoginPage({ onSuccess, expired = false }: {
  onSuccess?: () => void
  expired?: boolean
}) {
  const auth = useAuth()
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [touched, setTouched] = useState({ identifier: false, password: false })
  const [failure, setFailure] = useState('')
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const submitting = useRef(false)
  const form = useRef<HTMLFormElement>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const mounted = useRef(false)

  useEffect(() => {
    mounted.current = true
    heading.current?.focus()
    return () => { mounted.current = false }
  }, [])
  const errors = validateNicEmailLogin(identifier, password)

  // A failed session check still leaves normal login available; checking does not.

  const pending = status === 'pending' || auth.state.status === 'checking'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched({ identifier: true, password: true })
    setStatus('idle')
    if (errors.identifier || errors.password) {
      form.current?.querySelector<HTMLInputElement>(
        errors.identifier ? '#identifier' : '#password',
      )?.focus()
      return
    }
    setIdentifier(identifier.trim())
    submitting.current = true
    setStatus('pending')
    try {
      if (await auth.login(identifier.trim(), password)) {

        // Navigation may have unmounted the form while authentication was pending.

        if (mounted.current) {
          setStatus('success')
          onSuccess?.()
        }
      }
    } catch (error) {
      if (mounted.current) {
        setFailure(error instanceof Error ? error.message : 'Login unavailable.')
        setStatus('error')
      }
    } finally {
      if (mounted.current) {
        setPassword('')
        setTouched((previous) => ({ ...previous, password: false }))
      }
      submitting.current = false
    }
  }

  return (
    <AuthLayout
      headingId="login-heading"
      formProps={{ ref: form, onSubmit: submit, noValidate: true }}
      banner="University prototype. Use test details only."
      footerControls={<p>Log in with your email or NIC number.</p>}
    >
      <div className="heading">
        <h1 id="login-heading" tabIndex={-1} ref={heading}>Welcome back</h1>
        <p>Log in to use government services with GovAI.</p>
      </div>
      {expired && (
        <StatusMessage kind="error">Your session expired. Please log in again.</StatusMessage>
      )}
      {auth.state.status === 'error' && (
        <div>
          <StatusMessage kind="error">
            Session check failed. No authentication was confirmed.
          </StatusMessage>
        </div>
      )}
      <LoginCredentials
        identifier={identifier}
        password={password}
        pending={pending}
        identifierError={touched.identifier ? errors.identifier : undefined}
        passwordError={touched.password ? errors.password : undefined}
        onIdentifierBlur={() => {
          setIdentifier(identifier.trim())
          setTouched(previous => ({ ...previous, identifier: true }))
        }}
        onIdentifierChange={value => {
          setIdentifier(value)
          setStatus('idle')
        }}
        onPasswordBlur={() => setTouched(previous => ({ ...previous, password: true }))}
        onPasswordChange={value => {
          setPassword(value)
          setStatus('idle')
        }}
      />
      <div className="options login-recovery-link">
        <a href="/forgot-password">Forgot password?</a>
      </div>
      <Button type="submit" pending={pending}>
        {pending ? 'Logging in…' : 'Log in'}
        {!pending && <Icon name="login-arrow-right" />}
      </Button>
      {status === 'pending' && (
        <StatusMessage kind="pending">Signing in securely…</StatusMessage>
      )}
      {status === 'success' && (
        <StatusMessage kind="success">Signed in successfully.</StatusMessage>
      )}
      {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
      <p className="create-account">New to GovAI? <a href="/register">Create an account</a></p>
    </AuthLayout>
  )
}
