export default function RecentPage({ conversationAvailable = false }: {
  conversationAvailable?: boolean
}) {
  return (
    <>
      <div className="recent-preview-heading">
        <h1>Recent chats</h1>
        <p>Your previous conversations.</p>
      </div>
      <section className="history-unavailable">
        <h2>Chat history is not available yet</h2>
        <p>Conversations are not saved yet. Start a new chat from Home.</p>
        {conversationAvailable && (
          <p><a href="/app/chat">Open the current conversation in this tab</a></p>
        )}
        <a href="/app/home">Go to Home</a>
      </section>
    </>
  )
}
