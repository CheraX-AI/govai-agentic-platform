import { useEffect, useRef } from 'react'
import { useAuth } from '../auth/AuthContext'

export function AuthStatusPage({ error = false }: { error?: boolean }) {
  const { retry } = useAuth()
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => { heading.current?.focus() }, [error])
  return <main className="unknown-preview" aria-busy={!error}>
    <h1 ref={heading} tabIndex={-1}>{error ? 'Session check failed' : 'Checking session...'}</h1>
    <p role={error ? 'alert' : 'status'}>{error ? 'Protected content is unavailable. No authentication was confirmed.' : 'Protected content stays hidden while the session is checked.'}</p>
    {error && <><button className="primary-button" type="button" onClick={() => void retry()}>Retry session check</button><a href="#/login">Back to login</a></>}
  </main>
}
