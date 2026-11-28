import type { NotificationService } from '../notifications/types'

// No browser BFF endpoint has been agreed or deployed yet. Fail honestly rather
// than guessing a URL, storing tokens, or substituting development sample data.

async function unavailable(): Promise<never> {
  throw new Error('Notifications are currently unavailable. Please try again later.')
}

export const notificationService: NotificationService = {
  list: unavailable,
  unreadCount: unavailable,
  markRead: unavailable,
  getPreferences: unavailable,
  savePreferences: unavailable,
}
