import { Brand } from '../Brand'
import { Icon } from '../Icon'
import { demoUser } from '../../data/demoUser'
import { useEffect, useRef, useState } from 'react'

export function ApplicationHeader({ onOpenMenu, onDemoLogout }: { onOpenMenu: () => void; onDemoLogout?: () => void }) {
  const [accountOpen, setAccountOpen] = useState(false)
  const account = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!accountOpen) return
    account.current?.querySelector<HTMLElement>('.account-actions button:not(:disabled), .account-actions a')?.focus()
    function dismiss(event: PointerEvent) {
      if (!account.current?.contains(event.target as Node)) setAccountOpen(false)
    }
    function escape(event: globalThis.KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); setAccountOpen(false); trigger.current?.focus() }
    }
    document.addEventListener('pointerdown', dismiss)
    document.addEventListener('keydown', escape)
    return () => { document.removeEventListener('pointerdown', dismiss); document.removeEventListener('keydown', escape) }
  }, [accountOpen])
  return <header className="application-header">
    <button className="menu-toggle" type="button" aria-haspopup="dialog" aria-controls="mobile-navigation" onClick={onOpenMenu}>Menu</button>
    <div className="header-brand"><Brand showSubtitle={false} emblem="/assets/figma/shell/emblem.png" /><span className="brand-divider" /><p className="brand-slogan">Smarter government.<br />Brighter Sri Lanka.</p></div>
    <div className="header-account">
      <button className="shell-language" disabled title="Language switching unavailable"><Icon collection="shell" name="globe" />English<Icon collection="shell" name="chevron" /></button>
      <button className="notification-preview" disabled aria-label="Notifications unavailable"><Icon collection="shell" name="notifications" /></button>
      <span className="brand-divider" />
      <div className="account-disclosure" ref={account} onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setAccountOpen(false) }}>
        <button ref={trigger} type="button" className="synthetic-user" aria-label={onDemoLogout ? 'Signed-in account actions' : `${onDemoLogout ? 'GovAI user' : demoUser.displayName} demo account actions; no real signed-in account`} aria-expanded={accountOpen} aria-controls="account-actions" onClick={() => setAccountOpen(previous => !previous)} onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); setAccountOpen(true) } }}><span className="user-avatar">{onDemoLogout ? 'G' : demoUser.initials}</span><span><strong>{onDemoLogout ? 'GovAI user' : demoUser.displayName}</strong><small>{onDemoLogout ? 'Signed in' : 'Preview user'}</small></span><Icon collection="shell" name="user-chevron" /></button>
        {accountOpen && <div id="account-actions" className="account-actions" role="region" aria-label="Demo account actions">
          <p>{onDemoLogout ? 'Signed in securely.' : 'Static preview - no session required.'}</p>
          <button type="button" disabled={!onDemoLogout} onClick={() => { setAccountOpen(false); onDemoLogout?.() }}>Log out</button>
          {!onDemoLogout && <a href="#/login">Go to login</a>}
        </div>}
      </div>
    </div>
  </header>
}
