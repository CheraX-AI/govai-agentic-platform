import { webRoot } from './browserHarness.mjs'
if (process.env.GOVAI_LIVE_AUTH_TESTS !== '1') { console.log('SKIP live auth: set GOVAI_LIVE_AUTH_TESTS=1; requires running local services and creates one synthetic account.'); process.exit(0) }
process.chdir(webRoot)
import { spawn } from 'node:child_process'
import { mkdir, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/registration-browser-profile-${process.pid}-${Date.now()}`)
await mkdir(profile, { recursive: true })
const browser = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', ['--headless=new', '--disable-extensions', '--disable-component-extensions-with-background-pages', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore', windowsHide: true })
let launchError
browser.on('error', error => { launchError = error })
let socket
const pending = new Map()
try {
  let target
  for (let i = 0; i < 40; i++) {
    if (launchError) throw launchError
    assert.equal(browser.exitCode, null, 'Edge remains running during startup')
    try {
      const port = (await readFile(resolve(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0].trim()
      target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT', signal: AbortSignal.timeout(1000) })).json()
      break
    } catch { await delay(250) }
  }
  assert.ok(target?.webSocketDebuggerUrl, 'Headless Edge debugger available')
  socket = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((r, reject) => { socket.onopen = r; socket.onerror = reject })
  let id = 0
  const exceptions = []
  const apiRequests = []
  socket.onmessage = ({ data }) => {
    const message = JSON.parse(data)
    if (message.method === 'Runtime.exceptionThrown') exceptions.push(message.params)
    if (message.method === 'Network.requestWillBeSent' && ['Fetch', 'XHR'].includes(message.params.type)) apiRequests.push(message.params.request.url)
    if (message.id) {
      const call = pending.get(message.id)
      if (!call) return
      clearTimeout(call.timer)
      pending.delete(message.id)
      if (message.error) call.reject(message.error); else call.resolve(message.result)
    }
  }
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const callId = ++id
    const timer = setTimeout(() => { pending.delete(callId); reject(Error(`Debugger command timed out: ${method}`)) }, 10000)
    pending.set(callId, { resolve, reject, timer })
    socket.send(JSON.stringify({ id: callId, method, params }))
  })
  const evaluate = async (expression) => {
    const result = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails))
    return result.result.value
  }
  await send('Runtime.enable')
  await send('Network.enable')



  await send('Page.navigate', {url:'https://localhost:5173/#/register'})
  await delay(700)
  assert.ok(await evaluate('Boolean(document.querySelector("#register-fullName"))'))
  assert.equal(await evaluate('Boolean(document.querySelector("#register-outcome"))'), false)
  await evaluate(`window.registrationCalls=[];const originalFetch=window.fetch;window.fetch=async(...args)=>{window.registrationCalls.push({url:String(args[0]),keys:Object.keys(JSON.parse(args[1].body)),credentials:args[1].credentials,key:args[1].headers['Idempotency-Key']});return originalFetch(...args)}`)
  const crypto = await import('node:crypto')
  const email='ga84-ui-'+crypto.randomBytes(6).toString('hex')+'@govai.test'
  const values={fullName:'Synthetic UI Test',nic:String(crypto.randomInt(100000000,999999999))+'V',phone:'+94771234567',email,password:'Test!'+crypto.randomBytes(12).toString('hex')}
  values.confirmation=values.password
  for(const [field,value] of Object.entries(values)) await evaluate(`(()=>{const input=document.getElementById(${JSON.stringify('register-'+field)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,${JSON.stringify(value)});input.dispatchEvent(new Event('input',{bubbles:true}))})()`)
  await evaluate('document.querySelector("#register-terms").click();document.querySelector("button[type=submit]").click()')
  for(let i=0;i<40;i++){if(await evaluate('document.querySelector(".status-message")?.textContent?.includes("Request received") || document.body.innerText.includes("Request received")'))break;await delay(250)}
  assert.match(await evaluate('document.body.innerText'),/Request received/)
  assert.equal(await evaluate('document.querySelector("#register-password").value'),'')
  const calls=await evaluate('window.registrationCalls')
  assert.equal(calls.length,1)
  assert.deepEqual(calls[0].keys.sort(),['full_name','nic','mobile_number','email','password'].sort())
  assert.equal(calls[0].credentials,'omit')
  assert.match(calls[0].key,/^[a-f0-9-]{36}$/)
  const inbox=await(await fetch('http://localhost:8025/api/v1/messages')).json()
  assert.ok(inbox.messages.some(m=>m.To.some(t=>t.Address===email)))

  const messageSummary=inbox.messages.find(m=>m.To.some(t=>t.Address===email))
  const mail=await(await fetch('http://localhost:8025/api/v1/message/'+messageSummary.ID)).json()
  const link=(mail.HTML||mail.Text).replaceAll('&amp;','&').match(/https:\/\/localhost:8443\/[^\s"<>]+/)?.[0]
  assert.ok(link)
  await send('Page.navigate',{url:link})
  await delay(700)
  await evaluate(`(()=>{const a=[...document.querySelectorAll('a')].find(a=>a.textContent.includes('proceed'));if(a)a.click()})()`)
  await delay(700)
  await evaluate(`(()=>{const a=[...document.querySelectorAll('a')].find(a=>/back to application/i.test(a.textContent));if(a)a.click()})()`)
  await delay(800)
  assert.equal(await evaluate('location.origin'),'https://localhost:5173')
  assert.ok(await evaluate('Boolean(document.querySelector("#login-heading")) || document.body.innerText.includes("Welcome") || Boolean(document.querySelector("input[autocomplete=username]"))'))

  await send('Page.navigate',{url:'https://localhost:5173/#/forgot-password'});await delay(650)
  assert.ok(await evaluate('document.getElementById("recovery-phone").disabled'))
  await evaluate('document.getElementById("recovery-email").click()')
  await evaluate(`(()=>{const input=document.getElementById('recovery-nic');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,${JSON.stringify(values.nic)});input.dispatchEvent(new Event('input',{bubbles:true}));})()`)
  await evaluate('document.getElementById("recovery-human").click();window.realFetch=window.fetch;window.fetch=async(...args)=>String(args[0]).includes("forgot-password")?new Response("private error",{status:503}):window.realFetch(...args);document.querySelector("button[type=submit]").click()')
  await delay(250)
  assert.equal(await evaluate('location.hash'),'#/forgot-password')
  assert.match(await evaluate('document.body.innerText'),/temporarily unavailable/)
  assert.doesNotMatch(await evaluate('document.body.innerText'),/private error/)
  await evaluate('window.fetch=window.realFetch;document.querySelector("button[type=submit]").click()')
  for(let i=0;i<120;i++){if(await evaluate('location.hash==="#/check-email"'))break;await delay(250)}
  assert.equal(await evaluate('location.hash'),'#/check-email')
  assert.match(await evaluate('document.body.innerText'),/Registered email:/)
  const resetInbox=await(await fetch('http://localhost:8026/api/v1/messages')).json()
  assert.ok(resetInbox.messages.some(m=>m.To.some(t=>t.Address===email)))

  const resetSummary=resetInbox.messages.find(m=>m.To.some(t=>t.Address===email))
  const resetMail=await(await fetch('http://localhost:8026/api/v1/message/'+resetSummary.ID)).json()
  const raw=resetMail.Text||resetMail.HTML
  const resetLink=raw.replaceAll('&amp;','&').match(/https:\/\/localhost:5173\/reset-password#token=[A-Za-z0-9_-]{43}/)?.[0]
  assert.ok(resetLink,'New reset email targets the frontend')
  await send('Page.navigate',{url:resetLink});await delay(700)
  assert.equal(await evaluate('location.hash'),'#/reset-password')
  assert.ok(await evaluate('Boolean(document.getElementById("reset-password"))'))
  assert.equal(await evaluate('location.href.includes("token=")'),false)
  const fillReset=async(password,confirmation)=>{
    for(const [id,value] of [['reset-password',password],['reset-confirmation',confirmation]]) await evaluate(`(()=>{const i=document.getElementById(${JSON.stringify(id)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,${JSON.stringify(value)});i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
    await evaluate('document.querySelector("button[type=submit]").click()');await delay(350)
  }
  await fillReset('1234567','1234567');assert.match(await evaluate('document.body.innerText'),/at least 8/)
  const newPassword='Changed!'+crypto.randomBytes(12).toString('hex')
  await fillReset(newPassword,'different');assert.match(await evaluate('document.body.innerText'),/match exactly/)
  await fillReset(newPassword,newPassword)
  for(let i=0;i<40;i++){if(await evaluate('document.body.innerText.includes("Password updated.")'))break;await delay(250)}
  assert.match(await evaluate('document.body.innerText'),/Password updated/)
  assert.equal(await evaluate('document.getElementById("reset-password").value.length'),0)
  assert.equal(await evaluate('location.hash'),'#/reset-password')
  assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0)
  await send('Page.navigate',{url:resetLink});await delay(700)
  await fillReset(newPassword,newPassword)
  assert.match(await evaluate('document.body.innerText'),/invalid, expired or already used/)
  await send('Page.navigate',{url:'https://localhost:5173/#/login'});await delay(650)
  const fillLogin=async(password)=>{
    for(const [id,value] of [['identifier',email],['password',password]])await evaluate(`(()=>{const i=document.getElementById(${JSON.stringify(id)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,${JSON.stringify(value)});i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
    await evaluate('document.querySelector("button[type=submit]").click()');await delay(650)
  }
  await fillLogin(values.password);assert.match(await evaluate('document.body.innerText'),/Invalid email\/NIC or password/)
  await fillLogin(newPassword);assert.equal(await evaluate('location.hash'),'#/app/home')
  await evaluate('[...document.querySelectorAll("button")].find(b=>b.textContent==="Log out").click()');await delay(500)
  await send('Page.navigate',{url:'https://localhost:5173/reset-password#token=invalid'});await delay(500)
  assert.equal(await evaluate('location.hash'),'#/reset-password')
  assert.match(await evaluate('document.body.innerText'),/Open a valid reset link/)
  console.log('PASS: frontend reset email, token stripped, length/confirmation checks, live password reset, no automatic login, used-link rejection, old password denied/new password accepted, malformed link and no storage')
} finally { socket?.close(); browser.kill() }
