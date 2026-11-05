import assert from 'node:assert/strict'
import { resetPassword, takeResetLink, validateResetPasswords } from '../../src/services/recoveryApi.ts'
const token='A'.repeat(43)
const original=globalThis.fetch
try {
 let calls=0
 globalThis.fetch=async(url,options)=>{
  calls++
  assert.equal(String(url),'https://localhost:5001/api/v1/auth/reset-password')
  assert.equal(options.credentials,'omit')
  assert.equal(options.redirect,'error')
  assert.deepEqual(JSON.parse(options.body),{token,new_password:'abcdefgh',confirm_password:'abcdefgh'})
  return new Response(null,{status:204})
 }
 await resetPassword(token,'abcdefgh','abcdefgh')
 assert.equal(calls,1)
 await assert.rejects(resetPassword('bad','abcdefgh','abcdefgh'),/invalid/)
 assert.equal(calls,1)
 globalThis.fetch=async()=>Response.json({code:'invalid_or_expired_reset',error:'private detail'},{status:400})
 await assert.rejects(resetPassword(token,'abcdefgh','abcdefgh'),/invalid, expired or already used/)
 for(const status of [400,422,429,503]){
  globalThis.fetch=async()=>new Response('private detail',{status})
  await assert.rejects(resetPassword(token,'abcdefgh','abcdefgh'),e=>!e.message.includes('private detail'))
 }
 globalThis.fetch=async()=>{throw Error('private transport failure')}
 await assert.rejects(resetPassword(token,'abcdefgh','abcdefgh'),/Unable to confirm/)
}finally{globalThis.fetch=original}
for(const password of ['abcdefgh',' abcdefgh ','abcdef😀😀'])assert.equal(validateResetPasswords(password,password).password,undefined)
for(const password of ['1234567','a'.repeat(1025)])assert.ok(validateResetPasswords(password,password).password)
assert.ok(validateResetPasswords('abcdefgh','different').confirmation)
let replacement
const history={replaceState:(_state,_unused,url)=>{replacement=url}}
assert.deepEqual(takeResetLink({pathname:'/reset-password',hash:'#token='+token},history),{token})
assert.equal(replacement,'/reset-password')
assert.deepEqual(takeResetLink({pathname:'/reset-password',hash:'#token=bad'},history),{token:null})
assert.deepEqual(takeResetLink({pathname:'/',hash:'#token='+token},history),{token:null})
assert.equal(takeResetLink({pathname:'/',hash:'#/login'},history),null)
console.log('PASS: reset API payload/204, safe invalid/expired/error handling, no cookies/retries, password boundaries and fragment scrubbing')
