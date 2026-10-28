import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
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




  await send('DOM.enable');await send('CSS.enable')
  const go=async(hash)=>{await send('Page.navigate',{url:'http://127.0.0.1:5173/'+hash});await delay(150);await send('Page.reload');await delay(400);await evaluate('document.fonts.ready')}
  const key=async(key,shift=false)=>{const code=key==='Tab'?9:key==='Escape'?27:key==='ArrowDown'?40:13;await send('Input.dispatchKeyEvent',{type:'keyDown',key,code:key,windowsVirtualKeyCode:code,modifiers:shift?8:0,...(key==='Enter'?{text:'\r'}:{})});await send('Input.dispatchKeyEvent',{type:'keyUp',key,code:key,windowsVirtualKeyCode:code,modifiers:shift?8:0})}
  const fill=async(id,value)=>evaluate('(()=>{const i=document.getElementById('+JSON.stringify(id)+');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(i,'+JSON.stringify(value)+');i.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const fontEvidence=[]
  for(const [hash,selectors] of [['#/login',['h1','.input-group label','.primary-button','.demo-banner']],['#/register',['h1','.registration-policy','.primary-button']],['#/forgot-password',['h1','.recovery-warning','.input-group label']],['#/check-email',['h1','.recovery-tip','.recovery-resend']],['#/home',['.home-hero h1','.home-hero > p:first-child','.synthetic-user strong','.navigation-item']],['#/recent',['h1','.history-unavailable h2','.navigation-item']]]){
    await send('Emulation.setDeviceMetricsOverride',{width:1440,height:820,deviceScaleFactor:1,mobile:false});await go(hash)
    const {root}=await send('DOM.getDocument')
    for(const selector of [...selectors,'.wordmark']){
      const {nodeId}=await send('DOM.querySelector',{nodeId:root.nodeId,selector});assert.ok(nodeId,selector)
      const {fonts}=await send('CSS.getPlatformFontsForNode',{nodeId});const expected=selector==='.wordmark'?'Montserrat':'Inter'
      assert.ok(fonts.some(f=>f.isCustomFont&&f.familyName.includes(expected)&&f.glyphCount>0),hash+' actual '+expected+' '+selector);fontEvidence.push({hash,selector,fonts})
    }
    if(!['#/home','#/recent'].includes(hash)){assert.equal(await evaluate('document.querySelector(".language").textContent'),'English');assert.equal(await evaluate('document.querySelector(".language").tagName'),'SPAN');assert.equal(await evaluate('document.querySelector(".language").tabIndex'),-1)}
    assert.equal(await evaluate('/[\u2013\u2014]/.test([...document.querySelectorAll(".demo-banner,.recovery-warning,.helper,.demo-session-notice")].map(e=>e.textContent).join(""))'),false)
    for(const width of [1440,1024,768,375,320]){
      await send('Emulation.setDeviceMetricsOverride',{width,height:820,deviceScaleFactor:1,mobile:false});await delay(80)
      assert.equal(await evaluate('document.documentElement.scrollWidth-document.documentElement.clientWidth'),0,hash+' overflow '+width)
      if(hash==='#/home'||hash==='#/recent'){
        await evaluate('document.querySelector(".synthetic-user").click()');await delay(50)
        assert.ok(await evaluate('(()=>{const r=document.querySelector(".account-actions").getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})()'),'dropdown in viewport '+width)
        assert.equal(await evaluate('document.querySelector(".account-actions button").disabled'),true)
        await key('Escape');assert.equal(await evaluate('document.activeElement.className'),'synthetic-user')
      }
      if([1440,375].includes(width)){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/auth-ui-'+hash.slice(2)+'-'+width+'.png',Buffer.from(shot.data,'base64'))}
    }
  }
  await send('Emulation.setDeviceMetricsOverride',{width:1440,height:820,deviceScaleFactor:1,mobile:false});await go('#/register')
  for(const [id,value] of Object.entries({'register-fullName':'Demo Person','register-nic':'123456789v','register-phone':'0771234567','register-email':'demo@example.com','register-password':'abcdef1!','register-confirmation':'abcdef1!'}))await fill(id,value)
  await evaluate('document.querySelector("#register-terms").click();document.querySelector("button[type=submit]").click()');await delay(950)
  assert.equal(await evaluate('location.hash'),'#/register');assert.equal(await evaluate('document.querySelector("[role=status]").textContent'),'Demo registration successful. No account was created.')
  assert.ok(await evaluate('document.querySelector(".registration-login-action").getBoundingClientRect().height>=44'))
  assert.equal(await evaluate('getComputedStyle(document.querySelector(".registration-login-action")).color'),'rgb(255, 255, 255)','primary success link retains white contrast on brand background')
  assert.equal(await evaluate('document.querySelector("#register-password").value'),'');assert.equal(await evaluate('document.querySelector("#register-confirmation").value'),'')
  await evaluate('document.querySelector(".registration-login-action").focus()');await key('Enter');await delay(120);assert.equal(await evaluate('location.hash'),'#/login')
  await fill('phone','0771234567');await fill('password','demo-only');await evaluate('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").click()');await delay(950)
  assert.equal(await evaluate('location.hash'),'#/home')
  await evaluate('document.querySelector(".synthetic-user").focus()');await key('Enter');await delay(80)
  assert.equal(await evaluate('document.activeElement.textContent'),'Demo logout');assert.equal(await evaluate('document.querySelector(".synthetic-user").getAttribute("aria-expanded")'),'true')
  await key('Escape');assert.equal(await evaluate('document.activeElement.className'),'synthetic-user');assert.equal(await evaluate('!!document.querySelector(".account-actions")'),false)
  await key('ArrowDown');await delay(80);assert.equal(await evaluate('document.activeElement.textContent'),'Demo logout')
  await key('Tab',true);assert.equal(await evaluate('document.activeElement.className'),'synthetic-user');await key('Tab');assert.equal(await evaluate('document.activeElement.textContent'),'Demo logout');await key('Tab');assert.equal(await evaluate('!!document.querySelector(".account-actions")'),false,'Tab exits disclosure without trap')
  await evaluate('document.querySelector(".synthetic-user").click()');await delay(50);await key('Enter');await delay(150)
  assert.equal(await evaluate('location.hash'),'#/login');await evaluate('history.back()');await delay(120);assert.equal(await evaluate('location.hash'),'#/home');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false)
  await evaluate('document.querySelector(".synthetic-user").click()');await delay(80);assert.equal(await evaluate('document.querySelector(".account-actions button").disabled'),true)
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[]);assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  await writeFile('node_modules/.tmp/auth-ui-fonts.json',JSON.stringify(fontEvidence,null,2))
  console.log('PASS: actual Inter glyphs across six pages and Montserrat wordmarks, English noninteractive, notice punctuation, 30 responsive cases/screenshots, registration success/clear/keyboard login, account focus/Enter/ArrowDown/Tab/Escape/return/logout, no history session resurrection or API/storage/cookies/errors')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
