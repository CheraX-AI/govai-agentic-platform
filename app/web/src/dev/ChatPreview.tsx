import { useMemo, useState } from 'react'
import { useConversation } from '../chat/useConversation'
import ChatPage from '../pages/ChatPage'
import { createChatPreviewService } from './chatPreviewService'
import '../styles/application.css'
import '../styles/chat.css'
import './chatPreview.css'

// Presentation preview only: App mounts it outside AuthProvider in development.

export default function ChatPreview() {
  const [outcome, setOutcome] = useState<'success' | 'error'>('success')
  const service = useMemo(() => createChatPreviewService(outcome), [outcome])
  const conversation = useConversation(service)

  return (
    <div className="development-chat-preview">
      <aside className="development-preview-notice" aria-label="Development preview notice">
        <strong>Development-only synthetic chat preview</strong>
        <p>
          No authenticated session, backend requests or real AI responses.
          Reload clears the conversation.
        </p>
        <label htmlFor="preview-result">Fixture result </label>
        <select
          id="preview-result"
          value={outcome}
          disabled={conversation.status === 'pending'}
          onChange={event => setOutcome(event.target.value === 'error' ? 'error' : 'success')}
        >
          <option value="success">Synthetic reply</option>
          <option value="error">Synthetic error</option>
        </select>
        {' '}<a href="/login">Go to login</a>
      </aside>
      <main className="application-content chat-content">
        <ChatPage conversation={conversation} />
      </main>
    </div>
  )
}
