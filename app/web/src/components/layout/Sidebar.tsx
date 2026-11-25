import { NavigationItem } from './NavigationItem'
import { Icon } from '../Icon'

export type ApplicationPage = 'home' | 'recent' | 'chat' | 'profile'

export function Sidebar({ page, onNewChat, conversationAvailable }: {
  page: ApplicationPage
  onNewChat?: () => void
  conversationAvailable?: boolean
}) {
  return (
    <nav className="application-sidebar" aria-label="Application">
      <NavigationItem
        label="New chat"
        icon="new-chat"
        href="/app/home"
        onClick={onNewChat}
        newChat
        active={page === 'home'}
      />
      <div className="primary-navigation">
        {conversationAvailable && (
          <NavigationItem
            label="Current conversation"
            icon="message"
            href="/app/chat"
            active={page === 'chat'}
          />
        )}
        <NavigationItem
          label="Recent chats"
          icon={page === 'recent' ? 'recent-active' : 'recent'}
          href="/app/recent"
          active={page === 'recent'}
        />
        <NavigationItem label="My requests" icon="requests" />
        <NavigationItem label="Services" icon="services" />
      </div>
      <div className="sidebar-utilities">
        <NavigationItem label="Help & support" icon="help" />
        <NavigationItem label="Settings" icon="settings" href="/app/profile" active={page === 'profile'} />
        <div className="sidebar-plan-card" aria-label="GovAI Plus">
          <span className="crown-tile"><Icon collection="shell" name="crown" /></span>
          <span><strong>GovAI Plus</strong><small>Plan details unavailable</small></span>
          <button type="button" disabled title="Plan management is not available yet">Manage</button>
        </div>
      </div>
    </nav>
  )
}
