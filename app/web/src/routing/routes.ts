export type ProtectedDestination = '#/app/home' | '#/app/recent'
export function allowedReturn(value: string | null | undefined): ProtectedDestination {
  return value === '#/app/recent' ? value : '#/app/home'
}
export function loginDestination(destination: ProtectedDestination, expired = false) {
  return `#/login?returnTo=${encodeURIComponent(destination)}${expired ? '&reason=expired' : ''}`
}
export function readRoute(hash: string) {
  const [path, query = ''] = (hash || '#/login').split('?')
  const params = new URLSearchParams(query)
  return { path, returnTo: allowedReturn(params.get('returnTo')), expired: params.get('reason') === 'expired' }
}
export function pageTitle(path: string) {
  const titles: Record<string, string> = {
    '#/login': 'Login preview', '#/register': 'Registration preview',
    '#/forgot-password': 'Password recovery preview', '#/check-email': 'Check email preview',
    '#/verify-code': 'Recovery code verification preview', '#/reset-password': 'Password reset preview',
    '#/app/home': 'Home - mock application', '#/app/recent': 'Recent chats - mock application',
    '#/preview/home': 'Home layout preview', '#/preview/recent': 'Recent chats layout preview',
    '#/home': 'Home layout preview', '#/recent': 'Recent chats layout preview',
  }
  return titles[path] ?? 'Preview unavailable'
}
export function navigate(hash: string, replace = false) {
  if (replace) {
    history.replaceState(null, '', location.pathname + location.search + hash)
    dispatchEvent(new HashChangeEvent('hashchange'))
  } else location.hash = hash
}
