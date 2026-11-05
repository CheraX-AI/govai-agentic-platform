import { useCallback, useEffect, useLayoutEffect, useRef } from 'react'
import { Icon } from '../Icon'
import { Button } from '../Button'
import type { Conversation } from '../../chat/useConversation'

export function ChatComposer({ conversation, onSend }: { conversation: Conversation; onSend?: () => void }) {
  const composing = useRef(false)
  const textarea = useRef<HTMLTextAreaElement>(null)
  const resize = useCallback(() => {
    const node = textarea.current
    if (!node) return
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
    const observer = new ResizeObserver(() => {
      if (node.clientWidth !== width) { width = node.clientWidth; resize() }
    })
    observer.observe(node)
    return () => observer.disconnect()
  }, [resize])
  function send() { if (conversation.submit()) onSend?.() }
  return <form className="chat-composer" onSubmit={event => { event.preventDefault(); send() }}>
    <button type="button" disabled aria-label="Attachments unavailable"><Icon collection="shell" name="plus" /></button>
    <label className="sr-only" htmlFor="chat-draft">Your message</label>
    <textarea ref={textarea} id="chat-draft" rows={1} value={conversation.draft} placeholder="Message GovAI" aria-describedby="chat-input-help" onChange={event => conversation.setDraft(event.target.value)}
      onCompositionStart={() => { composing.current = true }} onCompositionEnd={() => { composing.current = false }}
      onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey && !composing.current && !event.nativeEvent.isComposing && event.keyCode !== 229) { event.preventDefault(); send() } }} />
    <button type="button" disabled aria-label="Microphone unavailable"><Icon collection="shell" name="mic" /></button>
    <Button type="submit" className="chat-send" disabled={conversation.status === 'pending' || !conversation.draft.trim()} aria-label="Send message"><Icon collection="shell" name="send" /></Button>
    <span id="chat-input-help" className="sr-only">Enter to send. Shift+Enter for a new line. Replies are simulated.</span>
  </form>
}
