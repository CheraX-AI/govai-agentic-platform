import { Icon } from '../Icon'
import { NavigationItem } from './NavigationItem'

export type LayoutPreview = 'home' | 'recent'

export function Sidebar({ page }: { page: LayoutPreview }) {
  return <nav className="application-sidebar" aria-label="Application preview">
    <NavigationItem label="New chat" icon="new-chat" href="#/home" newChat active={page === 'home'} />
    <div className="primary-navigation">
      <NavigationItem label="Recent chats" icon={page === 'recent' ? 'recent-active' : 'recent'} href="#/recent" active={page === 'recent'} />
      <NavigationItem label="My requests" icon="requests" />
      <NavigationItem label="Services" icon="services" />
    </div>
    <div className="sidebar-recent"><p>Recent <small>- static samples</small></p>{['Renew your NIC', 'Passport photo rules', 'Vehicle revenue licence', 'Nearest DS office'].map(label => <div className="sample-recent" key={label}><Icon collection="shell" name="message" /><span>{label}</span></div>)}</div>
    <div className="sidebar-utilities"><p className="unavailable-note">Other destinations unavailable</p><NavigationItem label="Help & support" icon="help" /><NavigationItem label="Settings" icon="settings" />
      <div className="plan-preview"><span className="crown-tile"><Icon collection="shell" name="crown" /></span><div><strong>GovAI Plus</strong><small>Sample plan only</small></div><button type="button" disabled>Manage</button></div>
      <a className="login-preview-link" href="#/login">Phone-login preview</a>
    </div>
  </nav>
}
