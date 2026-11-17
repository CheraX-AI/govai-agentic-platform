// Holds the backend conversation ID for the open chat. It is never shown in the UI.
// Each clear (New Chat, logout, rejected session) starts a new generation, so a
// reply to a request sent before the clear cannot bring the old ID back.

export type ConversationIdTracker = {
  current(): string | undefined
  snapshot(): number
  save(snapshot: number, id: string | undefined): boolean
  clear(): void
}

export function createConversationIdTracker(): ConversationIdTracker {
  let id: string | undefined
  let generation = 0

  return {
    // The ID to send; undefined until the backend has created the conversation
    current: () => id,

    // Taken when a request starts and passed back to save() with its reply
    snapshot: () => generation,

    save(snapshot, returned) {
      // A reply from before the last clear belongs to a chat that is gone
      if (snapshot !== generation) return false
      if (returned) id = returned
      return true
    },

    clear() {
      id = undefined
      generation++
    },
  }
}
