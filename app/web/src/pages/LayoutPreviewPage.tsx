import { Icon } from '../components/Icon'
import { ApplicationLayout } from '../components/layout/ApplicationLayout'
import type { LayoutPreview } from '../components/layout/Sidebar'
import { demoUser } from '../data/demoUser'

export default function LayoutPreviewPage({ page }: { page: LayoutPreview }) {
  return <ApplicationLayout page={page}>{page === 'home' ? <>
    <div className="home-hero"><p>Hello, {demoUser.displayName} 👋</p><h1>How can I help you today?</h1><p>Ask me anything about government services in Sri Lanka.</p></div>
    <div className="composer-preview" aria-label="Message composer preview; messaging unavailable"><button disabled aria-label="Attachments unavailable"><Icon collection="shell" name="plus" /></button><span>Ask about a service, fee or document</span><button disabled aria-label="Voice unavailable"><Icon collection="shell" name="mic" /></button><button className="send-preview" disabled aria-label="Sending unavailable"><Icon collection="shell" name="send" /></button></div>
    <div className="suggestion-previews">{[['search','Find government services'],['status','Check application status'],['request','Submit a request'],['documents','Get required documents']].map(([icon,label]) => <div className={`suggestion-preview ${icon}`} key={icon}><span><Icon collection="shell" name={`suggest-${icon}`} /></span><strong>{label}</strong></div>)}</div>
    <p className="guidance-disclaimer">GovAI can make mistakes. Check important details with the relevant government office.</p>
  </> : <><div className="recent-preview-heading"><h1>Recent chats</h1><p>Shared layout and active navigation preview.</p></div><section className="history-unavailable"><h2>Chat history is not implemented</h2><p>This view demonstrates the shared header, sidebar, content spacing and selected navigation styling only. No conversations are loaded or stored.</p><a href="#/home">Return to Home preview</a></section></>}</ApplicationLayout>
}
