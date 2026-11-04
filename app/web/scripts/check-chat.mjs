import assert from 'node:assert/strict'
import { safeSourceUrl } from '../src/chat/types.ts'
import { sampleReply, requestMockReply } from '../src/services/mockChatService.ts'
import { allowedReturn } from '../src/routing/routes.ts'

for (const url of ['javascript:alert(1)', 'data:text/html,test', '/relative', 'not a url', 'file:///etc/passwd', 'https://user:pass@example.com']) assert.equal(safeSourceUrl(url), null)
assert.equal(safeSourceUrl('https://example.com'), 'https://example.com/')
assert.equal(safeSourceUrl('http://example.com/test'), 'http://example.com/test')
for (const topic of ['NIC renewal', 'passport', 'vehicle licence', '<img src=x onerror=alert(1)>']) {
  const reply = sampleReply(topic)
  assert.match(reply.text, /Simulated reply/)
  assert.ok(reply.sources.every(source => safeSourceUrl(source.url) === 'https://example.com/'))
  assert.match(reply.sources[0].title, /demonstration only.*not evidence/)
}
assert.match(sampleReply('unrecognised topic').text, /cannot accurately answer arbitrary questions/)
const controller = new AbortController()
const cancelled = requestMockReply({ messages: [{ role: 'user', content: 'sample' }] }, { signal: controller.signal, outcome: 'success' })
controller.abort()
await assert.rejects(cancelled, { name: 'AbortError' })
await assert.rejects(requestMockReply({ messages: [{ role: 'user', content: 'sample' }] }, { signal: controller.signal, outcome: 'success' }), { name: 'AbortError' })
await assert.rejects(requestMockReply({ messages: [{ role: 'user', content: 'sample' }] }, { signal: new AbortController().signal, outcome: 'error' }), /Simulated chat failure/)
assert.deepEqual(await requestMockReply({ messages: [{ role: 'user', content: 'NIC' }] }, { signal: new AbortController().signal, outcome: 'success' }), sampleReply('NIC'))
assert.equal(allowedReturn('#/app/chat'), '#/app/chat')
assert.equal(allowedReturn('#/app/chat/elsewhere'), '#/app/home')
console.log('PASS synthetic topic/fallback fixtures, URL allowlist, mock success/error/cancellation, protected Chat return allowlist')
