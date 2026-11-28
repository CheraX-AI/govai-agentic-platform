import { useEffect, useRef } from 'react'
import type { CitizenNotification } from '../../notifications/types'

export function NotificationDetail({ item, reading, readError, onClose, onRetry }: {
  item: CitizenNotification | null
  reading: boolean
  readError: boolean
  onClose: () => void
  onRetry: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)

  // A native modal keeps keyboard focus inside the full message until dismissed.

  useEffect(() => {
    if (item && !dialog.current?.open) dialog.current?.showModal()
    else if (!item) dialog.current?.close()
  }, [item])

  return (
    <dialog
      ref={dialog}
      className="notification-detail"
      aria-labelledby="notification-detail-title"
      onCancel={event => { event.preventDefault(); onClose() }}
      onClick={event => { if (event.target === event.currentTarget) onClose() }}
    >
      {item && <div className="notification-detail-surface">
        <div className="notification-detail-heading">
          <h2 id="notification-detail-title">{item.title}</h2>
          <button type="button" className="notification-close" onClick={onClose} autoFocus>
            <span aria-hidden="true">×</span><span className="sr-only">Close notification</span>
          </button>
        </div>
        <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString('en')}</time>
        {item.priority === 'high' && <span className="notification-security">Security</span>}
        <div className="notification-full-message">
          {item.message_body.trim().split(/\r?\n(?:[ \t]*\r?\n)+/).map((paragraph, index) => (
            <p key={index}>{paragraph}</p>
          ))}
        </div>
        {reading && <p role="status">Marking as read…</p>}
        {readError && <div role="alert" className="notification-inline-error">
          <p>We couldn’t mark this notification as read.</p>
          <button type="button" className="notification-secondary" onClick={onRetry}>
            Try again
          </button>
        </div>}
      </div>}
    </dialog>
  )
}
