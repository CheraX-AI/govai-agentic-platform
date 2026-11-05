import { NavigationItem } from './NavigationItem'

export type ApplicationPage = 'home' | 'recent' | 'chat'

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
        <p className="unavailable-note">Other destinations unavailable</p>
        <NavigationItem label="Help & support" icon="help" />
        <NavigationItem label="Settings" icon="settings" />
      </div>
    </nav>
  )
}
