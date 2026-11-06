import type { ComponentPropsWithRef, ReactNode } from 'react'
import { Brand } from '../Brand'
import { Icon } from '../Icon'
import '../../styles/login.css'

type AuthLayoutProps = {
  headingId: string
  wide?: boolean
  formProps: ComponentPropsWithRef<'form'>
  footerControls?: ReactNode
  children: ReactNode
}

// Shared authentication chrome; each page retains its own form state and handlers.
export function AuthLayout({ headingId, wide = false, formProps, footerControls, children }: AuthLayoutProps) {
  return <main className={wide ? 'login-layout auth-layout-wide' : 'login-layout'}>
    <section className="form-panel" aria-labelledby={headingId}>
      <header className="top-bar"><Brand /><span className="language"><Icon name="globe" />English</span></header>
      <div className={wide ? 'login-content auth-content-wide' : 'login-content'}>
        <div className="demo-banner">Demo - backend unavailable. Use sample details only.</div>
        <form {...formProps} className="login-card">{children}</form>
      </div>
      <footer><span>University project preview · English only</span>{footerControls}</footer>
    </section>
    <aside className="photo-panel" aria-label="Government services introduction">
      <img className="login-photo" src="/assets/figma/login/auth-coast.jpg" alt="" />
      <div className="photo-scrim" />
      <div className="slide-caption"><h2>Government services,<br />closer to you</h2><p>Guidance for Sri Lanka’s government services.</p><p className="slide-note">Photo preview · additional slides unavailable</p></div>
      <div className="help-badge"><img src="/assets/figma/login/govai-emblem.png" alt="" width="40" height="40" /><span>Ask GovAI<small>Help unavailable in this demo</small></span></div>
    </aside>
  </main>
}
