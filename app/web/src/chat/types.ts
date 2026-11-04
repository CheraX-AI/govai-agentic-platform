export type SourceReference = { title: string; url: string }
export type Message = { id: string; sender: 'citizen' | 'ai'; text: string; createdAt: number; sources?: SourceReference[] }
export type ChatOutcome = 'success' | 'error'
export type ChatReply = { text: string; sources: SourceReference[] }
export type ChatRequest = { messages: { role: 'user' | 'assistant'; content: string }[] }
export type ChatService = (request: ChatRequest, options: { signal: AbortSignal; outcome: ChatOutcome }) => Promise<ChatReply>

export function safeSourceUrl(value: string): string | null {
  try {
    const url = new URL(value)
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? url.href : null
  } catch { return null }
}
