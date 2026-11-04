import { useEffect, useRef, useState } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { StatusMessage } from '../components/StatusMessage'
import { submitMockRecovery } from '../services/mockRecoveryService'
import type { MockOutcome } from '../services/mockLoginService'
import '../styles/recovery.css'

// Fixed synthetic display data, independent of any entered NIC.
const demoAddress = 'd•••@example.com'

export default function ResetEmailConfirmationPage() {
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const mounted = useRef(false)
  const submitting = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const pending = status === 'pending'
  async function resend() {
    if (submitting.current) return
    submitting.current = true
    setStatus('pending')
    try {
      const result = await submitMockRecovery(outcome)
      if (mounted.current) setStatus(result)
    } catch { if (mounted.current) setStatus('error') }
    finally { submitting.current = false }
  }
  return <AuthLayout headingId="check-email-heading" formProps={{ onSubmit: event => event.preventDefault(), 'aria-busy': pending }}
    footerControls={<details className="demo-options"><summary>Demo response controls</summary><label htmlFor="resend-outcome">Simulated resend result</label><select id="resend-outcome" disabled={pending} value={outcome} onChange={event => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select></details>}>
    <div className="recovery-mail"><img src="/assets/figma/login/recovery-mail-check.svg" alt="" width="34" height="34" /></div>
    <div className="heading"><h1 id="check-email-heading" tabIndex={-1} ref={heading}>Check your email</h1><p>Synthetic address: <strong>{demoAddress}</strong></p></div>
    {status === 'idle' && <StatusMessage kind="success">Demo confirmation preview. No email was sent.</StatusMessage>}
    <p className="recovery-warning">Demo only - no email was sent</p>
    <p className="recovery-tip"><img src="/assets/figma/login/register-info.svg" alt="" width="18" height="18" />The 30-minute link lifetime is a design preview only. There is no active reset link or expiry timer.</p>
    <a className="primary-button recovery-back" href="#/login">Back to log in</a>
    <button className="recovery-resend" type="button" disabled={pending} onClick={resend}>{pending ? 'Simulating resend...' : 'Resend link (demo)'}</button>
    {pending && <StatusMessage kind="pending">Simulating resend. No email is being sent.</StatusMessage>}
    {status === 'success' && <StatusMessage kind="success">Demo resend successful. No email was sent.</StatusMessage>}
    {status === 'error' && <StatusMessage kind="error">Demo resend failed. Please try again. No email was sent.</StatusMessage>}
  </AuthLayout>
}
