import { Icon } from '../Icon'
import { relativeTime } from '../../notifications/presentation'
import type { CitizenNotification } from '../../notifications/types'

export function NotificationRow({ item, compact = false, onOpen }: {
  item: CitizenNotification
  compact?: boolean
  onOpen: () => void
}) {
  const unread = item.read_at === null
  const icon = item.event_type === 'citizen.email.verified' ? 'mail'
    : item.event_type === 'citizen.password.changed' ? 'shield-check' : 'lock'
  return (
    <button
      type="button"
      className={`notification-row ${unread ? 'is-unread' : ''} ${compact ? 'is-compact' : ''}`}
      onClick={onOpen}
      data-close-navigation
      aria-label={`${item.title}${unread ? ', unread' : ''}. Open notification`}
    >
      <span className="notification-icon"><Icon name={icon} /></span>
      <span className="notification-copy">
        <strong>{item.title}</strong>
        <span className="notification-excerpt">{item.message_body}</span>
        <span className="notification-meta">
          {item.priority === 'high' && <span className="notification-security">Security</span>}
          {!compact && <time dateTime={item.created_at}>{relativeTime(item.created_at)}</time>}
        </span>
      </span>
      <span className="notification-end" aria-hidden="true">
        {compact && <time dateTime={item.created_at}>{relativeTime(item.created_at)}</time>}
        {unread && <span className="notification-unread-dot" />}
        {!compact && <span className="notification-chevron">›</span>}
      </span>
    </button>
  )
}
