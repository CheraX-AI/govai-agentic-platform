// Conversation ID handling: chat API transport plus the hidden per-chat tracker used by useConversation.
// Run: node scripts/integration/check-conversation-id.mjs
import assert from 'node:assert/strict'
import { sendChat, ChatError } from '../../src/services/chatApi.ts'
import { createConversationIdTracker } from '../../src/chat/conversationId.ts'

const first = '3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34'
const other = '01936f3e-7b2a-7c4d-9e5f-0a1b2c3d4e5f'
const signal = new AbortController().signal
const original = globalThis.fetch

// A fake gateway: answers CSRF, records each chat body, and sends a fresh copy of the reply each call
function gateway(reply) {
  const bodies = []
  globalThis.fetch = async (url, options) => {
    if (String(url).endsWith('/csrf')) return Response.json({ csrf_token: 'A'.repeat(43) })
    bodies.push(JSON.parse(options.body))
    return Response.json(reply)
  }
  return bodies
}

try {
  // First message omits the ID; the backend's ID is returned to the caller
  let bodies = gateway({ answer: 'Synthetic answer', sources: [], conversation_id: first })
  let reply = await sendChat(' Synthetic question ', signal)
  assert.deepEqual(bodies[0], { message: 'Synthetic question' })
  assert.equal(reply.conversationId, first)

  // A later message sends the same ID and keeps it
  reply = await sendChat('Follow-up', signal, first)
  assert.deepEqual(bodies[1], { message: 'Follow-up', conversation_id: first })
  assert.equal(reply.conversationId, first)

  // Legacy backend without conversation_id still works
  gateway({ answer: 'Synthetic answer', sources: [] })
  reply = await sendChat('Synthetic question', signal)
  assert.equal(reply.conversationId, undefined)
  assert.equal('conversationId' in reply, false)

  // Returned IDs are validated, and a continued chat may not switch conversations
  for (const conversation_id of ['not-a-uuid', first.toUpperCase(), null, 7, '00000000-0000-0000-0000-000000000000', first + '\n']) {
    gateway({ answer: 'x', sources: [], conversation_id })
    await assert.rejects(sendChat('Synthetic question', signal), e => e instanceof ChatError && e.status === 502)
  }
  gateway({ answer: 'x', sources: [], conversation_id: other })
  await assert.rejects(sendChat('Synthetic question', signal, first), e => e.status === 502)
  gateway({ answer: 'x', sources: [] })
  await assert.rejects(sendChat('Synthetic question', signal, first), e => e.status === 502)

  // A malformed local ID is never sent
  bodies = gateway({ answer: 'x', sources: [] })
  await assert.rejects(sendChat('Synthetic question', signal, 'not-a-uuid'), e => e.status === 400)
  assert.equal(bodies.length, 0)

  // Unknown or another citizen's conversation keeps its 404 status for the hook
  globalThis.fetch = async url => String(url).endsWith('/csrf')
    ? Response.json({ csrf_token: 'A'.repeat(43) })
    : Response.json({ error: 'private detail', code: 'conversation_not_found' }, { status: 404 })
  await assert.rejects(sendChat('Synthetic question', signal, first), e => e.status === 404 && !e.message.includes('private'))
} finally {
  globalThis.fetch = original
}

// Tracker: first message has no ID, later ones reuse it
let tracker = createConversationIdTracker()
assert.equal(tracker.current(), undefined)
let snapshot = tracker.snapshot()
assert.equal(tracker.save(snapshot, first), true)
assert.equal(tracker.current(), first)
assert.equal(tracker.save(tracker.snapshot(), undefined), true)
assert.equal(tracker.current(), first, 'a reply without an ID keeps the existing one')

// New Chat (or logout, or a rejected session) clears the ID
tracker.clear()
assert.equal(tracker.current(), undefined)

// A reply to a request sent before New Chat cannot bring the old conversation back
tracker = createConversationIdTracker()
const beforeNewChat = tracker.snapshot()
tracker.clear()
assert.equal(tracker.save(beforeNewChat, first), false)
assert.equal(tracker.current(), undefined)
const afterNewChat = tracker.snapshot()
assert.equal(tracker.save(afterNewChat, other), true)
assert.equal(tracker.current(), other)

// An old reply arriving after the new chat has its own ID cannot overwrite it
assert.equal(tracker.save(beforeNewChat, first), false)
assert.equal(tracker.current(), other)

console.log('PASS: first message omits conversation_id, later messages reuse it, IDs validated, New Chat/logout/401 clear it, stale replies cannot overwrite a new chat')
