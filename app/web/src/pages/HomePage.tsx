import type { Conversation } from '../chat/useConversation'
import { ChatComposer } from '../components/chat/ChatComposer'
import { Icon } from '../components/Icon'
import { demoUser } from '../data/demoUser'

export default function HomePage({ conversation, onOpenChat }: { conversation?: Conversation; onOpenChat?: () => void }) {
  return <>
    <div className="home-hero"><p>Hello, {demoUser.displayName} 👋</p><h1>How can I help you today?</h1><p>Ask me anything about government services in Sri Lanka.</p></div>
    {conversation ? <div className="home-chat-entry"><p>Simulated replies - no AI service is connected.</p><ChatComposer conversation={conversation} onSend={onOpenChat} /><a href="#/app/chat">Open current conversation</a></div> : <div className="composer-preview" aria-label="Message composer preview; messaging unavailable"><button disabled aria-label="Attachments unavailable"><Icon collection="shell" name="plus" /></button><span>Ask about a service, fee or document</span><button disabled aria-label="Voice unavailable"><Icon collection="shell" name="mic" /></button><button className="send-preview" disabled aria-label="Sending unavailable"><Icon collection="shell" name="send" /></button></div>}
    <div className="suggestion-previews">{[['search','Find government services'],['status','Check application status'],['request','Submit a request'],['documents','Get required documents']].map(([icon,label]) => <div className={`suggestion-preview ${icon}`} key={icon}><span><Icon collection="shell" name={`suggest-${icon}`} /></span>{conversation ? <button type="button" disabled={conversation.status === 'pending'} onClick={() => { if (conversation.submit(label)) onOpenChat?.() }}>{label}</button> : <strong>{label}</strong>}</div>)}</div>
    <p className="guidance-disclaimer">GovAI can make mistakes. Check important details with the relevant government office.</p>

  </>
}
