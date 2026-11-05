import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { InputField } from '../components/InputField'
import { Checkbox } from '../components/Checkbox'
import { Button } from '../components/Button'
import { StatusMessage } from '../components/StatusMessage'
import { validateRecovery } from '../validation/recoveryValidation'
import { requestRecovery } from '../services/recoveryApi'
import type { RecoveryReceipt } from '../services/recoveryApi'
import type { MockOutcome } from '../services/mockLoginService'
import '../styles/recovery.css'

export default function ForgotPasswordPage({ onReceived }: { onReceived: (receipt: RecoveryReceipt) => void }) {
  const [method, setMethod] = useState<'email' | 'phone' | ''>('')
  const [methodError, setMethodError] = useState(false)
  const [nic, setNic] = useState('')
  const [humanCheck, setHumanCheck] = useState(false)
  const [touched, setTouched] = useState({ nic: false, humanCheck: false })
  const [failure, setFailure] = useState('')
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
    if (method !== 'email') { document.getElementById('recovery-email')?.focus(); return }
    if (errors.nic || errors.humanCheck) {
      form.current?.querySelector<HTMLInputElement>(errors.nic ? '#recovery-nic' : '#recovery-human')?.focus()
      return
    }
    setNic(nic.trim())
    submitting.current = true
    setStatus('pending')
    try {
      const result = await requestRecovery(nic)
      if (!mounted.current) return
      onReceived(result)
    } catch (error) { if (mounted.current) { setFailure(error instanceof Error ? error.message : 'Recovery is unavailable.'); setStatus('error') } }
    finally { submitting.current = false }
  }
  return <AuthLayout banner="University prototype. Use test details only." headingId="recovery-heading" formProps={{ ref: form, noValidate: true, onSubmit: submit, 'aria-busy': pending }}
    footerControls={<details className="demo-options"><summary>Recovery information</summary><label htmlFor="recovery-outcome">Response source</label><select id="recovery-outcome" disabled value="backend"><option value="backend">GovAI backend</option></select></details>}>
    <div className="heading"><h1 id="recovery-heading" tabIndex={-1} ref={heading}>Reset your password</h1><p>Enter your NIC number to request a reset link at your registered verified email.</p></div>
    <p className="recovery-warning">Email recovery is available. Phone recovery is unavailable.</p>
    <fieldset className="recovery-methods" disabled={pending} aria-describedby={methodError ? 'recovery-method-note recovery-method-error' : 'recovery-method-note'}><legend>Choose a recovery method</legend>
      {(['email', 'phone'] as const).map(option => <label key={option}><input id={`recovery-${option}`} type="radio" name="recovery-method" required disabled={option === 'phone'} title={option === 'phone' ? 'Phone recovery is unavailable' : undefined} checked={method === option} value={option} aria-invalid={methodError ? true : undefined} onChange={() => { setMethod(option); setMethodError(false); setStatus('idle') }} />{option === 'email' ? 'Email' : 'Phone'}</label>)}
    </fieldset>
    <p className="create-account" id="recovery-method-note">Choose Email to receive a reset link at your registered email.</p>
    {methodError && <p className="field-error" id="recovery-method-error" role="alert">Choose Email recovery.</p>}
    <InputField id="recovery-nic" label="NIC number" icon="register-nic" required autoCapitalize="none" spellCheck={false} value={nic} disabled={pending} error={touched.nic ? errors.nic : undefined} onChange={event => { setNic(event.target.value); setStatus('idle') }} onBlur={() => { setNic(nic.trim()); setTouched(previous => ({ ...previous, nic: true })) }} />
    <div className="human-check"><Checkbox id="recovery-human" required disabled={pending} checked={humanCheck} aria-invalid={touched.humanCheck && errors.humanCheck ? true : undefined} aria-describedby={touched.humanCheck && errors.humanCheck ? 'recovery-human-note recovery-human-error' : 'recovery-human-note'} onChange={event => { setHumanCheck(event.target.checked); setStatus('idle') }}>I'm not a robot</Checkbox><span className="security-badge">Demo check</span></div>
    <p className="helper" id="recovery-human-note">Simulated checkbox only - no security verification.</p>
    {touched.humanCheck && errors.humanCheck && <p className="field-error" id="recovery-human-error">{errors.humanCheck}</p>}
    <Button type="submit" pending={pending}>{pending ? 'Sending request...' : 'Send reset link'}</Button>
    {pending && <StatusMessage kind="pending">Requesting your reset link...</StatusMessage>}
    {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
    <a className="recovery-back" href="#/login">Back to log in</a>
  </AuthLayout>
}
