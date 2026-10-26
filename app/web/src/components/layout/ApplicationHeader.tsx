import { Brand } from '../Brand'
import { Icon } from '../Icon'
import { demoUser } from '../../data/demoUser'

export function ApplicationHeader({ onOpenMenu }: { onOpenMenu: () => void }) {
  return <header className="application-header">
    <button className="menu-toggle" type="button" aria-haspopup="dialog" aria-controls="mobile-navigation" onClick={onOpenMenu}>Menu</button>
    <div className="header-brand"><Brand showSubtitle={false} emblem="/assets/figma/shell/emblem.png" /><span className="brand-divider" /><p className="brand-slogan">Smarter government.<br />Brighter Sri Lanka.</p></div>
    <div className="header-account">
      <button className="shell-language" disabled title="Language switching unavailable"><Icon collection="shell" name="globe" />English<Icon collection="shell" name="chevron" /></button>
      <button className="notification-preview" disabled aria-label="Notifications unavailable"><Icon collection="shell" name="notifications" /></button>
      <span className="brand-divider" />
      <div className="synthetic-user" aria-label={`Synthetic preview user ${demoUser.displayName}; no signed-in account`}><span className="user-avatar">{demoUser.initials}</span><div><strong>{demoUser.displayName}</strong><small>Preview user</small></div><Icon collection="shell" name="user-chevron" /></div>
    </div>
  </header>
}
