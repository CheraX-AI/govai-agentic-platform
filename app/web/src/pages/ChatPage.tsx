import type { Conversation } from '../chat/useConversation'
import { ChatComposer } from '../components/chat/ChatComposer'
import { ConversationView } from '../components/chat/ConversationView'
import '../styles/chat.css'

export default function ChatPage({ conversation }: { conversation: Conversation }) {
  return (
    <section className="chat-page" aria-label="Conversation">
      <ConversationView conversation={conversation} />
      <div className="chat-bottom">
        <ChatComposer conversation={conversation} />
        <p className="guidance-disclaimer">
          GovAI can make mistakes. Verify important details with the relevant government office.
        </p>
      </div>
    </section>
  )
}
