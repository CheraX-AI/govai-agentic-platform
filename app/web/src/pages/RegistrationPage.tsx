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
import { registerCitizen } from '../services/registrationApi'
import { normalizeRegistrationPhone } from '../validation/registrationValidation'
import '../styles/registration.css'

const emptyValues: RegistrationValues = { fullName: '', nic: '', phone: '', email: '', password: '', confirmation: '', terms: false }
type Field = keyof RegistrationValues

export default function RegistrationPage() {
  const [values, setValues] = useState(emptyValues)
  const [touched, setTouched] = useState<Partial<Record<Field, boolean>>>({})
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const [failure, setFailure] = useState('')
  const operation = useRef<string | null>(null)
  const form = useRef<HTMLFormElement>(null)
  const submitting = useRef(false)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const errors = validateRegistration(values)
  const pending = status === 'pending'
  const error = (field: Field) => touched[field] ? errors[field] : undefined
  function change<K extends Field>(field: K, value: RegistrationValues[K]) {
    operation.current = null
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
      operation.current ??= crypto.randomUUID()
      await registerCitizen({ full_name: values.fullName.trim(), nic: values.nic.trim().toUpperCase(), mobile_number: normalizeRegistrationPhone(values.phone)!, email: values.email.trim(), password: values.password }, operation.current)
      if (mounted.current) setStatus('success')
    } catch (error) {
      if (mounted.current) { setFailure(error instanceof Error ? error.message : 'Registration is temporarily unavailable.'); setStatus('error') }
    } finally {
      if (mounted.current) {
        setValues(previous => ({ ...previous, password: '', confirmation: '' }))
        setTouched(previous => ({ ...previous, password: false, confirmation: false }))
      }
      submitting.current = false
    }
  }

  return <AuthLayout headingId="registration-heading" wide formProps={{ ref: form, onSubmit: submit, noValidate: true }}
    banner="University prototype. Use test details only."
    footerControls={<p>Registration uses the GovAI backend. Email verification is required before login.</p>}>
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
    <p id="registration-password-policy" className="registration-policy"><img src="/assets/figma/login/register-info.svg" alt="" />At least 8 characters. Passwords must match exactly.</p>
    <div><Checkbox id="register-terms" required checked={values.terms} disabled={pending} aria-invalid={error('terms') ? true : undefined} aria-describedby={error('terms') ? 'registration-legal register-terms-error' : 'registration-legal'} onChange={event => change('terms', event.target.checked)}>I agree (demo only).</Checkbox>
      <p id="registration-legal" className="registration-legal">Terms of use and Privacy notice unavailable.</p>
      {error('terms') && <p id="register-terms-error" className="field-error">{error('terms')}</p>}
    </div>
    <Button type="submit" pending={pending}>{pending ? 'Creating account...' : 'Create account'}{!pending && <Icon name="login-arrow-right" />}</Button>
    {status === 'pending' && <StatusMessage kind="pending">Submitting your registration...</StatusMessage>}
    {status === 'success' && <><StatusMessage kind="success">Request received. If registration can proceed, check your email for the next step. Verify your email before logging in.</StatusMessage>{import.meta.env.DEV && <p className="create-account"><a href="http://localhost:8025" target="_blank" rel="noopener noreferrer">Open verification inbox (local Mailpit)</a></p>}<a className="primary-button registration-login-action" href="/login">Go to login</a></>}
    {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
    <p className="create-account">Already have an account? <a href="/login">Back to login</a></p>
  </AuthLayout>
}
