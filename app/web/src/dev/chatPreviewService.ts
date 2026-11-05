import { ChatError } from '../services/chatApi.ts'
import type { ChatService } from '../chat/types.ts'

// Explicit development preview only. Never a fallback from a failed real request.

export function createChatPreviewService(outcome: 'success' | 'error' = 'success'): ChatService {
  return (_message, signal) => new Promise((resolve, reject) => {
    signal.throwIfAborted()
    const abort = () => {
      clearTimeout(timer)
      reject(new DOMException('Preview cancelled', 'AbortError'))
    }

    // A short delay lets the same UI exercise pending and cancellation without I/O.

    const timer = setTimeout(() => {
      signal.removeEventListener('abort', abort)
      if (outcome === 'error') {
        reject(new ChatError(503, 'Synthetic preview error. No backend request was made.'))
      } else {
        resolve({
          answer: 'Synthetic preview reply [2]. This fixture is not AI-generated government guidance. No backend request was made.',
          sources: [{ number: 2, title: 'Synthetic document - preview only', page: 3 }],
        })
      }
    }, 600)
    signal.addEventListener('abort', abort, { once: true })
  })
}
