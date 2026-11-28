import assert from 'node:assert/strict'
import { createNotificationPreviewService } from '../src/dev/notificationPreviewService.ts'
import { notificationService } from '../src/services/notificationService.ts'
import { dayGroup, relativeTime } from '../src/notifications/presentation.ts'
import { allowedReturn } from '../src/routing/routes.ts'

// Verify preview mutations and production fallback without real accounts or APIs.

let requests = 0
const originalFetch = globalThis.fetch
globalThis.fetch = async () => { requests++; throw Error('Unexpected network request') }
try {
  const service = createNotificationPreviewService('ready')
  const { signal } = new AbortController()
  const first = await service.list(null, signal)
  assert.equal(first.items.length, 3)
  assert.equal(await service.unreadCount(signal), 2)
  const id = first.items[0].id
  await service.markRead(id, signal)
  await service.markRead(id, signal)
  assert.equal(await service.unreadCount(signal), 1, 'Mark-read is idempotent')
  const second = await service.list(first.next_cursor, signal)
  assert.equal(second.next_cursor, null)
  assert.equal(new Set([...first.items, ...second.items].map(row => row.id)).size, 4)
  const preferences = await service.savePreferences({
    email_enabled: false, in_app_enabled: false,
  }, signal)
  assert.equal(preferences.security_in_app_required, true)
  assert.equal((await service.getPreferences(signal)).email_enabled, false)
  assert.equal(await service.unreadCount(signal), 1, 'Preferences preserve inbox read state')
  assert.equal(await createNotificationPreviewService('ready').unreadCount(signal), 2)
  assert.equal((await createNotificationPreviewService('empty').list(null, signal)).items.length, 0)
  await assert.rejects(createNotificationPreviewService('error').list(null, signal))
  const cancel = new AbortController()
  const loading = createNotificationPreviewService('loading').list(null, cancel.signal)
  cancel.abort()
  await assert.rejects(loading, { name: 'AbortError' })
  await assert.rejects(notificationService.list(null, signal))
  await assert.rejects(notificationService.markRead(id, signal))
  assert.equal(requests, 0, 'No synthetic service or unavailable adapter calls a real API')
} finally { globalThis.fetch = originalFetch }

const noon = new Date(2026, 10, 27, 12)
assert.equal(dayGroup(noon.toISOString(), noon), 'Today')
assert.equal(dayGroup(new Date(2026, 10, 26, 12).toISOString(), noon), 'Yesterday')
assert.equal(dayGroup(new Date(2026, 10, 24, 12).toISOString(), noon), 'Earlier')
assert.equal(relativeTime(new Date(noon.getTime() - 120_000).toISOString(), +noon), '2 minutes ago')
assert.equal(allowedReturn('/app/notifications'), '/app/notifications')
assert.equal(allowedReturn('/app/notifications/settings'), '/app/notifications/settings')
assert.equal(allowedReturn('//evil.example/app/notifications'), '/app/home')
console.log('PASS notification fixture, cancellation, preferences, pagination and protected routes')
