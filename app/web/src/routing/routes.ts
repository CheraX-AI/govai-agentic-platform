export type ProtectedDestination = '/app/home' | '/app/recent'
export function allowedReturn(value: string | null | undefined): ProtectedDestination {
  return value === '/app/recent' ? value : '/app/home'
}
export function loginDestination(destination: ProtectedDestination, expired = false) {
  return `/login?returnTo=${encodeURIComponent(destination)}${expired ? '&reason=expired' : ''}`
}
export function readRoute(pathname: string, search = '') {
  const path = pathname === '/' || pathname === '' ? '/login' : pathname.replace(/\/+$/, '')
  const params = new URLSearchParams(search)
  return { path, returnTo: allowedReturn(params.get('returnTo')), expired: params.get('reason') === 'expired' }
}
export function pageTitle(path: string) {
  const titles: Record<string, string> = {
    '/login': 'Log in', '/register': 'Create an account',
    '/forgot-password': 'Reset your password', '/check-email': 'Check your email',
    '/verify-code': 'Open a valid reset link', '/reset-password': 'Choose a new password',
    '/app/home': 'Home', '/app/recent': 'Recent chats',
  }
  return titles[path] ?? 'Page not found'
}
// Changes the address without reloading the page; AppRouter and AuthProvider
// listen for popstate, which the browser also fires for Back and Forward.
export function navigate(path: string, replace = false) {
  if (replace) history.replaceState(null, '', path)
  else history.pushState(null, '', path)
  dispatchEvent(new PopStateEvent('popstate'))
}
// Links inside the app (<a href="/login">) navigate without a full page reload.
export function handleLinkClick(event: MouseEvent) {
  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  const link = (event.target as Element | null)?.closest?.('a')
  if (!link || link.target || link.hasAttribute('download')) return
  const url = new URL(link.href, location.href)
  if (url.origin !== location.origin || url.hash) return
  event.preventDefault()
  navigate(url.pathname + url.search)
}
// Old bookmarks such as /#/login open the same page at /login.
export function upgradeHashLink() {
  if (location.hash.startsWith('#/')) history.replaceState(null, '', location.hash.slice(1))
}
