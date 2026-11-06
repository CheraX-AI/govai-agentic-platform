import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { InputField } from '../components/InputField'
import { PasswordField } from '../components/PasswordField'
import { Checkbox } from '../components/Checkbox'
import { Button } from '../components/Button'
import { Icon } from '../components/Icon'
import { StatusMessage } from '../components/StatusMessage'
import { validateRegistration } from '../validation/registrationValidation'
import type { RegistrationValues } from '../validation/registrationValidation'
import { submitMockRegistration } from '../services/mockRegistrationService'
import type { MockOutcome } from '../services/mockLoginService'
import '../styles/registration.css'

const emptyValues: RegistrationValues = { fullName: '', nic: '', phone: '', email: '', password: '', confirmation: '', terms: false }
type Field = keyof RegistrationValues

export default function RegistrationPage() {
  const [values, setValues] = useState(emptyValues)
  const [touched, setTouched] = useState<Partial<Record<Field, boolean>>>({})
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const form = useRef<HTMLFormElement>(null)
  const submitting = useRef(false)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const errors = validateRegistration(values)
  const pending = status === 'pending'
  const error = (field: Field) => touched[field] ? errors[field] : undefined
  function change<K extends Field>(field: K, value: RegistrationValues[K]) {
    setValues(previous => ({ ...previous, [field]: value }))
    setStatus('idle')
  }
  function blur(field: Field) { setTouched(previous => ({ ...previous, [field]: true })) }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched(Object.fromEntries(Object.keys(emptyValues).map(field => [field, true])))
    setStatus('idle')
    const invalid = (Object.keys(errors) as Field[]).find(field => errors[field])
    if (invalid) { form.current?.querySelector<HTMLInputElement>(`#register-${invalid}`)?.focus(); return }
    submitting.current = true
    setStatus('pending')
    try {
      const result = await submitMockRegistration(outcome)
      if (mounted.current) setStatus(result)
    } catch {
      if (mounted.current) setStatus('error')
    } finally {
      if (mounted.current) {
        setValues(previous => ({ ...previous, password: '', confirmation: '' }))
        setTouched(previous => ({ ...previous, password: false, confirmation: false }))
      }
      submitting.current = false
    }
  }

  return <AuthLayout headingId="registration-heading" wide formProps={{ ref: form, onSubmit: submit, noValidate: true }}
    footerControls={<details className="demo-options registration-demo-details"><summary>Demo information and controls</summary>
      <p>This is a local interface demonstration. No account is created, no SMS or email is sent, and no session is established.</p>
      <p>The demo password policy counts Unicode code points and requires an ASCII digit plus a Unicode punctuation or symbol character. Whitespace is not a symbol. Passwords are not trimmed or truncated. This policy must be aligned with the real authentication system.</p>
      <p>Legal content is unavailable. The required demo-only checkbox records no legal agreement.</p>
      <label htmlFor="register-outcome">Simulated result</label><select id="register-outcome" value={outcome} disabled={pending} onChange={event => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select>
    </details>}>
    <div className="heading"><h1 id="registration-heading">Create your account</h1><p>Enter the details exactly as they appear on your NIC.</p></div>
    <InputField id="register-fullName" label="Full name (as on NIC)" icon="register-user" autoComplete="name" required value={values.fullName} disabled={pending} error={error('fullName')} onChange={event => change('fullName', event.target.value)} onBlur={() => blur('fullName')} />
    <div className="registration-row">
      <InputField id="register-nic" label="NIC number" icon="register-nic" autoCapitalize="none" spellCheck={false} required value={values.nic} disabled={pending} error={error('nic')} onChange={event => change('nic', event.target.value)} onBlur={() => blur('nic')} />
      <InputField id="register-phone" label="Mobile number" icon="phone" type="tel" inputMode="tel" autoComplete="tel" required value={values.phone} disabled={pending} error={error('phone')} onChange={event => change('phone', event.target.value)} onBlur={() => blur('phone')} />
    </div>
    <InputField id="register-email" label="Email address" icon="mail" type="text" inputMode="email" autoComplete="email" autoCapitalize="none" spellCheck={false} required value={values.email} disabled={pending} error={error('email')} onChange={event => change('email', event.target.value)} onBlur={() => blur('email')} />
    <div className="registration-row">
      <PasswordField id="register-password" autoComplete="new-password" aria-describedby="registration-password-policy" required value={values.password} disabled={pending} error={error('password')} onChange={event => change('password', event.target.value)} onBlur={() => blur('password')} />
      <PasswordField id="register-confirmation" label="Confirm password" autoComplete="new-password" required value={values.confirmation} disabled={pending} error={error('confirmation')} onChange={event => change('confirmation', event.target.value)} onBlur={() => blur('confirmation')} />
    </div>
    <p id="registration-password-policy" className="registration-policy"><img src="/assets/figma/login/register-info.svg" alt="" />8+ code points, 0-9, and punctuation/symbol (not whitespace).</p>
    <div><Checkbox id="register-terms" required checked={values.terms} disabled={pending} aria-invalid={error('terms') ? true : undefined} aria-describedby={error('terms') ? 'registration-legal register-terms-error' : 'registration-legal'} onChange={event => change('terms', event.target.checked)}>I agree (demo only).</Checkbox>
      <p id="registration-legal" className="registration-legal">Terms of use and Privacy notice unavailable.</p>
      {error('terms') && <p id="register-terms-error" className="field-error">{error('terms')}</p>}
    </div>
    <Button type="submit" pending={pending}>{pending ? 'Simulating registration...' : 'Create account'}{!pending && <Icon name="login-arrow-right" />}</Button>
    {status === 'pending' && <StatusMessage kind="pending">Simulating registration only...</StatusMessage>}
    {status === 'success' && <><StatusMessage kind="success">Demo registration successful. No account was created.</StatusMessage><a className="primary-button registration-login-action" href="#/login">Go to login</a></>}
    {status === 'error' && <StatusMessage kind="error">Demo registration failed. Please try again. No account was created.</StatusMessage>}
    <p className="create-account">Already have an account? <a href="#/login">Back to login</a></p>
  </AuthLayout>
}
