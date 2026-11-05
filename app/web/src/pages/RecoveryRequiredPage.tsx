import { useEffect, useRef } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'

export default function RecoveryRequiredPage() {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { heading.current?.focus() }, [])
  return <AuthLayout banner="University prototype. Use test details only." headingId="recovery-required-heading" formProps={{ onSubmit: event => event.preventDefault() }}>
    <div className="heading"><h1 id="recovery-required-heading" tabIndex={-1} ref={heading}>Open a valid reset link</h1><p>Open the newest link from your email. After refreshing this page, reopen the email link or request a new one.</p></div>
    <p className="status-message">No password was changed.</p>
    <a className="primary-button" href="/forgot-password">Start recovery</a>
    <a href="/login">Back to login</a>
  </AuthLayout>
}
