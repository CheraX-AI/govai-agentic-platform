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
  await send('Page.enable')
  await send('Network.enable')





  const production=process.argv[2]==='production',origin=production?'http://127.0.0.1:4173':'http://127.0.0.1:5173',storageKey='govai.dev.mock-session.v1'
  const go=async(hash)=>{await send('Page.navigate',{url:'about:blank'});await delay(100);await send('Page.navigate',{url:origin+'/'+hash});await delay(500);const ready='!!document.querySelector("h1") && !document.querySelector("vite-error-overlay") && (document.querySelector("h1").textContent!=="Checking session..." || !!window.restoreCallbacks?.length) && (!document.querySelector("#phone") || !document.querySelector("button[type=submit]").disabled)';for(let i=0;i<50&&!await evaluate(ready);i++)await delay(100);assert.ok(await evaluate(ready),'route ready '+hash)}
  const fill=async(id,value)=>evaluate('(()=>{const i=document.getElementById('+JSON.stringify(id)+');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(i,'+JSON.stringify(value)+');i.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const key=async(key,code)=>{await send('Input.dispatchKeyEvent',{type:'keyDown',key,code:key,windowsVirtualKeyCode:code,...(key==='Enter'?{text:'\r'}:{})});await send('Input.dispatchKeyEvent',{type:'keyUp',key,code:key,windowsVirtualKeyCode:code})}
  const seed=async()=>evaluate('(()=>{const now=new Date().getTime();sessionStorage.setItem('+JSON.stringify(storageKey)+',JSON.stringify({version:1,kind:"demo",userId:"synthetic-harshan",createdAt:now,expiresAt:now+900000}))})()')
  const login=async(method=0,value='0771234567')=>{if(method)await evaluate('document.querySelectorAll(".login-method button")[1].click()');await fill(method?'identifier':'phone',value);await fill('password','demo-only');await evaluate('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").focus()');await key('Enter',13);await delay(1100)}
  await go('#/login');assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back')
  for(const [hash,title] of [['#/register','Registration preview'],['#/forgot-password','Password recovery preview'],['#/check-email','Check email preview']]){await go(hash);assert.equal(await evaluate('document.title'),'GovAI | '+title)}
  await go('#/app/recent');assert.ok((await evaluate('location.hash')).startsWith('#/login?'));assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false)
  assert.match(await evaluate('location.hash'),/returnTo=%23%2Fapp%2Frecent/)
  if(production){
    await seed();await go('#/app/home');assert.ok((await evaluate('location.hash')).startsWith('#/login?'));assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false,'production never restores synthetic metadata')
    for(const [method,value] of [[0,'0771234567'],[1,'123456789v'],[1,'demo@example.com']]){await go('#/login');await login(method,value);assert.equal(await evaluate('location.hash'),'#/login');assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/not signed in/)}
    for(const hash of ['#/home','#/recent','#/preview/home','#/preview/recent']){await go(hash);assert.match(await evaluate('document.querySelector("h1").textContent'),/Development preview unavailable/);assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false)}
    await evaluate('sessionStorage.clear()')
  }else{
    // Delay restoration to prove the guard never mounts protected content early.
    const slow=await send('Page.addScriptToEvaluateOnNewDocument',{source:'const originalTimer=setTimeout;window.restoreCallbacks=[];window.restoreReleased=false;window.setTimeout=(f,ms,...args)=>ms===200?(window.restoreCallbacks.push(()=>f(...args)),0):originalTimer(f,ms,...args);window.earlyProtected=false;const obs=new MutationObserver(()=>{if(document.querySelector(".application-layout")&&!window.restoreReleased)window.earlyProtected=true});obs.observe(document,{childList:true,subtree:true})'})
    await seed();await go('#/app/recent');assert.equal(await evaluate('document.querySelector("h1").textContent'),'Checking session...');assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false);assert.ok(await evaluate('window.restoreCallbacks.length>0'));await evaluate('window.restoreReleased=true;window.restoreCallbacks.forEach(f=>f())');await delay(300);assert.ok(await evaluate('!!document.querySelector(".recent-content")'));assert.equal(await evaluate('window.earlyProtected'),false);await send('Page.removeScriptToEvaluateOnNewDocument',{identifier:slow.identifier})
    await go('#/app/recent');assert.ok(await evaluate('!!document.querySelector(".recent-content")'),'refresh restores valid synthetic session');assert.equal(await evaluate('document.title'),'GovAI | Recent chats - mock application');assert.match(await evaluate('document.querySelector(".demo-session-notice").textContent'),/Mock session.*no real authentication/)
    assert.deepEqual(await evaluate('Object.keys(JSON.parse(sessionStorage.getItem('+JSON.stringify(storageKey)+'))).sort()'),['createdAt','expiresAt','kind','userId','version'])
    await evaluate('location.hash="#/app/home"');await delay(150)
    await evaluate('document.querySelector(".synthetic-user").focus()');await key('Enter',13);await delay(80);assert.equal(await evaluate('document.activeElement.textContent'),'Demo logout');await key('Escape',27);assert.equal(await evaluate('document.activeElement.className'),'synthetic-user')
    await evaluate('document.querySelector(".synthetic-user").click()');await delay(80);await key('Enter',13);await delay(200);assert.equal(await evaluate('location.hash'),'#/login');assert.equal(await evaluate('sessionStorage.length'),0)
    await evaluate('window.loggedOutFlash=false;new MutationObserver(()=>{if(document.querySelector(".application-layout"))window.loggedOutFlash=true}).observe(document.body,{childList:true,subtree:true});history.back()');await delay(500);assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false);assert.equal(await evaluate('window.loggedOutFlash'),false)
    // A confirmed anonymous direct URL is replaced with allowlisted return navigation.
    await go('#/app/recent');const historyBefore=await evaluate('history.length');await login();assert.equal(await evaluate('location.hash'),'#/app/recent');assert.equal(await evaluate('history.length'),historyBefore,'login and anonymous redirect use replacement')
    await evaluate('document.querySelector(".demo-session-notice button").click()');await delay(200)
    for(const [method,value] of [[0,'0771234567'],[1,'123456789v'],[1,'demo@example.com']]){
      await go('#/login?returnTo=https%3A%2F%2Fevil.example');await login(method,value);assert.equal(await evaluate('location.hash'),'#/app/home','external return destinations cannot escape allowlist');await evaluate('document.querySelector(".demo-session-notice button").click()');await delay(160)
    }
    for(const raw of ['bad',JSON.stringify({version:1,kind:'demo',userId:'synthetic-harshan',createdAt:0,expiresAt:1}),JSON.stringify({version:1,kind:'demo',userId:'unknown',createdAt:Date.now(),expiresAt:Date.now()+60000})]){await evaluate('sessionStorage.setItem('+JSON.stringify(storageKey)+','+JSON.stringify(raw)+')');await go('#/app/home');assert.ok((await evaluate('location.hash')).startsWith('#/login?'));assert.equal(await evaluate('sessionStorage.length'),0)}
    // Checking failures are not treated as confirmed anonymous sessions.
    const failure=await send('Page.addScriptToEvaluateOnNewDocument',{source:'window.failRead=true;const nativeGet=Storage.prototype.getItem;Storage.prototype.getItem=function(key){if(window.failRead&&key==="govai.dev.mock-session.v1")throw Error("fixture storage failure");return nativeGet.call(this,key)}'})
    await go('#/app/home');assert.equal(await evaluate('location.hash'),'#/app/home');assert.equal(await evaluate('document.querySelector("h1").textContent'),'Session check failed');assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false);assert.equal(await evaluate('document.activeElement.tagName'),'H1')
    await evaluate('window.failRead=false;document.querySelector("button").focus()');await key('Enter',13);await delay(400);assert.ok((await evaluate('location.hash')).startsWith('#/login?'));await send('Page.removeScriptToEvaluateOnNewDocument',{identifier:failure.identifier})
    // Suspend periodic expiry checks: resume events must independently invalidate.
    const pause=await send('Page.addScriptToEvaluateOnNewDocument',{source:'const nativeInterval=setInterval;window.setInterval=(f,ms,...args)=>ms===250?0:nativeInterval(f,ms,...args)'})
    for(const event of ['focus','pageshow','visibilitychange']){
      await seed();await go('#/app/home');assert.ok(await evaluate('!!document.querySelector(".application-layout")'),JSON.stringify({event,...await evaluate('({hash:location.hash,heading:document.querySelector("h1")?.textContent,record:sessionStorage.getItem("govai.dev.mock-session.v1")})')}))
      await evaluate('const nativeNow=Date.now;Date.now=()=>nativeNow()+901000;'+(event==='visibilitychange'?'document':'window')+'.dispatchEvent(new Event('+JSON.stringify(event)+'))');await delay(160)
      assert.ok((await evaluate('location.hash')).startsWith('#/login?'));assert.match(await evaluate('location.hash'),/reason=expired/);assert.equal(await evaluate('!!document.querySelector(".application-layout")'),false);assert.equal(await evaluate('sessionStorage.length'),0)
    }
    await send('Page.removeScriptToEvaluateOnNewDocument',{identifier:pause.identifier})
    await seed();await go('#/app/home');await evaluate('const nativeNow=Date.now;Date.now=()=>nativeNow()+901000');await delay(400);assert.ok((await evaluate('location.hash')).startsWith('#/login?'),'active interval expires session')
    for(const [alias,canonical] of [['#/home','#/preview/home'],['#/recent','#/preview/recent']]){await go(alias);assert.equal(await evaluate('location.hash'),canonical);assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false);assert.equal(await evaluate('sessionStorage.length'),0)}
    await seed();await go('#/preview/home');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false,'preview stays static even with a valid mock session');await evaluate('sessionStorage.clear()')
  }
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[]);assert.equal(await evaluate('localStorage.length'),0);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  console.log('PASS '+(production?'production':'development')+': public/protected/preview separation, checking no-flash, schema/expiry/restore, errors/retry, allowlisted replacement redirects, mock login/logout/history, active/resume expiry, keyboard account access, production no mock creation/restoration, no API/localStorage/cookies/errors')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
