import assert from 'node:assert/strict'
import { browserAuthAdapter, WebAuthError } from '../../src/services/browserAuthAdapter.ts'
const original = globalThis.fetch
const calls = []
let denied = false
try {
  globalThis.fetch = async (url, options) => {
    const endpoint = new URL(url).pathname.split('/').pop()
    calls.push({ endpoint, options })
    assert.equal(options.credentials, 'include')
    assert.equal(options.headers['X-GovAI-Web'], '1')
    assert.equal(options.cache, 'no-store')
    if (denied) return new Response(JSON.stringify({code:'session_rejected',error:'private detail'}),{status:401})
    if(endpoint==='session') return Response.json({subject:'synthetic-subject'})
    if(endpoint==='csrf') return Response.json({csrf_token:'A'.repeat(43)})
    if(endpoint==='login') return Response.json({csrf_token:'A'.repeat(43),expires_at:'2099-01-01'})
    assert.equal(options.headers['X-CSRF-Token'],'A'.repeat(43))
    assert.equal(options.body,undefined)
    return new Response(null,{status:204})
  }
  const session=await browserAuthAdapter.login('test@example.test','synthetic password')
  assert.equal(session.kind,'server')
  assert.deepEqual(JSON.parse(calls[0].options.body),{identifier:'test@example.test',password:'synthetic password'})
  assert.equal((await browserAuthAdapter.restore()).userId,'synthetic-subject')
  await browserAuthAdapter.refresh()
  await browserAuthAdapter.logout()
  assert.equal('createMockSession' in browserAuthAdapter,false,'obsolete demo authentication is absent')
  denied=true
  assert.equal(await browserAuthAdapter.restore(),null)
  await browserAuthAdapter.logout()
  await assert.rejects(browserAuthAdapter.login('test@example.test','wrong'),error=>error instanceof WebAuthError && !error.message.includes('private detail'))
  globalThis.fetch=async()=>{throw new Error('private network detail')}
  await assert.rejects(browserAuthAdapter.restore(),/service is unavailable/)
} finally {globalThis.fetch=original}
console.log('PASS: web API headers, cookie custody, login payload, CSRF refresh/logout, restore rejection, absence of obsolete mock login and safe network errors')
