import type { ChatReply, ChatService } from '../chat/types'

export function sampleReply(text: string): ChatReply {
  const topic = /\bnic\b/i.test(text) ? 'NIC services' : /passport/i.test(text) ? 'passport services' : /vehicle|licence/i.test(text) ? 'vehicle services' : null
  return {
    text: topic
      ? `Simulated reply - this is a demonstration about ${topic}, not verified government guidance.\n\n1. Identify the service you want to ask about.\n2. Make a list of your questions and documents to check.\n3. Confirm current requirements with the responsible office before acting.\n\nThis mock does not look up eligibility, fees or application procedures.`
      : 'Simulated reply - this is a temporary test fixture, not an AI answer to your question.\n\nYou can enter any free-text question and continue with follow-ups. The interface keeps the full active conversation and passes it to the replaceable service boundary.\n\nNo AI service was contacted. This mock cannot accurately answer arbitrary questions. Verify important details independently.',
    sources: [{ title: 'Example source - demonstration only (not evidence)', url: 'https://example.com/' }],
  }
}

// Replace this boundary under GA-61. No input leaves the browser.
export const requestMockReply: ChatService = (request, { signal, outcome }) => new Promise((resolve, reject) => {
  if (signal.aborted) { reject(new DOMException('Cancelled', 'AbortError')); return }
  const abort = () => { clearTimeout(timer); reject(new DOMException('Cancelled', 'AbortError')) }
  const timer = setTimeout(() => {
    signal.removeEventListener('abort', abort)
    if (outcome === 'error') reject(Error('Simulated chat failure'))
    else resolve(sampleReply(request.messages.filter(turn => turn.role === 'user').at(-1)?.content ?? ''))
  }, 1000)
  signal.addEventListener('abort', abort, { once: true })
})
