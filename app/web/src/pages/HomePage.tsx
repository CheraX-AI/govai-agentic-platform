import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { sendChat, ChatError } from '../services/chatApi'
import type { ChatReply } from '../services/chatApi'
import { useAuth } from '../auth/AuthContext'
import { Icon } from '../components/Icon'

export default function HomePage() {
  const auth = useAuth()
  const [message, setMessage] = useState('')
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [exchange, setExchange] = useState<{question: string;reply: ChatReply} | null>(null)
  const active = useRef<AbortController | null>(null)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; active.current?.abort() } }, [])
  const enabled = auth.state.status === 'authenticated'
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!enabled || active.current) return
    const controller = new AbortController()
    active.current = controller; setPending(true); setFailure('')
    const question = message.trim()
    try {
      const reply = await sendChat(question, controller.signal)
      if (mounted.current) { setExchange({question,reply}); setMessage('') }
    } catch (error) {
      if (mounted.current && !controller.signal.aborted) {
        setFailure(error instanceof Error ? error.message : 'Chat is unavailable.')
        if (error instanceof ChatError && error.status === 401) void auth.retry()
      }
    } finally { active.current = null; if (mounted.current) setPending(false) }
  }
  return <>
    <div className="home-hero"><p>Hello 👋</p><h1>How can I help you today?</h1><p>Ask me anything about government services in Sri Lanka.</p></div>
    {exchange && <section className="chat-exchange" aria-label="Latest chat answer"><p className="chat-question">{exchange.question}</p><p className="chat-answer">{exchange.reply.answer}</p>{exchange.reply.sources.length > 0 && <ul aria-label="Sources">{exchange.reply.sources.map(source => {
      // "[1] TIN guide, p. 3", linked only when the document has a public address
      const label = `${source.title}${source.page ? `, p. ${source.page}` : ''}`
      return <li key={source.number}>[{source.number}] {source.url ? <a href={source.url} target="_blank" rel="noopener noreferrer">{label}</a> : label}</li>
    })}</ul>}</section>}
    <form className="composer-preview" aria-label="Chat message" onSubmit={submit} aria-busy={pending}><button type="button" disabled aria-label="Attachments unavailable"><Icon collection="shell" name="plus" /></button><textarea id="chat-message" aria-label="Your question" rows={1} placeholder="Ask about a service, fee or document" value={message} disabled={!enabled || pending} onChange={event=>setMessage(event.target.value)} /><button type="button" disabled aria-label="Voice unavailable"><Icon collection="shell" name="mic" /></button><button type="submit" className="send-preview" disabled={!enabled || pending || !message.trim()} aria-label="Send question"><Icon collection="shell" name="send" /></button></form>
    {pending && <p role="status">Getting your answer...</p>}
    {failure && <p role="alert">{failure}</p>}
    <div className="suggestion-previews">{[['search','Find government services'],['status','Check application status'],['request','Submit a request'],['documents','Get required documents']].map(([icon,label]) => <div className={`suggestion-preview ${icon}`} key={icon}><span><Icon collection="shell" name={`suggest-${icon}`} /></span><strong>{label}</strong></div>)}</div>
    <p className="guidance-disclaimer">GovAI can make mistakes. Check important details with the relevant government office.</p>

  </>
}
