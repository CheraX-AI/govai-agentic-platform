import { useCallback, useEffect, useRef, useState } from 'react'
import { sendChat, ChatError } from '../services/chatApi'
import type { ChatService, Message } from './types'

// Owns only in-memory UI state. The default uses the existing real gateway;
// the isolated development preview explicitly supplies its synthetic service.

export function useConversation(
  service: ChatService = sendChat,
  onSessionRejected?: () => void,
) {
  const [messages, setMessages] = useState<Message[]>([])
  const [draft, setDraft] = useState('')
  const [status, setStatus] = useState<
    'idle' | 'pending' | 'success' | 'error' | 'cancelled'
  >('idle')
  const [error, setError] = useState<ChatError | null>(null)
  const [canRetry, setCanRetry] = useState(false)
  const active = useRef<AbortController | null>(null)
  const failed = useRef<Message | null>(null)

  useEffect(() => () => {
    active.current?.abort()
    active.current = null
  }, [service])

  const cancel = useCallback(() => {
    active.current?.abort()
    active.current = null
    failed.current = null
    setStatus('cancelled')
    setError(null)
    setCanRetry(false)
  }, [])

  const reset = useCallback(() => {
    active.current?.abort()
    active.current = null
    failed.current = null
    setMessages([])
    setDraft('')
    setStatus('idle')
    setError(null)
    setCanRetry(false)
  }, [])

  async function request(message: Message) {
    const controller = new AbortController()

    // This ref also blocks duplicate submissions before React publishes pending state.

    active.current = controller
    failed.current = null
    setStatus('pending')
    setError(null)
    setCanRetry(false)

    try {

      // The backend accepts one message, not the local transcript.

      const reply = await service(message.text, controller.signal)

      // Cancellation/reset may win even when a service resolves after being aborted.

      if (controller.signal.aborted || active.current !== controller) return
      setMessages(previous => [
        ...previous,
        {
          id: crypto.randomUUID(),
          sender: 'ai',
          text: reply.answer,
          sources: reply.sources,
          createdAt: Date.now(),
        },
      ])
      setStatus('success')
    } catch (caught) {
      if (controller.signal.aborted || active.current !== controller) return
      const next = caught instanceof ChatError
        ? caught
        : new ChatError(503, 'Chat is temporarily unavailable. Your messages are still here.')
      failed.current = [400, 401, 403, 413].includes(next.status) ? null : message
      setCanRetry(failed.current !== null)
      setError(next)
      setStatus('error')
      if (next.status === 401) onSessionRejected?.()
    } finally {
      if (active.current === controller) active.current = null
    }
  }

  function submit(text = draft) {
    if (active.current) return false
    const value = text.trim()
    if (
      !value ||
      Array.from(value).length > 4000 ||
      new TextEncoder().encode(JSON.stringify({ message: value })).length > 16384
    ) {
      setError(new ChatError(
        400,
        'Enter a question of at most 4,000 Unicode characters within the 16 KiB message limit.',
      ))
      return false
    }
    const message: Message = {
      id: crypto.randomUUID(),
      sender: 'citizen',
      text: value,
      createdAt: Date.now(),
    }
    setMessages(previous => [...previous, message])
    setDraft('')
    void request(message)
    return true
  }

  // Retrying resends the existing turn; it must not append the citizen message twice.
  // The UI makes this explicit because the backend does not guarantee idempotency.

  function retry() {
    if (!active.current && failed.current) void request(failed.current)
  }

  return { messages, draft, setDraft, status, error, submit, retry, canRetry, cancel, reset }
}

export type Conversation = ReturnType<typeof useConversation>
