import { useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Icon } from '../components/Icon'
import { Brand } from '../components/Brand'
import { Button } from '../components/Button'
import { InputField } from '../components/InputField'
import { PasswordField } from '../components/PasswordField'
import { Checkbox } from '../components/Checkbox'
import { StatusMessage } from '../components/StatusMessage'
import { validateLogin } from '../validation/loginValidation'
import { submitMockLogin } from '../services/mockLoginService'
import type { MockOutcome } from '../services/mockLoginService'
import '../styles/login.css'

export default function LoginPage() {
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [humanCheck, setHumanCheck] = useState(false)
  const [touched, setTouched] = useState({ phone: false, password: false, humanCheck: false })
  const [outcome, setOutcome] = useState<MockOutcome>('success')
  const [status, setStatus] = useState<'idle' | 'pending' | MockOutcome>('idle')
  const submitting = useRef(false)
  const form = useRef<HTMLFormElement>(null)
  const errors = validateLogin(phone, password, humanCheck)
  const pending = status === 'pending'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current) return
    setTouched({ phone: true, password: true, humanCheck: true })
    setStatus('idle')
    if (Object.values(errors).some(Boolean)) {
      const firstInvalid = errors.phone ? '#phone' : errors.password ? '#password' : '#human-check'
      form.current?.querySelector<HTMLInputElement>(firstInvalid)?.focus()
      return
    }
    submitting.current = true
    setStatus('pending')
    try {
      setStatus(await submitMockLogin(outcome))
    } catch {
      setStatus('error')
    } finally {
      setPassword('')
      setTouched((previous) => ({ ...previous, password: false }))
      submitting.current = false
    }
  }

  return <main className="login-layout">
    <section className="form-panel" aria-labelledby="login-heading">
      <header className="top-bar"><Brand /><span className="language"><Icon name="globe" />English only</span></header>
      <div className="login-content">
        <div className="demo-banner">Demo — backend unavailable. Use sample details only.</div>
        <form ref={form} className="login-card" onSubmit={submit} noValidate>
          <div className="heading"><h1 id="login-heading">Welcome back</h1><p>Log in to use government services with GovAI.</p></div>
          <div className="login-method" aria-label="Login method">
            <span className="selected-method"><Icon name="mobile-tab" />Mobile number</span>
            <span className="unavailable-method"><Icon name="nic-email-tab" />NIC or email<small>Coming later</small></span>
          </div>
          <InputField id="phone" label="Mobile number" icon="phone" type="tel" inputMode="tel" autoComplete="tel" required
            placeholder="+94 77 123 4567" value={phone} disabled={pending}
            error={touched.phone ? errors.phone : undefined}
            onBlur={() => setTouched((previous) => ({ ...previous, phone: true }))}
            onChange={(event) => { setPhone(event.target.value); setStatus('idle') }} />
          <PasswordField id="password" autoComplete="current-password" required value={password} disabled={pending}
            error={touched.password ? errors.password : undefined}
            onBlur={() => setTouched((previous) => ({ ...previous, password: true }))}
            onChange={(event) => { setPassword(event.target.value); setStatus('idle') }} />
          <div className="options"><Checkbox disabled>Remember me <small>(unavailable)</small></Checkbox><span className="unavailable-text">Password reset unavailable</span></div>
          <div className="human-check">
            <Checkbox id="human-check" checked={humanCheck} disabled={pending}
              aria-invalid={touched.humanCheck && errors.humanCheck ? true : undefined}
              aria-describedby={touched.humanCheck && errors.humanCheck ? 'human-note human-error' : 'human-note'}
              onChange={(event) => { setHumanCheck(event.target.checked); setStatus('idle') }}>I’m not a robot</Checkbox>
            <div className="security-badge"><Icon name="shield-check" /><span>Demo check</span></div>
          </div>
          <p className="helper" id="human-note">Simulated checkbox only — no security verification.</p>
          {touched.humanCheck && errors.humanCheck && <p id="human-error" className="field-error">{errors.humanCheck}</p>}
          <Button type="submit" pending={pending}>{pending ? 'Simulating…' : 'Log in'}{!pending && <Icon name="login-arrow-right" />}</Button>
          {status === 'pending' && <StatusMessage kind="pending">Simulating a response…</StatusMessage>}
          {status === 'success' && <StatusMessage kind="success">Demo submission successful. You are not signed in.</StatusMessage>}
          {status === 'error' && <StatusMessage kind="error">Demo login failed. Please try again.</StatusMessage>}
          <p className="create-account">New to GovAI? <span>Registration unavailable</span></p>

        </form>
      </div>
      <footer><span>University project preview · English only</span><details className="demo-options"><summary>Demo response controls</summary><label htmlFor="outcome">Simulated result</label><select id="outcome" disabled={pending} value={outcome} onChange={(event) => { setOutcome(event.target.value === 'error' ? 'error' : 'success'); setStatus('idle') }}><option value="success">Success</option><option value="error">Error</option></select></details></footer>
    </section>
    <aside className="photo-panel" aria-label="Government services introduction">
      <img className="login-photo" src="/assets/figma/login/login-photo.jpg" alt="" />
      <div className="photo-scrim" />
      <div className="slide-caption"><h2>Government services,<br />closer to you</h2><p>Guidance for Sri Lanka’s government services.</p><p className="slide-note">Photo preview · additional slides unavailable</p></div>
      <div className="help-badge"><img src="/assets/figma/login/govai-emblem.png" alt="" width="40" height="40" /><span>Ask GovAI<small>Help unavailable in this demo</small></span></div>
    </aside>
  </main>
}
