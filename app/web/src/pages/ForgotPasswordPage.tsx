import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { InputField } from '../components/InputField'
import { Checkbox } from '../components/Checkbox'
import { Button } from '../components/Button'
import { StatusMessage } from '../components/StatusMessage'
import { validateRecovery } from '../validation/recoveryValidation'
import { submitMockRecovery } from '../services/mockRecoveryService'
import type { MockOutcome } from '../services/mockLoginService'
import '../styles/recovery.css'

export default function ForgotPasswordPage({ onPhoneRecovery }: { onPhoneRecovery: () => void }) {
  const [method, setMethod] = useState<'email' | 'phone' | ''>('')
  const [methodError, setMethodError] = useState(false)
  const [nic, setNic] = useState('')
  const [humanCheck, setHumanCheck] = useState(false)
  const [touched, setTouched] = useState({ nic: false, humanCheck: false })
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const mounted = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  const submitting = useRef(false)
  const form = useRef<HTMLFormElement>(null)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const errors = validateRecovery(nic, humanCheck)
  const pending = status === 'pending'
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched({ nic: true, humanCheck: true })
    setStatus('idle')
    setMethodError(!method)
    if (!method) { document.getElementById('recovery-email')?.focus(); return }
    if (errors.nic || errors.humanCheck) {
      form.current?.querySelector<HTMLInputElement>(errors.nic ? '#recovery-nic' : '#recovery-human')?.focus()
      return
    }
    setNic(nic.trim())
    submitting.current = true
    setStatus('pending')
    try {
      const result = await submitMockRecovery(outcome)
      if (!mounted.current) return
      if (result === 'success') { if (method === 'phone') onPhoneRecovery(); else location.hash = '#/check-email' }
      else setStatus('error')
    } catch { if (mounted.current) setStatus('error') }
    finally { submitting.current = false }
  }
  return <AuthLayout headingId="recovery-heading" formProps={{ ref: form, noValidate: true, onSubmit: submit, 'aria-busy': pending }}
    footerControls={<details className="demo-options"><summary>Demo response controls</summary><label htmlFor="recovery-outcome">Simulated result</label><select id="recovery-outcome" disabled={pending} value={outcome} onChange={event => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select></details>}>
    <div className="heading"><h1 id="recovery-heading" tabIndex={-1} ref={heading}>Reset your password</h1><p>Enter a sample NIC number to preview the recovery interface. No account lookup is performed.</p></div>
    <p className="recovery-warning">Demo only - no email or SMS was sent.</p>
    <fieldset className="recovery-methods" disabled={pending} aria-describedby={methodError ? 'recovery-method-note recovery-method-error' : 'recovery-method-note'}><legend>Choose a recovery method</legend>
      {(['email', 'phone'] as const).map(option => <label key={option}><input id={`recovery-${option}`} type="radio" name="recovery-method" required checked={method === option} value={option} aria-invalid={methodError ? true : undefined} onChange={() => { setMethod(option); setMethodError(false); setStatus('idle') }} />{option === 'email' ? 'Email' : 'Phone'}</label>)}
    </fieldset>
    <p className="create-account" id="recovery-method-note">Provisional selector preview. Synthetic destinations: d•••@example.com or +94 77 *** 0000. No account lookup.</p>
    {methodError && <p className="field-error" id="recovery-method-error" role="alert">Choose Email or Phone recovery.</p>}
    <InputField id="recovery-nic" label="NIC number" icon="register-nic" required autoCapitalize="none" spellCheck={false} value={nic} disabled={pending} error={touched.nic ? errors.nic : undefined} onChange={event => { setNic(event.target.value); setStatus('idle') }} onBlur={() => { setNic(nic.trim()); setTouched(previous => ({ ...previous, nic: true })) }} />
    <div className="human-check"><Checkbox id="recovery-human" required disabled={pending} checked={humanCheck} aria-invalid={touched.humanCheck && errors.humanCheck ? true : undefined} aria-describedby={touched.humanCheck && errors.humanCheck ? 'recovery-human-note recovery-human-error' : 'recovery-human-note'} onChange={event => { setHumanCheck(event.target.checked); setStatus('idle') }}>I'm not a robot</Checkbox><span className="security-badge">Demo check</span></div>
    <p className="helper" id="recovery-human-note">Simulated checkbox only - no security verification.</p>
    {touched.humanCheck && errors.humanCheck && <p className="field-error" id="recovery-human-error">{errors.humanCheck}</p>}
    <Button type="submit" pending={pending}>{pending ? 'Simulating request...' : method === 'phone' ? 'Continue to code preview' : method === 'email' ? 'Continue to email preview' : 'Continue (demo)'}</Button>
    {pending && <StatusMessage kind="pending">Simulating recovery. No email or SMS is being sent.</StatusMessage>}
    {status === 'error' && <StatusMessage kind="error">Demo request failed. Please try again. No email or SMS was sent.</StatusMessage>}
    <a className="recovery-back" href="#/login">Back to log in</a>
  </AuthLayout>
}
