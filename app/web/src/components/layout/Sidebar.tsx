import { NavigationItem } from './NavigationItem'

export type ApplicationPage = 'home' | 'recent'

export function Sidebar({ page }: { page: ApplicationPage }) {
  return <nav className="application-sidebar" aria-label="Application">
    <NavigationItem label="New chat" icon="new-chat" href="/app/home" newChat active={page === 'home'} />
    <div className="primary-navigation">
      <NavigationItem label="Recent chats" icon={page === 'recent' ? 'recent-active' : 'recent'} href="/app/recent" active={page === 'recent'} />
      <NavigationItem label="My requests" icon="requests" />
      <NavigationItem label="Services" icon="services" />
    </div>
    <div className="sidebar-utilities"><p className="unavailable-note">Other destinations unavailable</p><NavigationItem label="Help & support" icon="help" /><NavigationItem label="Settings" icon="settings" /></div>
  </nav>
}
