import assert from 'node:assert/strict'
import { requestRecovery } from '../../src/services/recoveryApi.ts'
const original=globalThis.fetch
try {
 let calls=0
 globalThis.fetch=async(url,options)=>{
  calls++
  assert.equal(String(url),'https://localhost:5001/api/v1/auth/forgot-password')
  assert.equal(options.credentials,'omit')
  assert.equal(options.cache,'no-store')
  assert.deepEqual(JSON.parse(options.body),{nic:'123456789V'})
  return Response.json({status:'received',masked_email:'te***@example.test'},{status:202})
 }
 assert.deepEqual(await requestRecovery(' 123456789v '),{nic:'123456789V',maskedEmail:'te***@example.test'})
 assert.equal(calls,1)
 globalThis.fetch=async()=>Response.json({status:'received'},{status:202})
 assert.equal((await requestRecovery('123456789V')).maskedEmail,undefined)
 for(const status of [400,422,429,503]){
  globalThis.fetch=async()=>new Response('private upstream error',{status})
  await assert.rejects(requestRecovery('123456789V'),e=>!e.message.includes('private upstream'))
 }
 globalThis.fetch=async()=>Response.json({status:'received',masked_email:42},{status:202})
 await assert.rejects(requestRecovery('123456789V'),/Unable to confirm/)
 globalThis.fetch=async()=>{throw Error('private failure')}
 await assert.rejects(requestRecovery('123456789V'),/Unable to confirm delivery/)
}finally{globalThis.fetch=original}
console.log('PASS: normalized NIC-only request, no cookies, real receipt, unknown-account response, cooldown, safe dependency errors and malformed receipt')
