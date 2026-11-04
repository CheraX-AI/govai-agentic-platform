
export default function RecentPage({ protectedDemo = false }: { protectedDemo?: boolean }) {
  return <><div className="recent-preview-heading"><h1>Recent chats</h1><p>Shared layout and active navigation preview.</p></div><section className="history-unavailable"><h2>Chat history is not implemented</h2><p>This view demonstrates the shared header, sidebar, content spacing and selected navigation styling only. No conversations are loaded or stored.</p><a href={protectedDemo ? '#/app/home' : '#/preview/home'}>Return to Home preview</a></section></>
}
