import type { ChatSource } from '../services/chatApi'

export type SourceReference = ChatSource

export type Message = {
  id: string
  sender: 'citizen' | 'ai'
  text: string
  createdAt: number
  sources?: SourceReference[]
}

// The UI accepts either the existing gateway service or an explicitly injected fixture.

export type ChatService = (
  message: string,
  signal: AbortSignal,
) => Promise<{ answer: string; sources: ChatSource[] }>

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
