import assert from 'node:assert/strict'
import { openBrowser, origin, delay } from './browserHarness.mjs'
import { installAuthFixtures } from './authFixtures.mjs'
export async function runAuthBrowserCheck(scenario) {
 const b=await openBrowser()
 const {send,evaluate,until,go,fill}=b
 try {
  const model=await installAuthFixtures(b.socket,send)
  const submit=()=>evaluate('document.querySelector("button[type=submit]").click()')
  const text=()=>evaluate('document.body.innerText')
  const login=async(identifier='test@example.test')=>{await fill('identifier',identifier);await fill('password','Synthetic!123');await submit()}
  const logout=async()=>{await evaluate('[...document.querySelectorAll("button")].find(x=>x.textContent==="Log out").click()');await until('location.hash.startsWith("#/login")','logout')}
  if(scenario==='login'||scenario==='navigation') {
   await go('#/login');await until('!document.querySelector("button[type=submit]").disabled','session check')
   assert.equal(await evaluate('Boolean(document.querySelector(".login-method"))'),false)
   assert.equal(await evaluate('document.getElementById("human-check").disabled'),true)
   await submit();assert.equal(await evaluate('document.activeElement.id'),'identifier')
   model.failLogin=true;await login();await until('document.body.innerText.includes("Invalid email/NIC")','invalid login')
   assert.equal(await evaluate('document.getElementById("password").value.length'),0)
   model.failLogin=false;await login();await until('location.hash==="#/app/home"','protected home')
   await send('Page.reload');await until('document.body.innerText.includes("Signed in securely")','restore on reload')
   const cookies=await send('Network.getCookies',{urls:['https://localhost:5001']})
   assert.ok(cookies.cookies.some(c=>c.name==='__Host-govai_session'&&c.httpOnly&&c.secure&&c.sameSite==='Strict'))
   await logout();await go('#/app/home');await until('location.hash.startsWith("#/login")','protected route rejected')
   await login('123456789V');await until('location.hash==="#/app/home"','NIC login');await logout()
   assert.ok(model.calls.filter(c=>c.path.endsWith('/web/login')).every(c=>c.keys.join(',')==='identifier,password'))
  } else if(scenario==='registration') {
   await go('#/register');await submit();assert.equal(await evaluate('document.activeElement.id'),'register-fullName')
   const fields={fullName:'Synthetic Test',nic:'123456789V',phone:'+447911123456',email:'test@example.test',password:'abcdefgh',confirmation:'abcdefgh'}
   for(const [id,value]of Object.entries(fields))await fill('register-'+id,value)
   await evaluate('document.getElementById("register-terms").click()')
   model.registrationStatus=503;await submit();await until('document.body.innerText.includes("temporarily unavailable")','registration failure')
   assert.doesNotMatch(await text(),/Request received/)
   await fill('register-password','abcdefgh');await fill('register-confirmation','abcdefgh')
   model.registrationStatus=202;await evaluate('document.querySelector("form").requestSubmit();document.querySelector("form").requestSubmit()')
   await until('document.body.innerText.includes("Request received")','receipt')
   assert.equal(model.calls.filter(c=>c.path.endsWith('/registrations')).length,2,'one request per submission')
   assert.equal(await evaluate('document.getElementById("register-password").value.length'),0)
   assert.ok(await evaluate('[...document.links].some(a=>a.href === "http://localhost:8025/")'))
   const call=model.calls.find(c=>c.path.endsWith('/registrations'))
   assert.deepEqual(call.keys,['email','full_name','mobile_number','nic','password'])
   assert.ok(call.headers.includes('idempotency-key'))
  } else if(scenario==='recovery') {
   await go('#/forgot-password')
   assert.equal(await evaluate('document.getElementById("recovery-phone").disabled'),true)
   await submit();assert.equal(await evaluate('document.activeElement.id'),'recovery-email')
   await evaluate('document.getElementById("recovery-email").click()');await fill('recovery-nic','123456789V');await evaluate('document.getElementById("recovery-human").click()')
   model.recoveryStatus=503;await submit();await until('document.body.innerText.includes("temporarily unavailable")','request error')
   assert.equal(await evaluate('location.hash'),'#/forgot-password')
   model.recoveryStatus=202;await submit();await until('location.hash==="#/check-email"','email receipt')
   assert.match(await text(),/te\*\*\*@example.test/)
   assert.ok(await evaluate('[...document.links].some(a=>a.href === "http://localhost:8026/")'))
   model.recoveryStatus=429;await evaluate('document.querySelector(".recovery-resend").click()');await until('document.body.innerText.includes("Please wait")','resend cooldown')
   await send('Page.reload');await until('Boolean(document.querySelector(".recovery-resend"))','receipt reload')
   assert.equal(await evaluate('document.querySelector(".recovery-resend").disabled'),true)
  } else if(scenario==='reset') {
   const link=origin+'/reset-password#token='+'A'.repeat(43)
   await send('Page.navigate',{url:link});await until('Boolean(document.getElementById("reset-password"))','reset form')
   assert.equal(await evaluate('location.href.includes("token=")'),false)
   await fill('reset-password','1234567');await fill('reset-confirmation','1234567');await submit()
   assert.match(await text(),/at least 8/)
   await fill('reset-password','abcdefgh');await fill('reset-confirmation','different');await submit();assert.match(await text(),/match exactly/)
   assert.equal(model.calls.filter(c=>c.path.endsWith('/reset-password')).length,0)
   await fill('reset-confirmation','abcdefgh');model.resetStatus=400;await submit();await until('document.body.innerText.includes("invalid, expired or already used")','invalid/expired result')
   await fill('reset-password','abcdefgh');await fill('reset-confirmation','abcdefgh');model.resetStatus=204;await submit();await until('document.body.innerText.includes("Password updated")','reset success')
   assert.equal(model.session,false,'reset never logs in automatically')
   assert.equal(await evaluate('document.getElementById("reset-password").value.length'),0)
   await send('Page.reload');await until('document.body.innerText.includes("Open a valid reset link")','token discarded after reload')
   await send('Page.navigate',{url:link});await until('Boolean(document.getElementById("reset-password"))','reuse form')
   await fill('reset-password','abcdefgh');await fill('reset-confirmation','abcdefgh');await submit();await until('document.body.innerText.includes("invalid, expired or already used")','used link')
   await go('#/verify-code');assert.match(await text(),/Open a valid reset link/)
  } else if(scenario==='ui') {
   for(const hash of ['#/login','#/register','#/forgot-password','#/check-email','#/preview/home']) {
    await go(hash);await evaluate('document.fonts.ready')
    for(const [width,height]of [[1440,820],[768,900],[375,667],[320,568]]) {
     await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false});await delay(50)
     assert.ok(await evaluate('document.documentElement.scrollWidth<=innerWidth'),'no horizontal overflow')
     assert.ok(await evaluate('[...document.images].every(i=>i.complete&&i.naturalWidth>0)'),'assets loaded')
    }
   }
  } else throw Error('Unknown browser scenario')
  assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0,'credentials never persisted in browser storage')
  assert.equal(b.exceptions.length,0,'no runtime exceptions')
  assert.deepEqual(model.errors,[])
  console.log('PASS browser '+scenario+' (synthetic API fixtures; not live backend evidence)')
 }finally{b.close()}
}
