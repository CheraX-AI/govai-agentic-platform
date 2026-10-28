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




  const production=process.argv[2]==='production',origin=production?'http://127.0.0.1:4173':'http://127.0.0.1:5173'
  const go=async(hash)=>{await send('Page.navigate',{url:origin+'/'+hash});await delay(150);await send('Page.reload');await delay(450);for(let attempt=0;attempt<40;attempt++){if(await evaluate('!!document.querySelector("h1") && !document.querySelector("vite-error-overlay")'))break;await delay(100)}}
  const fill=async(id,value)=>evaluate('(()=>{const i=document.getElementById('+JSON.stringify(id)+');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(i,'+JSON.stringify(value)+');i.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const key=async()=>{await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r'});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13})}
  for(const [method,id,value] of [[0,'phone','0771234567'],[1,'identifier','123456789v'],[1,'identifier','demo@example.com']]){
    await go('#/login')
    if(method)await evaluate('document.querySelectorAll(".login-method button")[1].click()')
    await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('location.hash'),'#/login');assert.equal(await evaluate('document.activeElement.id'),id)
    await fill(id,'invalid');await fill('password','demo-only');await evaluate('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('location.hash'),'#/login')
    await fill(id,value);await evaluate('var s=document.querySelector("#outcome");s.value="error";s.dispatchEvent(new Event("change",{bubbles:true}));document.querySelector("button[type=submit]").click()');await delay(950)
    assert.equal(await evaluate('location.hash'),'#/login');assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/failed/)
    await fill('password','demo-only');await evaluate('var s=document.querySelector("#outcome");s.value="success";s.dispatchEvent(new Event("change",{bubbles:true}));document.querySelector("button[type=submit]").focus()');await key();await delay(1000)
    if(production){assert.equal(await evaluate('location.hash'),'#/login');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false);assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/not signed in/);continue}
    assert.equal(await evaluate('location.hash'),'#/home');assert.equal(await evaluate('document.title'),'GovAI | Home layout preview')
    assert.match(await evaluate('document.querySelector(".demo-session-notice").textContent'),/Demo session.*not authenticated/)
    assert.equal(await evaluate('document.activeElement.id'),'application-content')
    assert.equal(await evaluate('!!document.querySelector("input[type=password]")'),false)
    await evaluate('history.back()');await delay(150);assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back')
    await evaluate('history.forward()');await delay(150);assert.equal(await evaluate('location.hash'),'#/home');assert.ok(await evaluate('!!document.querySelector(".demo-session-notice")'))
    await evaluate('document.querySelector(".demo-session-notice button").focus()');await key();await delay(150)
    assert.equal(await evaluate('location.hash'),'#/login');assert.equal(await evaluate('document.querySelector("#password").value'),'')
    await evaluate('history.back()');await delay(150);assert.equal(await evaluate('location.hash'),'#/home');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false,'logout never resurrects session')
    await evaluate('history.forward()');await delay(150);assert.equal(await evaluate('location.hash'),'#/login')
  }
  if(!production){
    await go('#/login');await fill('phone','0771234567');await fill('password','demo-only');await evaluate('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").click()');await delay(950);await send('Page.reload');await delay(450);assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false,'refresh discards in-memory session')
    await go('#/login');await fill('phone','0771234567');await fill('password','demo-only');await evaluate('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").click();location.hash="#/register"');await delay(950);assert.equal(await evaluate('location.hash'),'#/register','late login cannot hijack another page')
  }else{
    for(const hash of ['#/home','#/recent']){await go(hash);assert.match(await evaluate('document.querySelector("h1").textContent'),/Development preview unavailable/);assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false)}
  }
  for(const [hash,title] of [['#/register','Registration preview'],['#/forgot-password','Password recovery preview'],['#/check-email','Check email preview']]){await go(hash);assert.equal(await evaluate('document.title'),'GovAI | '+title);await send('Page.reload');await delay(350);assert.equal(await evaluate('document.title'),'GovAI | '+title)}
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[]);assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  console.log('PASS '+(production?'production':'development')+': phone/NIC/email invalid/error/success, keyboard submit/logout, demo isolation, history/logout/refresh safety, late completion, registration/recovery URLs, no API/storage/cookies')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
