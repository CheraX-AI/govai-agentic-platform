import { useCallback, useEffect, useRef, useState } from 'react'
import { requestMockReply } from '../services/mockChatService'
import type { ChatOutcome, ChatService, Message } from './types'

export function useConversation(service: ChatService = requestMockReply) {
  const [messages, setMessages] = useState<Message[]>([])
  const [draft, setDraft] = useState('')
  const [outcome, setOutcome] = useState<ChatOutcome>('success')
  const [status, setStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle')
  const active = useRef<AbortController | null>(null)
  const failed = useRef<Message | null>(null)
  const reset = useCallback(() => {
    active.current?.abort(); active.current = null; failed.current = null
    setMessages([]); setDraft(''); setStatus('idle'); setOutcome('success')
  }, [])
  useEffect(() => () => { active.current?.abort(); active.current = null }, [])

  async function request(message: Message, context: Message[]) {
    const controller = new AbortController()
    active.current = controller; failed.current = null; setStatus('pending')
    try {
      // Send the complete active context, including the pending citizen turn once.
      // The service owns protocol mapping; UI messages never contain auth credentials.
      const reply = await service({ messages: context.map(turn => ({ role: turn.sender === 'citizen' ? 'user' : 'assistant', content: turn.text })) }, { signal: controller.signal, outcome })
      if (controller.signal.aborted || active.current !== controller) return
      setMessages(previous => [...previous, { id: crypto.randomUUID(), sender: 'ai', createdAt: Date.now(), ...reply }])
      setStatus('success')
    } catch {
      if (controller.signal.aborted || active.current !== controller) return
      failed.current = message; setStatus('error')
    } finally { if (active.current === controller) active.current = null }
  }
  function submit(text = draft) {
    if (active.current || !text.trim()) return false
    const message: Message = { id: crypto.randomUUID(), sender: 'citizen', text, createdAt: Date.now() }
    const context = [...messages, message]
    setMessages(context); setDraft(''); void request(message, context)
    return true
  }
  function retry() { if (!active.current && failed.current) void request(failed.current, messages) }
  return { messages, draft, setDraft, outcome, setOutcome, status, submit, retry, reset }
}

export type Conversation = ReturnType<typeof useConversation>
