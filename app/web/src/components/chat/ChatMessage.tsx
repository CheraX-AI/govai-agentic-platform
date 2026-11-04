import type { Message } from '../../chat/types'
import { SourceReferences } from './SourceReferences'

export function ChatMessage({ message }: { message: Message }) {
  return <article className={`chat-message ${message.sender}`} aria-label={`${message.sender === 'citizen' ? 'You' : 'GovAI simulated reply'} message`}>
    <div className="message-meta"><strong>{message.sender === 'citizen' ? 'You' : 'GovAI'}</strong><time dateTime={new Date(message.createdAt).toISOString()}>{new Date(message.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></div>
    <div className="message-bubble">{message.text}</div>
    {message.sender === 'ai' && message.sources && <SourceReferences sources={message.sources} />}
  </article>
}
