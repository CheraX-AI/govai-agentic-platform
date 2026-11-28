export function relativeTime(value: string, now = Date.now()) {
  const seconds = Math.max(0, Math.floor((now - Date.parse(value)) / 1000))
  if (seconds < 60) return 'Just now'
  const format = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })
  if (seconds < 3600) return format.format(-Math.floor(seconds / 60), 'minute')
  if (seconds < 86400) return format.format(-Math.floor(seconds / 3600), 'hour')
  return format.format(-Math.floor(seconds / 86400), 'day')
}

export function dayGroup(value: string, now = new Date()) {
  const date = new Date(value)
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (date.toDateString() === now.toDateString()) return 'Today'
  if (date.toDateString() === yesterday.toDateString()) return 'Yesterday'
  return 'Earlier'
}
