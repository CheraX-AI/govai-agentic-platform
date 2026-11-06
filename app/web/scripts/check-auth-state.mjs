import assert from 'node:assert/strict'
import { createMockAuthAdapter, parseMockSession, mockSessionKey, mockSessionLifetime } from '../src/services/mockAuthAdapter.ts'
import { allowedReturn, readRoute } from '../src/routing/routes.ts'
const memory=new Map();const port={getItem:key=>memory.get(key)??null,setItem:(key,value)=>memory.set(key,value),removeItem:key=>memory.delete(key)}
let now=100000;const adapter=createMockAuthAdapter(()=>port,true,()=>now)
assert.equal(await adapter.restore(),null);const session=await adapter.createMockSession();assert.equal(session.expiresAt,now+mockSessionLifetime);assert.deepEqual(await adapter.restore(),session)
const raw=JSON.parse(memory.get(mockSessionKey));assert.deepEqual(Object.keys(raw).sort(),['createdAt','expiresAt','kind','userId','version']);assert.equal(raw.userId,'synthetic-harshan')
for(const value of ['bad', 'null','[]',JSON.stringify({...raw,password:'not-allowed'}),JSON.stringify({...raw,userId:'actual-person'}),JSON.stringify({...raw,version:2}),JSON.stringify({...raw,expiresAt:now+mockSessionLifetime+1}),JSON.stringify({...raw,createdAt:now+1}),JSON.stringify({...raw,expiresAt:now}),JSON.stringify({...raw,expiresAt:'later'})]){assert.equal(parseMockSession(value,now),null);port.setItem(mockSessionKey,value);assert.equal(await adapter.restore(),null);assert.equal(port.getItem(mockSessionKey),null)}
await adapter.createMockSession();now+=mockSessionLifetime;assert.equal(await adapter.restore(),null);assert.equal(memory.size,0);await adapter.createMockSession();await adapter.logout();assert.equal(memory.size,0)
const disabled=createMockAuthAdapter(()=>{throw Error('production storage must not be read')},false);assert.equal(await disabled.restore(),null);await assert.rejects(disabled.createMockSession());await disabled.logout()
const blocked=createMockAuthAdapter(()=>{throw Error('storage blocked')},true);await assert.rejects(blocked.restore());await assert.rejects(blocked.createMockSession());await assert.rejects(blocked.logout())
for(const path of ['https://example.com','#/preview/home','//example.com','javascript:alert(1)','#/app/home/extra',undefined])assert.equal(allowedReturn(path),'#/app/home');assert.equal(allowedReturn('#/app/recent'),'#/app/recent');assert.equal(readRoute('#/login?returnTo=%23%2Fapp%2Frecent').returnTo,'#/app/recent');assert.equal(readRoute('').path,'#/login')
console.log('PASS bounded synthetic metadata schema, invalid/expired cleanup, restore/logout/storage errors, production disabled without storage reads, internal return allowlist')
