import type {
  CitizenNotification, ChannelPreferences, NotificationService,
} from '../notifications/types'

export type PreviewOutcome = 'ready' | 'empty' | 'error' | 'loading'

// This service is imported only by the development preview, never live sessions.

export function createNotificationPreviewService(outcome: PreviewOutcome): NotificationService {
  const now = Date.now()
  const rows: CitizenNotification[] = [
    {
      id: '123e4567-e89b-42d3-a456-426614174001', event_type: 'citizen.login.succeeded',
      priority: 'high', locale: 'en', title: 'Successful sign-in', read_at: null,
      created_at: new Date(now - 120_000).toISOString(),
      message_body: [
        'A successful sign-in to your GovAI account was recorded.',
        "If this wasn't you, open GovAI and use Forgot password on the login page" +
        ' to reset your password. Never share your password or verification code.',
      ].join('\n\n'),
    },
    {
      id: '123e4567-e89b-42d3-a456-426614174002', event_type: 'citizen.password.changed',
      priority: 'high', locale: 'en', title: 'Password changed', read_at: null,
      created_at: new Date(now - 3_600_000).toISOString(),
      message_body: [
        'Your GovAI password was successfully changed.',
        'If you made this change, no further action is needed.',
        "If you didn't make this change, open GovAI and use Forgot password" +
        ' on the login page to reset your password.',
        'Never share your password or verification code.',
      ].join('\n\n'),
    },
    {
      id: '123e4567-e89b-42d3-a456-426614174003', event_type: 'citizen.email.verified',
      priority: 'normal', locale: 'en', title: 'Email verified',
      created_at: new Date(now - 86_400_000).toISOString(),
      read_at: new Date(now - 85_000_000).toISOString(),
      message_body: 'Your email address has been verified for your GovAI account.',
    },
    {
      id: '123e4567-e89b-42d3-a456-426614174004', event_type: 'citizen.login.succeeded',
      priority: 'high', locale: 'en', title: 'Successful sign-in',
      created_at: new Date(now - 3 * 86_400_000).toISOString(),
      read_at: new Date(now - 2 * 86_400_000).toISOString(),
      message_body: 'A successful sign-in to your GovAI account was recorded.',
    },
  ]
  let preferences: ChannelPreferences = {
    email_enabled: true, in_app_enabled: true, locale: 'en', security_in_app_required: true,
  }

  async function wait(signal: AbortSignal) {
    signal.throwIfAborted()
    if (outcome === 'error') throw new Error('Synthetic notification failure.')
    await new Promise<void>((resolve, reject) => {
      const timer = outcome === 'loading' ? undefined : setTimeout(() => {
        signal.removeEventListener('abort', abort)
        resolve()
      }, 150)
      function abort() {
        clearTimeout(timer)
        reject(new DOMException('Preview cancelled.', 'AbortError'))
      }
      signal.addEventListener('abort', abort, { once: true })
    })
  }

  return {
    async list(cursor, signal) {
      await wait(signal)
      if (outcome === 'empty') return { items: [], next_cursor: null }
      const selected = cursor ? rows.slice(3) : rows.slice(0, 3)
      return {
        items: selected.map(row => ({ ...row })), next_cursor: cursor ? null : 'preview-next',
      }
    },
    async unreadCount(signal) {
      await wait(signal)
      return outcome === 'empty' ? 0 : rows.filter(row => row.read_at === null).length
    },
    async markRead(id, signal) {
      await wait(signal)
      const row = rows.find(item => item.id === id)
      if (!row) throw new Error('Synthetic notification not found.')
      row.read_at ??= new Date().toISOString()
    },
    async getPreferences(signal) { await wait(signal); return { ...preferences } },
    async savePreferences(channels, signal) {
      await wait(signal)
      preferences = { ...preferences, ...channels }
      return { ...preferences }
    },
  }
}
