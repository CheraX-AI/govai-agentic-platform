import { useEffect, useRef } from 'react'
import { AuthLayout } from '../components/auth/AuthLayout'

export default function RecoveryRequiredPage() {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { heading.current?.focus() }, [])
  return <AuthLayout headingId="recovery-required-heading" formProps={{ onSubmit: event => event.preventDefault() }}>
    <div className="heading"><h1 id="recovery-required-heading" tabIndex={-1} ref={heading}>Start the recovery demo</h1><p>This preview needs recovery progression in this tab. Refresh clears that temporary progress.</p></div>
    <p className="status-message">Demo only - no email or SMS was sent. No password was changed.</p>
    <a className="primary-button" href="#/forgot-password">Start recovery</a>
    <a href="#/login">Back to login</a>
  </AuthLayout>
}
