import { useLayoutEffect, useRef, useState } from 'react'
import type { Conversation } from '../../chat/useConversation'
import { ChatMessage } from './ChatMessage'

export function ConversationView({ conversation, showDate = true }: {
  conversation: Conversation
  showDate?: boolean
}) {
  const viewport = useRef<HTMLDivElement>(null)
  const following = useRef(true)
  const [latest, setLatest] = useState(false)
  const previousCitizen = useRef<string | undefined>(undefined)

  useLayoutEffect(() => {
    const node = viewport.current
    if (!node) return
    const citizen = conversation.messages
      .filter(message => message.sender === 'citizen')
      .at(-1)?.id
    const ownSend = citizen !== previousCitizen.current
    previousCitizen.current = citizen

    // Reveal your own send, but do not drag you away from older messages for a reply.

    if (following.current || ownSend || !conversation.messages.length) {
      node.scrollTop = node.scrollHeight
      following.current = true
      setLatest(false)
    } else setLatest(true)
  }, [conversation.messages, conversation.status])

  return (
    <div className="conversation-wrapper">
      <div
        ref={viewport}
        className="conversation-scroll"
        role="region"
        aria-label="Conversation messages"
        tabIndex={0}
        onScroll={event => {
          const node = event.currentTarget
          following.current = node.scrollHeight - node.scrollTop - node.clientHeight < 80
          if (following.current) setLatest(false)
        }}
      >
        {!conversation.messages.length ? (
          <div className="chat-welcome">
            <h2>How can I help you today?</h2>
            <p>
              Ask a question in your own words. Each request sends only your latest
              message; earlier messages remain in this page.
            </p>
          </div>
        ) : showDate && <p className="chat-date">Today</p>}
        {conversation.messages.map(message => <ChatMessage key={message.id} message={message} />)}
        <div className="chat-feedback" role="status">
          {conversation.status === 'pending'
            ? 'Getting your answer...'
            : conversation.status === 'success'
              ? 'Reply received.'
              : conversation.status === 'cancelled'
                ? 'Request cancelled. The remote model may still be processing it.'
                : ''}
        </div>
        {conversation.error && (
          <div className="chat-error">
            <p role="alert">{conversation.error.message}</p>
            {conversation.canRetry && (
              <button type="button" onClick={conversation.retry}>
                Send again (may duplicate processing)
              </button>
            )}
          </div>
        )}
      </div>
      {latest && (
        <button
          type="button"
          className="chat-latest"
          onClick={() => {
            const node = viewport.current
            if (node) node.scrollTop = node.scrollHeight
            following.current = true
            setLatest(false)
            node?.focus()
          }}
        >
          Latest message
        </button>
      )}
    </div>
  )
}
