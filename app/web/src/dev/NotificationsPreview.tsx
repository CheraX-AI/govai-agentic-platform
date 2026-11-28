import { useMemo, useState } from 'react'
import { ApplicationLayout } from '../components/layout/ApplicationLayout'
import { NotificationProvider } from '../notifications/NotificationProvider'
import NotificationsPage from '../pages/NotificationsPage'
import NotificationSettingsPage from '../pages/NotificationSettingsPage'
import { createNotificationPreviewService } from './notificationPreviewService'
import type { PreviewOutcome } from './notificationPreviewService'
import './notificationsPreview.css'

export default function NotificationsPreview() {
  const requested = new URLSearchParams(location.search).get('state')
  const outcome: PreviewOutcome = requested === 'empty' || requested === 'error' ||
    requested === 'loading' ? requested : 'ready'
  const [view, setView] = useState<'notifications' | 'notification-settings'>('notifications')
  const service = useMemo(() => createNotificationPreviewService(outcome), [outcome])
  return (
    <div className="notifications-development">
      <NotificationProvider
        key={outcome}
        service={service}
        onNavigate={destination => setView(destination.endsWith('/settings')
          ? 'notification-settings' : 'notifications')}
      >
        <ApplicationLayout
          page={view}
          onLogout={() => { location.href = '/login' }}
          onNewChat={() => setView('notifications')}
        >
          {view === 'notifications' ? <NotificationsPage /> : <NotificationSettingsPage />}
          <p className="notification-preview-note" role="note">Design preview · Sample data</p>
        </ApplicationLayout>
      </NotificationProvider>
    </div>
  )
}
