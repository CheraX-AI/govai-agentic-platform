import type { Conversation } from '../chat/useConversation'
import { ChatComposer } from '../components/chat/ChatComposer'
import { Icon } from '../components/Icon'

export default function HomePage({ conversation, onOpenChat }: {
  conversation: Conversation
  onOpenChat: () => void
}) {
  return (
    <>
      <div className="home-hero">
        <p>Hello 👋</p>
        <h1>How can I help you today?</h1>
        <p>Ask me anything about government services in Sri Lanka.</p>
      </div>
      <div className="home-chat-entry">
        <ChatComposer conversation={conversation} onSend={onOpenChat} />
        <p className="guidance-disclaimer">
          GovAI can make mistakes. Check important details with the relevant government office.
        </p>
      </div>
      {conversation.error && <p role="alert">{conversation.error.message}</p>}
      {conversation.messages.length > 0 && (
        <a href="/app/chat">Open current conversation</a>
      )}
      <div className="suggestion-previews">
        {[
          ['search', 'Find government services'],
          ['status', 'Check application status'],
          ['request', 'Submit a request'],
          ['documents', 'Get required documents'],
        ].map(([icon, label]) => (
          <div className={`suggestion-preview ${icon}`} key={icon}>
            <span><Icon collection="shell" name={`suggest-${icon}`} /></span>
            <strong>{label}</strong>
          </div>
        ))}
      </div>
    </>
  )
}
