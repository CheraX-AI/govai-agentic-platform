import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'
import { VerificationCodeInput } from '../components/auth/VerificationCodeInput'
import { Button } from '../components/Button'
import { StatusMessage } from '../components/StatusMessage'
import { validateCode } from '../validation/codeValidation'
import { createDemoChallenge, demoCode, resendMockCode, secondsUntil, verifyMockCode } from '../services/mockCodeService'
import type { CodeOutcome } from '../services/mockCodeService'
import '../styles/verification.css'

const emptyDigits = () => Array<string>(6).fill('')
const clock = (seconds: number) => `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`

export default function VerificationCodePage({ onVerified }: { onVerified: () => void }) {
  const [challenge, setChallenge] = useState(createDemoChallenge)
  const [now, setNow] = useState(Date.now)
  const [digits, setDigits] = useState(emptyDigits)
  const [error, setError] = useState<string>()
  const [status, setStatus] = useState<'idle' | 'pending' | 'resending' | 'resent' | 'error' | CodeOutcome>('idle')
  const [outcome, setOutcome] = useState<CodeOutcome>('success')
  const mounted = useRef(false)
  const inFlight = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    mounted.current = true
    heading.current?.focus()
    const interval = setInterval(() => setNow(Date.now()), 250)
    return () => { mounted.current = false; clearInterval(interval) }
  }, [])
  const expires = secondsUntil(challenge.expiresAt, now)
  const cooldown = secondsUntil(challenge.resendAt, now)
  const expired = status !== 'success' && (expires === 0 || status === 'expired')
  const busy = status === 'pending' || status === 'resending' || status === 'success'
  useEffect(() => { if (status === 'resent') document.getElementById('code-0')?.focus() }, [status])
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (inFlight.current) return
    setStatus('idle')
    const invalid = validateCode(digits.join(''))
    setError(invalid)
    if (invalid) { document.getElementById(`code-${Math.max(0, digits.findIndex(digit => !digit))}`)?.focus(); return }
    if (Date.now() >= challenge.expiresAt || expired) { setStatus('expired'); return }
    inFlight.current = true
    setStatus('pending')
    try {
      const result = await verifyMockCode(digits.join(''), challenge, outcome)
      if (!mounted.current) return
      setStatus(result)
      if (result === 'expired') setChallenge(previous => ({ ...previous, expiresAt: Date.now() }))
      if (result === 'success') {
        setDigits(emptyDigits())
        await new Promise(resolve => setTimeout(resolve, 600))
        if (mounted.current) onVerified()
      }
    } catch { if (mounted.current) setStatus('error') }
    finally { inFlight.current = false }
  }
  async function resend() {
    if (inFlight.current || Date.now() < challenge.resendAt) return
    inFlight.current = true
    setStatus('resending')
    try {
      const next = await resendMockCode(challenge)
      if (!mounted.current) return
      setChallenge(next); setNow(Date.now()); setDigits(emptyDigits()); setError(undefined); setStatus('resent')
    } catch { if (mounted.current) setStatus('error') }
    finally { inFlight.current = false }
  }
  return <AuthLayout headingId="verification-heading" formProps={{ noValidate: true, onSubmit: submit, 'aria-busy': busy }} footerControls={<details className="demo-options"><summary>Demo verification controls</summary><p>Use demo code {demoCode}. No SMS, account lookup or real verification occurs. Refresh clears recovery progress. Start recovery again to create a new demo challenge.</p><label htmlFor="code-outcome">Simulated verification result</label><select id="code-outcome" value={outcome} disabled={busy} onChange={event => { setOutcome(event.target.value as CodeOutcome); setStatus('idle') }}><option value="success">Success with demo code</option><option value="invalid">Invalid code</option><option value="expired">Expired code</option></select></details>}>
    <div className="heading"><h1 id="verification-heading" tabIndex={-1} ref={heading}>Enter your code</h1><p>Synthetic number: +94 77 *** 0000. This demo challenge lasts five minutes.</p></div>
    <p className="verification-warning">Demo - no code was sent.</p>
    <VerificationCodeInput value={digits} disabled={busy} error={error} onChange={next => { setDigits(next); setError(undefined); if (status !== 'expired') setStatus('idle') }} />
    <p className="verification-hint" id="code-hint">Enter six digits. Demo code: {demoCode}.</p>
    {error && <p className="field-error" id="code-error" role="alert">{error}</p>}
    <p className="verification-timer" role="timer" aria-live="off">Demo expiry: {clock(expires)}</p>
    <button className="verification-resend" type="button" disabled={busy || cooldown > 0} onClick={resend}>{status === 'resending' ? 'Simulating resend...' : cooldown ? `Resend in ${clock(cooldown)}` : 'Resend code (demo)'}</button>
    <Button type="submit" pending={status === 'pending'} disabled={busy || expired}>{status === 'pending' ? 'Simulating verification...' : 'Verify code'}</Button>
    {status === 'pending' && <StatusMessage kind="pending">Simulating code verification...</StatusMessage>}
    {status === 'resending' && <StatusMessage kind="pending">Simulating resend. No code is being sent.</StatusMessage>}
    {status === 'resent' && <StatusMessage kind="success">Demo challenge reset. No code was sent.</StatusMessage>}
    {status === 'success' && <StatusMessage kind="success">Demo recovery verification successful. Continuing to the reset preview. You are not authenticated.</StatusMessage>}
    {status === 'invalid' && <StatusMessage kind="error">Invalid demo code. Please try again.</StatusMessage>}
    {expired && <StatusMessage kind="error">Demo code expired. Resend to start a new demo challenge.</StatusMessage>}
    {status === 'error' && <StatusMessage kind="error">Demo action failed. Please try again. No code was sent.</StatusMessage>}
    <a className="verification-back" href="#/forgot-password">Choose another recovery method</a>
  </AuthLayout>
}
