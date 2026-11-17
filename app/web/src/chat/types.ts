import type { ChatReply, ChatSource } from '../services/chatApi'

export type SourceReference = ChatSource

export type Message = {
  id: string
  sender: 'citizen' | 'ai'
  text: string
  createdAt: number
  sources?: SourceReference[]
}

// The UI accepts either the existing gateway service or an explicitly injected fixture.
// conversationId is omitted for the first message of a chat.

export type ChatService = (
  message: string,
  signal: AbortSignal,
  conversationId?: string,
) => Promise<ChatReply>

export function safeSourceUrl(value: string): string | undefined {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password
      ? url.href
      : undefined
  } catch {
    return undefined
  }
}
