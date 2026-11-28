import { useRef, useState } from 'react'
import { Icon } from '../components/Icon'
import { NotificationFeedback } from '../components/notifications/NotificationFeedback'
import { NotificationRow } from '../components/notifications/NotificationRow'
import { useNotifications } from '../notifications/context'
import { dayGroup } from '../notifications/presentation'

export default function NotificationsPage() {
  const data = useNotifications()
  const [filter, setFilter] = useState<'all' | 'unread'>('all')
  const tabs = useRef<HTMLDivElement>(null)
  const visible = data.items.filter(item => filter === 'all' || item.read_at === null)

  return (
    <section className="notification-page" aria-labelledby="notifications-title">
      <div className="notification-page-heading">
        <div>
          <h1 id="notifications-title">Notifications</h1>
          <p>Account updates and security alerts.
            {data.unread !== null && ` You have ${data.unread} unread.`}
          </p>
        </div>
        <button
          className="notification-secondary"
          type="button"
          onClick={() => data.go('/app/notifications/settings')}
        >Notification settings <Icon collection="shell" name="settings" /></button>
      </div>
      <div
        className="notification-tabs"
        role="tablist"
        aria-label="Filter notifications"
        ref={tabs}
        onKeyDown={event => {
          if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
          event.preventDefault()
          const next = event.key === 'Home' ? 'all' : event.key === 'End' ? 'unread'
            : filter === 'all' ? 'unread' : 'all'
          setFilter(next)
          tabs.current?.querySelector<HTMLButtonElement>(`[data-filter="${next}"]`)?.focus()
        }}
      >
        {(['all', 'unread'] as const).map(value => <button
          type="button"
          key={value}
          id={`notification-tab-${value}`}
          role="tab"
          aria-selected={filter === value}
          aria-controls="notification-tab-panel"
          tabIndex={filter === value ? 0 : -1}
          data-filter={value}
          onClick={() => setFilter(value)}
        >
          {value === 'all' ? 'All' : 'Unread'}
          {value === 'unread' && data.unread !== null && <span>{data.unread}</span>}
        </button>)}
      </div>
      <div
        id="notification-tab-panel"
        role="tabpanel"
        aria-labelledby={`notification-tab-${filter}`}
        tabIndex={0}
      >
        {data.status !== 'ready' ? (
          <NotificationFeedback state={data.status} onRetry={() => void data.refresh()} />
        ) : visible.length === 0 ? (
          filter === 'unread' && data.items.length > 0 ? (
            <div className="notification-feedback" role="status">
              <strong>No unread notifications here</strong>
              <p>{data.hasMore ? 'Load more to see earlier updates.' : 'You’re all caught up.'}</p>
            </div>
          ) : <NotificationFeedback state="empty" />
        ) : (
          ['Today', 'Yesterday', 'Earlier'].map(group => {
            const rows = visible.filter(item => dayGroup(item.created_at) === group)
            return rows.length > 0 && <section key={group} className="notification-day">
              <h2>{group}</h2>
              <ul>
                {rows.map(item => <li key={item.id}><NotificationRow
                  item={item}
                  onOpen={() => data.openDetails(item)}
                /></li>)}
              </ul>
            </section>
          })
        )}
        {data.status === 'ready' && data.hasMore && <div className="notification-pagination">
          {data.moreError && <p role="alert">Earlier notifications couldn’t be loaded.</p>}
          <button
            className="notification-secondary"
            type="button"
            disabled={data.loadingMore}
            onClick={() => void data.loadMore()}
          >{data.loadingMore ? 'Loading…' : data.moreError ? 'Try again' : 'Load more'}</button>
        </div>}
      </div>
    </section>
  )
}
