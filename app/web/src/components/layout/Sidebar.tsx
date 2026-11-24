import { NavigationItem } from './NavigationItem'
import { SidebarAccount } from './SidebarAccount'

export type ApplicationPage = 'home' | 'recent' | 'chat'

export function Sidebar({ page, onLogout, onNewChat, conversationAvailable }: {
  page: ApplicationPage
  onLogout: () => void
  onNewChat?: () => void
  conversationAvailable?: boolean
}) {
  return (
    <nav className="application-sidebar" aria-label="Application">
      <a className="sidebar-brand" href="/app/home" aria-label="GovAI home">
        <img src="/assets/figma/shell/emblem.png" width="57" height="48" alt="" />
        <span className="sidebar-brand-divider" />
        <span className="sidebar-brand-text">
          <span className="wordmark"><span>Gov</span><span>AI</span></span>
          <small>Government Assistant</small>
        </span>
      </a>
      <p className="sidebar-section-label">Workspace</p>
      <div className="primary-navigation">
        <NavigationItem
          label="New chat"
          icon="new-chat"
          href="/app/home"
          onClick={onNewChat}
          newChat
          active={page === 'home'}
        />
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
        <NavigationItem label="Artifacts" icon="artifacts" />
      </div>
      <div className="sidebar-utilities">
        <SidebarAccount onLogout={onLogout} />
      </div>
    </nav>
  )
}
