import { useCallback, useEffect, useLayoutEffect, useRef } from 'react'
import { Icon } from '../Icon'
import { Button } from '../Button'
import type { Conversation } from '../../chat/useConversation'

// Shared by Home, authenticated Chat and the isolated development preview.

export function ChatComposer({ conversation, onSend }: {
  conversation: Conversation
  onSend?: () => void
}) {
  const composing = useRef(false)
  const textarea = useRef<HTMLTextAreaElement>(null)
  const resize = useCallback(() => {
    const node = textarea.current
    if (!node) return

    // Reset first so deleting text can shrink the field as well as grow it.

    node.style.height = '0px'
    const height = node.scrollHeight
    node.style.height = `${Math.min(160, Math.max(44, height))}px`
    node.style.overflowY = height > 160 ? 'auto' : 'hidden'
  }, [])

  useLayoutEffect(resize, [conversation.draft, resize])
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
      className="chat-composer"
      aria-busy={conversation.status === 'pending'}
      onSubmit={event => {
        event.preventDefault()
        send()
      }}
    >
      <button type="button" disabled aria-label="Attachments unavailable">
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
      <button type="button" disabled aria-label="Microphone unavailable">
        <Icon collection="shell" name="mic" />
      </button>
      <Button
        type="submit"
        className="chat-send"
        disabled={conversation.status === 'pending' || !conversation.draft.trim()}
        aria-label="Send message"
      >
        <Icon collection="shell" name="send" />
      </Button>
      <span id="chat-input-help" className="sr-only">
        Enter to send. Shift+Enter for a new line. Each request sends one message,
        not the conversation history.
      </span>
    </form>
  )
}
