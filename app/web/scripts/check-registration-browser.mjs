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

  await send('Page.navigate', {url:'http://127.0.0.1:5173/#/register'})
  await delay(800)
  assert.equal(await evaluate('document.title'),'GovAI | Registration preview')
  assert.equal(await evaluate('document.querySelector("#register-terms").checked'),false)
  assert.deepEqual(await evaluate('[...document.querySelectorAll("a")].map(a=>a.hash)'),['#/login'],'no fabricated legal or other destinations')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'),'register-fullName')
  assert.equal(await evaluate('document.querySelectorAll(".field-error").length'),7)
  assert.equal(await evaluate('document.querySelector("#register-password").getAttribute("aria-describedby")'),'registration-password-policy register-password-error')
  const fill = async (id,value) => evaluate(`(() => {const i=document.getElementById(${JSON.stringify(id)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,${JSON.stringify(value)});i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
  const password='abcdef1'+String.fromCodePoint(0x1f600)
  const validFields={'register-fullName':'Demo '+String.fromCodePoint(0x0dc4,0x0dbb,0x0dca,0x0dc2),'register-nic':'123456789v','register-phone':'+94 77 123 4567','register-email':' demo@example.com ','register-password':password,'register-confirmation':password}
  for(const [id,value] of Object.entries(validFields)) await fill(id,value)
  await evaluate('document.querySelector("#register-terms").click()')
  await fill('register-password','aaaaa1'+String.fromCodePoint(0x1f600))
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.ok(await evaluate('document.querySelector("#register-password-error") !== null'),'seven Unicode code points rejected')
  await fill('register-password',password)
  await fill('register-confirmation','different')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'),'register-confirmation')
  await fill('register-confirmation',password)
  const toggles=await evaluate('[...document.querySelectorAll(".icon-button")].map(b=>b.getAttribute("aria-label"))')
  assert.deepEqual(toggles,['Show password','Show confirm password'])
  await evaluate('document.querySelectorAll(".icon-button")[1].click()')
  assert.equal(await evaluate('document.querySelector("#register-password").type'),'password')
  assert.equal(await evaluate('document.querySelector("#register-confirmation").type'),'text')
  await evaluate('document.querySelectorAll(".icon-button")[0].click();window.mockTimerCount=0;const oldTimer=window.setTimeout.bind(window);window.setTimeout=(f,ms,...args)=>{if(ms===800)window.mockTimerCount++;return oldTimer(f,ms,...args)}')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.ok(await evaluate('[...document.querySelectorAll("form input, form button")].every(i=>i.disabled)'),'pending locks form')
  await evaluate('document.querySelector("form").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))')
  assert.equal(await evaluate('window.mockTimerCount'),1,'duplicate submit guarded')
  await delay(1000)
  assert.equal(await evaluate('document.querySelector("[role=status]").textContent'),'Demo registration successful. No account was created.')
  assert.equal(await evaluate('location.hash'),'#/register','success stays on page')
  assert.deepEqual(await evaluate('["#register-password","#register-confirmation"].map(s=>({value:document.querySelector(s).value,type:document.querySelector(s).type}))'),[{value:'',type:'password'},{value:'',type:'password'}])
  await evaluate('document.querySelector("#register-outcome").value="error";document.querySelector("#register-outcome").dispatchEvent(new Event("change",{bubbles:true}))')
  await fill('register-password',' abcdef1! ');await fill('register-confirmation',' abcdef1! ')
  assert.equal(await evaluate('document.querySelector("#register-password").value'),' abcdef1! ')
  await evaluate('document.querySelector("button[type=submit]").click()');await delay(1000)
  assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/No account was created/)
  assert.equal(await evaluate('document.querySelector("#register-confirmation").value'),'')
  const measurements=[]
  for(const [width,height] of [[1440,820],[1366,768],[1024,768],[900,900],[768,900],[580,900],[375,667],[320,568]]){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false})
    await send('Page.reload');await delay(300)
    for(const state of ['initial','errors']){
      if(state==='errors')await evaluate('document.querySelector("button[type=submit]").click()')
      const metrics=await evaluate(`(() => {const c=document.querySelector('.login-card').getBoundingClientRect();return {overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth,card:c.width,columns:getComputedStyle(document.querySelector('.registration-row')).gridTemplateColumns.split(' ').length,images:[...document.images].every(i=>i.complete&&i.naturalWidth>0)}})()`)
      assert.equal(metrics.overflow,0,'registration overflow '+width+' '+state)
      assert.equal(metrics.columns,width>580?2:1,'paired fields stack on narrow website')
      assert.ok(metrics.images)
      if(width===1440)assert.equal(metrics.card,520)
      await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})')
      assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'))
      await evaluate('scrollTo(0,0)')
      if(state==='initial'&&(width===1440||width===375)){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/registration-'+width+'.png',Buffer.from(shot.data,'base64'))}
      measurements.push({width,height,state,...metrics})
    }
  }
  await evaluate(`[...document.querySelectorAll('a')].find(a=>a.hash==='#/login').click()`);await delay(100)
  assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back')
  await evaluate(`[...document.querySelectorAll('a')].find(a=>a.hash==='#/register').click()`);await delay(100)
  assert.equal(await evaluate('document.querySelector("#register-fullName").value'),'')
  await evaluate('history.back()');await delay(100);assert.equal(await evaluate('location.hash'),'#/login')
  await evaluate('history.forward()');await delay(100);assert.equal(await evaluate('location.hash'),'#/register')
  for(const [id,value] of Object.entries(validFields))await fill(id,value)
  await evaluate('document.querySelector("#register-terms").click();document.querySelector("button[type=submit]").click();location.hash="#/login"');await delay(1000)
  assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back','late mock result does not change destination')
  await evaluate('location.hash="#/register"');await delay(100)
  assert.equal(await evaluate('document.querySelectorAll(".status-message").length'),0)
  assert.deepEqual(apiRequests,[],'no delivery/authentication/registration requests')
  assert.deepEqual(exceptions,[])
  assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0)
  assert.deepEqual((await send('Network.getCookies')).cookies,[])
  await writeFile('node_modules/.tmp/registration-layout-measurements.json',JSON.stringify(measurements,null,2))
  console.log('PASS: registration required errors/focus/associations, Unicode policy boundaries, independent masking, exact passwords, terms unchecked/no legal links, pending/duplicate guard, mock success/failure/clearing, 16 responsive cases, login/back-forward/direct access/unmount safety, loaded assets and no API/storage/cookies/runtime errors')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
