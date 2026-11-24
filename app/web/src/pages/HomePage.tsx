import type { Conversation } from '../chat/useConversation'
import { ChatComposer } from '../components/chat/ChatComposer'
import { Icon } from '../components/Icon'

const quickActions = [
  ['search', 'Find services', 'Which government service do I need to '],
  ['status', 'Check application status', 'How can I check the status of my '],
  ['documents', 'Required documents', 'What documents do I need for '],
] as const

export default function HomePage({ conversation, onOpenChat }: {
  conversation: Conversation
  onOpenChat: () => void
}) {
  return (
    <>
      <div className="home-hero">
        <p>Ayubowan 👋</p>
        <h1>How can I help you today?</h1>
        <p>Ask about any government service in Sri Lanka: documents, applications, fees and more.</p>
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
      <div className="suggestion-previews" aria-label="Quick actions">
        {quickActions.map(([icon, label, prompt]) => (
          <button
            type="button"
            className={`suggestion-preview ${icon}`}
            key={icon}
            onClick={() => {
              // Fill the input so the citizen can finish the question before sending.
              conversation.setDraft(prompt)
              requestAnimationFrame(() => {
                const input = document.getElementById('chat-message') as HTMLTextAreaElement | null
                input?.focus()
                input?.setSelectionRange(prompt.length, prompt.length)
              })
            }}
          >
            <span><Icon collection="shell" name={`suggest-${icon}`} /></span>
            {label}
          </button>
        ))}
      </div>
    </>
  )
}
