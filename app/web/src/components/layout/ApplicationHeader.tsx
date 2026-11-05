import { Brand } from '../Brand'
import { Icon } from '../Icon'
import { useEffect, useRef, useState } from 'react'

export function ApplicationHeader({ onOpenMenu, onLogout }: { onOpenMenu: () => void; onLogout: () => void }) {
  const [accountOpen, setAccountOpen] = useState(false)
  const account = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!accountOpen) return
    account.current?.querySelector<HTMLElement>('.account-actions button:not(:disabled)')?.focus()
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
        <button ref={trigger} type="button" className="account-button" aria-label="Signed-in account actions" aria-expanded={accountOpen} aria-controls="account-actions" onClick={() => setAccountOpen(previous => !previous)} onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); setAccountOpen(true) } }}><span className="user-avatar">G</span><span><strong>GovAI user</strong><small>Signed in</small></span><Icon collection="shell" name="user-chevron" /></button>
        {accountOpen && <div id="account-actions" className="account-actions" role="region" aria-label="Account actions">
          <p>Signed in securely.</p>
          <button type="button" onClick={() => { setAccountOpen(false); onLogout() }}>Log out</button>
        </div>}
      </div>
    </div>
  </header>
}
