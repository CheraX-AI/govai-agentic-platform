import { useLayoutEffect, useRef, useState } from 'react'
import type { Conversation } from '../chat/useConversation'
import { ChatComposer } from '../components/chat/ChatComposer'
import { ConversationView } from '../components/chat/ConversationView'
import '../styles/chat.css'

export default function ChatPage({ conversation }: { conversation: Conversation }) {
  const [expanded, setExpanded] = useState(true)
  const page = useRef<HTMLElement>(null)
  const position = useRef<{
    top: number
    atBottom: boolean
    anchor?: HTMLElement
    offset?: number
  } | null>(null)

  function toggleHeader() {
    const node = page.current?.querySelector<HTMLElement>('.conversation-scroll')
    if (node) {

      // Header height changes the viewport; save the visible message as an anchor.

      const anchor = [...node.querySelectorAll<HTMLElement>('.chat-message')].find(
        message => message.getBoundingClientRect().bottom > node.getBoundingClientRect().top,
      )
      position.current = {
        top: node.scrollTop,
        atBottom: node.scrollHeight - node.clientHeight - node.scrollTop < 2,
        anchor,
        offset: anchor
          ? anchor.getBoundingClientRect().top - node.getBoundingClientRect().top
          : undefined,
      }
    }
    setExpanded(previous => !previous)
  }

  useLayoutEffect(() => {
    const node = page.current?.querySelector<HTMLElement>('.conversation-scroll')
    if (node && position.current) {
      const saved = position.current

      // Restore before paint so collapsing does not jump away from the reading position.

      if (saved.atBottom) node.scrollTop = node.scrollHeight
      else if (saved.anchor?.isConnected && saved.offset !== undefined) {
        node.scrollTop += saved.anchor.getBoundingClientRect().top
          - node.getBoundingClientRect().top - saved.offset
      } else node.scrollTop = saved.top
    }
    position.current = null
  }, [expanded])

  return (
    <section
      ref={page}
      className={`chat-page ${expanded ? '' : 'header-collapsed'}`}
      aria-label="Conversation"
    >
      <header className={`chat-heading ${expanded ? 'expanded' : 'collapsed'}`}>
        <div className="chat-heading-row">
          <div className="chat-title-group" hidden={!expanded}>
            <h1 id="chat-title">Your conversation</h1>
          </div>
          <div className="chat-heading-actions">
            <button className="chat-new" type="button" onClick={conversation.reset}>
              New chat
            </button>
            <button
              className="chat-header-toggle"
              type="button"
              aria-expanded={expanded}
              aria-controls="chat-title chat-header-details"
              aria-label={expanded ? 'Collapse conversation header' : 'Expand conversation header'}
              onClick={toggleHeader}
            >
              {expanded ? 'Collapse' : (
                <svg aria-hidden="true" width="18" height="18" viewBox="0 0 18 18">
                  <path
                    d="m4 6 5 5 5-5"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              )}
            </button>
          </div>
        </div>
        <div id="chat-header-details" hidden={!expanded}>
          <p><span aria-hidden="true" />GovAI assistant</p>
          <p>Each request sends one message. Conversation history is not sent or saved.</p>
        </div>
      </header>
      <ConversationView conversation={conversation} showDate={expanded} />
      <div className="chat-bottom">
        <ChatComposer conversation={conversation} />
        {conversation.status === 'pending' && (
          <button type="button" onClick={conversation.cancel}>Cancel request</button>
        )}
        <p className="guidance-disclaimer">
          GovAI can make mistakes. Verify important details with the relevant government office.
        </p>
      </div>
    </section>
  )
}
