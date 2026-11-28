import { useCallback, useEffect, useRef, useState } from 'react'
import type { CitizenNotification, NotificationService } from './types'

export function useNotificationData(service: NotificationService) {
  const [items, setItems] = useState<CitizenNotification[]>([])
  const [unread, setUnread] = useState<number | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [cursor, setCursor] = useState<string | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  const [moreError, setMoreError] = useState(false)
  const lifetime = useRef({
    version: 0,
    request: null as AbortController | null,
    moreRequest: null as AbortController | null,
  })

  // Replacement requests invalidate earlier results, including queued pagination.

  const refresh = useCallback(async () => {
    const state = lifetime.current
    const current = ++state.version
    state.request?.abort()
    state.moreRequest?.abort()
    const controller = new AbortController()
    state.request = controller
    setStatus('loading')
    setLoadingMore(false)
    setMoreError(false)
    try {
      const [page, count] = await Promise.all([
        service.list(null, controller.signal), service.unreadCount(controller.signal),
      ])
      if (controller.signal.aborted || state.version !== current) return
      setItems(page.items)
      setCursor(page.next_cursor)
      setUnread(count)
      setStatus('ready')
    } catch {
      if (controller.signal.aborted || state.version !== current) return
      setStatus('error')
      setUnread(null)
    }
  }, [service, lifetime])

  useEffect(() => {
    const state = lifetime.current
    let mounted = true
    queueMicrotask(() => { if (mounted) void refresh() })
    return () => {
      mounted = false
      ++state.version
      state.request?.abort()
      state.moreRequest?.abort()
    }
  }, [refresh, lifetime])

  const loadMore = useCallback(async () => {
    if (!cursor || loadingMore || status !== 'ready') return
    const state = lifetime.current
    const current = state.version
    const controller = new AbortController()
    state.moreRequest?.abort()
    state.moreRequest = controller
    setLoadingMore(true)
    setMoreError(false)
    try {
      const page = await service.list(cursor, controller.signal)
      if (controller.signal.aborted || state.version !== current) return
      if (page.next_cursor === cursor) throw new Error('Cursor did not advance.')
      setItems(previous => {
        const existing = new Set(previous.map(item => item.id))
        return [...previous, ...page.items.filter(item => !existing.has(item.id))]
      })
      setCursor(page.next_cursor)
    } catch {
      if (!controller.signal.aborted && state.version === current) setMoreError(true)
    } finally {
      if (!controller.signal.aborted && state.version === current) setLoadingMore(false)
    }
  }, [cursor, loadingMore, service, status, lifetime])

  return {
    items, unread, status, hasMore: cursor !== null, loadingMore, moreError, refresh, loadMore,
  }
}
