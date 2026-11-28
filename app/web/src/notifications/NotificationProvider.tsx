import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { navigate } from '../routing/routes'
import { notificationService } from '../services/notificationService'
import { NotificationDetail } from '../components/notifications/NotificationDetail'
import { NotificationContext } from './context'
import { useNotificationData } from './useNotificationData'
import type {
  CitizenNotification, NotificationDestination, NotificationService,
} from './types'
import '../styles/notifications.css'

export function NotificationProvider({
  children, service = notificationService, onNavigate = navigate,
}: {
  children: ReactNode
  service?: NotificationService
  onNavigate?: (destination: NotificationDestination) => void
}) {
  const data = useNotificationData(service)
  const [selected, setSelected] = useState<CitizenNotification | null>(null)
  const [readError, setReadError] = useState(false)
  const [reading, setReading] = useState(false)
  const readRequest = useRef<AbortController | null>(null)
  const returnFocus = useRef<HTMLElement | null>(null)

  useEffect(() => () => readRequest.current?.abort(), [])

  async function markRead(item: CitizenNotification) {
    if (item.read_at) return
    readRequest.current?.abort()
    const controller = new AbortController()
    readRequest.current = controller
    setReading(true)
    setReadError(false)
    try {
      await service.markRead(item.id, controller.signal)
      if (controller.signal.aborted) return
      setSelected(previous => previous?.id === item.id
        ? { ...previous, read_at: new Date().toISOString() } : previous)
      await data.refresh()
    } catch {
      if (!controller.signal.aborted) setReadError(true)
    } finally {
      if (!controller.signal.aborted) setReading(false)
    }
  }

  function closeDetails() {
    setSelected(null)
    requestAnimationFrame(() => {
      const target = returnFocus.current
      if (target?.isConnected) target.focus()
      else {
        const visible = (selector: string) => [
          ...document.querySelectorAll<HTMLButtonElement>(selector),
        ].find(button => button.getClientRects().length > 0)
        ;(visible('.notification-bell') ?? visible('.menu-toggle'))?.focus()
      }
    })
  }

  return (
    <NotificationContext.Provider value={{
      ...data, service, go: onNavigate,
      openDetails: item => {
        returnFocus.current = document.activeElement as HTMLElement | null
        readRequest.current?.abort()
        setReading(false)
        setSelected(item)
        setReadError(false)
        void markRead(item)
      },
    }}>
      {children}
      <NotificationDetail
        item={selected}
        reading={reading}
        readError={readError}
        onClose={closeDetails}
        onRetry={() => { if (selected) void markRead(selected) }}
      />
    </NotificationContext.Provider>
  )
}
