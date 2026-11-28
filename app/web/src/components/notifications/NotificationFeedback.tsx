export function NotificationFeedback({ state, onRetry }: {
  state: 'loading' | 'error' | 'empty'
  onRetry?: () => void
}) {
  if (state === 'loading') return (
    <div className="notification-feedback" role="status">
      <span className="notification-loader" aria-hidden="true" />
      <p>Loading notifications…</p>
    </div>
  )
  if (state === 'error') return (
    <div className="notification-feedback" role="alert">
      <strong>Notifications are unavailable</strong>
      <p>Please try again in a moment.</p>
      <button className="notification-secondary" type="button" onClick={onRetry}>
        Try again
      </button>
    </div>
  )
  return (
    <div className="notification-feedback" role="status">
      <strong>You’re all caught up</strong>
      <p>Your account updates will appear here.</p>
    </div>
  )
}
