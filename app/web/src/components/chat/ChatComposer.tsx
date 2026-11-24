import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Icon } from '../Icon'
import type { Conversation } from '../../chat/useConversation'

// Shared by Home, authenticated Chat and the isolated development preview.

export function ChatComposer({ conversation, onSend }: {
  conversation: Conversation
  onSend?: () => void
}) {
  const composing = useRef(false)
  const textarea = useRef<HTMLTextAreaElement>(null)

  // Wrapped text expands the box (buttons drop below) until the field is cleared.
  const [expanded, setExpanded] = useState(false)
  const resize = useCallback(() => {
    const node = textarea.current
    if (!node) return

    // Reset first so deleting text can shrink the field as well as grow it.

    node.style.height = '0px'
    const height = node.scrollHeight
    node.style.height = `${Math.min(160, Math.max(40, height))}px`
    node.style.overflowY = height > 160 ? 'auto' : 'hidden'
    setExpanded(previous => node.value !== '' && (previous || height > 44))
  }, [])

  // Re-measure after the layout switch, since the field width changes.
  useLayoutEffect(resize, [conversation.draft, expanded, resize])
  useEffect(() => {
    const node = textarea.current
    if (!node) return
    let width = node.clientWidth

    // A narrower viewport can wrap the same draft onto more lines.

    const observer = new ResizeObserver(() => {
      if (node.clientWidth !== width) {
        width = node.clientWidth
        resize()
      }
    })
    observer.observe(node)
    return () => observer.disconnect()
  }, [resize])

  function send() {
    if (conversation.status === 'pending' || !conversation.draft.trim()) return
    if (conversation.submit()) onSend?.()
  }

  return (
    <form
      className={`chat-composer${expanded ? ' expanded' : ''}`}
      aria-busy={conversation.status === 'pending'}
      onSubmit={event => {
        event.preventDefault()
        send()
      }}
    >
      <button type="button" className="chat-attach" disabled aria-label="Attachments unavailable">
        <Icon collection="shell" name="plus" />
      </button>
      <label className="sr-only" htmlFor="chat-message">Your message</label>
      <textarea
        ref={textarea}
        id="chat-message"
        rows={1}
        value={conversation.draft}
        placeholder="Message GovAI"
        aria-describedby="chat-input-help"
        aria-invalid={conversation.error?.status === 400 || undefined}
        onChange={event => conversation.setDraft(event.target.value)}
        onCompositionStart={() => { composing.current = true }}
        onCompositionEnd={() => { composing.current = false }}
        onKeyDown={event => {

          // 229 covers browsers that report IME confirmation as an Enter-like key.
          // requestSubmit keeps keyboard sends on the same path as the Send button.

          if (
            event.key === 'Enter' &&
            !event.shiftKey &&
            !composing.current &&
            !event.nativeEvent.isComposing &&
            event.keyCode !== 229
          ) {
            event.preventDefault()
            event.currentTarget.form?.requestSubmit()
          }
        }}
      />
      <button type="button" className="chat-mic" disabled aria-label="Microphone unavailable">
        <Icon collection="shell" name="mic" />
      </button>
      {/* While a reply loads, the send button becomes a stop button. */}
      {conversation.status === 'pending' ? (
        <button type="button" className="chat-send" aria-label="Stop response" onClick={conversation.cancel}>
          <svg aria-hidden="true" width="14" height="14" viewBox="0 0 14 14"><rect width="14" height="14" rx="3" fill="currentColor" /></svg>
        </button>
      ) : (
        <button
          type="submit"
          className="chat-send"
          disabled={!conversation.draft.trim()}
          aria-label="Send message"
        >
          {/* Nudged down-left: the plane's visual weight sits top-right of its box. */}
          <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ transform: 'translate(-1px, 1px)' }}>
            <path d="m22 2-7 20-4-9-9-4z" />
            <path d="M22 2 11 13" />
          </svg>
        </button>
      )}
      <span id="chat-input-help" className="sr-only">
        Enter to send. Shift+Enter for a new line. Each request sends one message,
        not the conversation history.
      </span>
    </form>
  )
}
