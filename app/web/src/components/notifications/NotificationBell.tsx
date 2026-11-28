import { useEffect, useId, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { Icon } from '../Icon'
import { useNotifications } from '../../notifications/context'
import { NotificationFeedback } from './NotificationFeedback'
import { NotificationRow } from './NotificationRow'

export function NotificationBell({ onOpen }: { onOpen: () => void }) {
  const data = useNotifications()
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState<CSSProperties>({})
  const popoverId = useId()
  const headingId = useId()
  const region = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const heading = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    if (!open) return
    const frame = requestAnimationFrame(() => heading.current?.focus())
    function place() {
      const anchor = trigger.current?.getBoundingClientRect()
      if (!anchor) return
      const width = Math.min(430, innerWidth - 24)
      const above = anchor.top > innerHeight / 2
      const space = above ? anchor.top - 24 : innerHeight - anchor.bottom - 24
      setPosition({
        position: 'fixed', width,
        left: Math.max(12, Math.min(anchor.right + 12, innerWidth - width - 12)),
        top: above ? 'auto' : anchor.bottom + 12,
        bottom: above ? innerHeight - anchor.top + 8 : 'auto',
        maxHeight: Math.max(60, Math.min(620, space)),
      })
    }
    function dismiss(event: PointerEvent) {
      if (!region.current?.contains(event.target as Node)) setOpen(false)
    }
    function escape(event: globalThis.KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      setOpen(false)
      trigger.current?.focus()
    }
    function closeOnNavigation() { setOpen(false) }
    place()
    document.addEventListener('pointerdown', dismiss)
    document.addEventListener('keydown', escape)
    window.addEventListener('resize', place)
    window.addEventListener('popstate', closeOnNavigation)
    return () => {
      cancelAnimationFrame(frame)
      document.removeEventListener('pointerdown', dismiss)
      document.removeEventListener('keydown', escape)
      window.removeEventListener('resize', place)
      window.removeEventListener('popstate', closeOnNavigation)
    }
  }, [open])

  return (
    <div
      ref={region}
      className="notification-disclosure"
      onBlur={event => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false)
      }}
    >
      <button
        ref={trigger}
        type="button"
        className="notification-bell"
        aria-label={`Notifications${data.unread ? `, ${data.unread} unread` : ''}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? popoverId : undefined}
        onClick={() => {
          if (!open) onOpen()
          setOpen(previous => !previous)
        }}
      >
        <Icon collection="shell" name="notifications" />
        {!!data.unread && <span className="notification-badge" aria-hidden="true">
          {data.unread > 99 ? '99+' : data.unread}
        </span>}
      </button>
      {open && <section
        id={popoverId}
        className="notification-popover"
        role="dialog"
        aria-modal="false"
        aria-labelledby={headingId}
        style={position}
      >
        <div className="notification-popover-heading">
          <h2 id={headingId} tabIndex={-1} ref={heading}>Notifications</h2>
          {!!data.unread && <span className="notification-count">{data.unread} unread</span>}
          <button
            type="button"
            className="notification-settings-icon"
            aria-label="Notification settings"
            data-close-navigation
            onClick={() => { setOpen(false); data.go('/app/notifications/settings') }}
          ><Icon collection="shell" name="settings" /></button>
        </div>
        <div className="notification-popover-list">
          {data.status !== 'ready' ? (
            <NotificationFeedback state={data.status} onRetry={() => void data.refresh()} />
          ) : data.items.length === 0 ? <NotificationFeedback state="empty" /> : (
            data.items.slice(0, 3).map(item => <NotificationRow
              key={item.id}
              item={item}
              compact
              onOpen={() => { setOpen(false); data.openDetails(item) }}
            />)
          )}
        </div>
        <button
          type="button"
          className="notification-view-all"
          data-close-navigation
          onClick={() => { setOpen(false); data.go('/app/notifications') }}
        >View all notifications <span aria-hidden="true">→</span></button>
      </section>}
    </div>
  )
}
