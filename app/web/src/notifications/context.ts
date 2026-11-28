import { createContext, useContext } from 'react'
import type {
  CitizenNotification, NotificationDestination, NotificationService,
} from './types'

export interface NotificationContextValue {
  items: CitizenNotification[]
  unread: number | null
  status: 'loading' | 'ready' | 'error'
  hasMore: boolean
  loadingMore: boolean
  moreError: boolean
  refresh: () => Promise<void>
  loadMore: () => Promise<void>
  openDetails: (notification: CitizenNotification) => void
  go: (destination: NotificationDestination) => void
  service: NotificationService
}

export const NotificationContext = createContext<NotificationContextValue | null>(null)

export function useNotifications() {
  const value = useContext(NotificationContext)
  if (!value) throw new Error('Notification components require their provider.')
  return value
}
