import assert from 'node:assert/strict'
import {sendChat,ChatError} from '../../src/services/chatApi.ts'
const original=globalThis.fetch
try {
 let count=0
 globalThis.fetch=async(url,options)=>{
  count++;assert.equal(options.credentials,'include');assert.equal(options.headers['X-GovAI-Web'],'1')
  if(String(url).endsWith('/csrf'))return Response.json({csrf_token:'A'.repeat(43)})
  assert.equal(options.headers['X-CSRF-Token'],'A'.repeat(43));assert.deepEqual(JSON.parse(options.body),{message:'Synthetic question'})
  return Response.json({answer:'Synthetic [1]',sources:[{number:1,title:'Reference',page:3},{number:2,title:'Guide',url:'https://example.test/guide'}]})
 }
 const signal=new AbortController().signal
 const reply=await sendChat(' Synthetic question ',signal);assert.equal(reply.answer,'Synthetic [1]');assert.deepEqual(reply.sources,[{number:1,title:'Reference',page:3},{number:2,title:'Guide',url:'https://example.test/guide'}]);assert.equal(count,2)
 await assert.rejects(sendChat(' ',signal),e=>e.status===400);assert.equal(count,2)
 await assert.rejects(sendChat('x'.repeat(4001),signal),e=>e.status===400)
 for(const status of [401,403,429,503,504]){
  globalThis.fetch=async(url)=>String(url).endsWith('/csrf')?Response.json({csrf_token:'A'.repeat(43)}):new Response('private upstream detail',{status})
  await assert.rejects(sendChat('Synthetic question',signal),e=>e instanceof ChatError&&e.status===status&&!e.message.includes('private upstream'))
 }
 for(const sources of [[{number:1,title:'bad',url:'javascript:bad'}],[{title:'no number'}],[{number:1,title:'A'},{number:1,title:'B'}],[{number:1,title:'A',page:0}],[{number:1,title:' '}]]){
  globalThis.fetch=async(url)=>String(url).endsWith('/csrf')?Response.json({csrf_token:'A'.repeat(43)}):Response.json({answer:'x',sources})
  await assert.rejects(sendChat('Synthetic question',signal),e=>e.status===502)
 }
 globalThis.fetch=async()=>{throw Error('private transport')}
 await assert.rejects(sendChat('Synthetic question',signal),/Unable to confirm/)
}finally{globalThis.fetch=original}
console.log('PASS: chat cookie/CSRF transport, bounded question, safe replies, numbered sources with optional page/url and error handling; no automatic retries')
