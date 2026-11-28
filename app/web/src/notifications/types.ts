export type AccountEvent =
  | 'citizen.login.succeeded'
  | 'citizen.password.changed'
  | 'citizen.email.verified'

export interface CitizenNotification {
  id: string
  event_type: AccountEvent
  priority: 'high' | 'normal'
  locale: 'en'
  title: string
  message_body: string
  created_at: string
  read_at: string | null
}

export interface NotificationPage {
  items: CitizenNotification[]
  next_cursor: string | null
}

export interface ChannelPreferences {
  email_enabled: boolean
  in_app_enabled: boolean
  locale: 'en'
  security_in_app_required: true
}

// A browser BFF adapter can implement this contract without exposing access tokens.

export interface NotificationService {
  list(cursor: string | null, signal: AbortSignal): Promise<NotificationPage>
  unreadCount(signal: AbortSignal): Promise<number>
  markRead(id: string, signal: AbortSignal): Promise<void>
  getPreferences(signal: AbortSignal): Promise<ChannelPreferences>
  savePreferences(
    channels: Pick<ChannelPreferences, 'email_enabled' | 'in_app_enabled'>,
    signal: AbortSignal,
  ): Promise<ChannelPreferences>
}

export type NotificationDestination =
  | '/app/notifications'
  | '/app/notifications/settings'
