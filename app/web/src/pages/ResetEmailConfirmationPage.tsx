import { useEffect, useRef, useState } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { StatusMessage } from '../components/StatusMessage'
import { requestRecovery } from '../services/recoveryApi'
import type { RecoveryReceipt } from '../services/recoveryApi'
import '../styles/recovery.css'

export default function ResetEmailConfirmationPage({ receipt, onReceived }: { receipt: RecoveryReceipt | null; onReceived: (receipt: RecoveryReceipt) => void }) {
  const [failure, setFailure] = useState('')
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const mounted = useRef(false)
  const submitting = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { mounted.current = true; heading.current?.focus(); return () => { mounted.current = false } }, [])
  const pending = status === 'pending'
  async function resend() {
    if (submitting.current || !receipt) return
    submitting.current = true
    setStatus('pending')
    try {
      const result = await requestRecovery(receipt.nic)
      if (mounted.current) { onReceived(result); setStatus('success') }
    } catch (error) { if (mounted.current) { setFailure(error instanceof Error ? error.message : 'Recovery is unavailable.'); setStatus('error') } }
    finally { submitting.current = false }
  }
  return <AuthLayout banner="University prototype. Use test details only." headingId="check-email-heading" formProps={{ onSubmit: event => event.preventDefault(), 'aria-busy': pending }}>
    <div className="recovery-mail"><img src="/assets/figma/login/recovery-mail-check.svg" alt="" width="34" height="34" /></div>
    <div className="heading"><h1 id="check-email-heading" tabIndex={-1} ref={heading}>Check your email</h1><p>{receipt?.maskedEmail ? <>Registered email: <strong>{receipt.maskedEmail}</strong></> : 'If these details match an eligible account, a link will be sent to its registered email.'}</p></div>
    {status === 'idle' && <StatusMessage kind="success">Request received. Check your email for the next step.</StatusMessage>}
    <p className="recovery-warning">Use the newest reset email. Check your spam folder too.</p>
    <p className="recovery-tip"><img src="/assets/figma/login/register-info.svg" alt="" width="18" height="18" />The link expires at the time stated in the email and can be used once.</p>
    {import.meta.env.DEV && <p className="create-account"><a href="http://localhost:8026" target="_blank" rel="noopener noreferrer">Open password-reset inbox (local Mailpit)</a></p>}
    <a className="primary-button recovery-back" href="/login">Back to log in</a>
    <button className="recovery-resend" type="button" disabled={pending || !receipt} onClick={resend}>{pending ? 'Requesting link...' : 'Resend link'}</button>
    {pending && <StatusMessage kind="pending">Requesting another reset link...</StatusMessage>}
    {status === 'success' && <StatusMessage kind="success">Request received. Check your email.</StatusMessage>}
    {status === 'error' && <StatusMessage kind="error">{failure}</StatusMessage>}
  </AuthLayout>
}
