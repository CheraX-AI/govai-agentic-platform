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


  const go=async(hash)=>{await send('Page.navigate',{url:'http://127.0.0.1:5173/'+hash});await delay(500)}
  const fill=async(value)=>evaluate('(()=>{const i=document.querySelector("#recovery-nic");Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(i,'+JSON.stringify(value)+');i.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const select=async(id,value)=>evaluate('(()=>{const s=document.getElementById('+JSON.stringify(id)+');s.value='+JSON.stringify(value)+';s.dispatchEvent(new Event("change",{bubbles:true}))})()')
  await go('#/login');await evaluate('[...document.querySelectorAll("a")].find(a=>a.hash==="#/forgot-password").click()');await delay(150)
  assert.equal(await evaluate('document.title'),'GovAI | Password recovery preview')
  assert.equal(await evaluate('document.querySelectorAll(".recovery-methods input:checked").length'),0)
  await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('document.activeElement.id'),'recovery-email');assert.match(await evaluate('document.querySelector("#recovery-method-error").textContent'),/Choose Email or Phone/)
  await evaluate('document.querySelector("#recovery-email").click();document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'),'recovery-nic')
  assert.equal(await evaluate('document.querySelectorAll(".field-error").length'),2)
  await fill('123456789z');await evaluate('document.querySelector("button[type=submit]").click()')
  assert.match(await evaluate('document.querySelector("#recovery-nic-error").textContent'),/V\/X/)
  await fill(' 123456789v ');await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'),'recovery-human')
  assert.match(await evaluate('document.activeElement.getAttribute("aria-describedby")'),/recovery-human-error/)
  await evaluate('document.querySelector("#recovery-human").click()');await select('recovery-outcome','error')
  await evaluate('window.mockTimerCount=0;const oldTimer=window.setTimeout.bind(window);window.setTimeout=(f,ms,...args)=>{if(ms===800)window.mockTimerCount++;return oldTimer(f,ms,...args)};document.querySelector("button[type=submit]").click();document.querySelector("form").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))')
  assert.equal(await evaluate('window.mockTimerCount'),1)
  assert.ok(await evaluate('[...document.querySelectorAll("form input,form button,#recovery-outcome")].every(e=>e.matches(":disabled"))'))
  assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/Simulating/)
  assert.equal(await evaluate('document.querySelector("#recovery-nic").value'),'123456789v')
  await delay(950);assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/Demo request failed.*No email or SMS was sent/)
  await select('recovery-outcome','success');await evaluate('document.querySelector("button[type=submit]").click()');await delay(1000)
  assert.equal(await evaluate('location.hash'),'#/check-email')
  assert.equal(await evaluate('document.activeElement.id'),'check-email-heading')
  assert.equal(await evaluate('document.querySelector("strong").textContent'),'d'+String.fromCodePoint(8226).repeat(3)+'@example.com')
  assert.match(await evaluate('document.querySelector(".recovery-tip").textContent'),/30-minute.*design preview only.*no active reset link/)
  assert.match(await evaluate('document.querySelector(".recovery-warning").textContent'),/no email was sent/)
  await evaluate('window.resendTimerCount=0;const savedTimer=window.setTimeout.bind(window);window.setTimeout=(f,ms,...args)=>{if(ms===800)window.resendTimerCount++;return savedTimer(f,ms,...args)};document.querySelector(".recovery-resend").focus()');await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r'});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13})
  assert.ok(await evaluate('document.querySelector(".recovery-resend").disabled'))
  await evaluate('document.querySelector(".recovery-resend").click()');await delay(950)
  assert.equal(await evaluate('window.resendTimerCount'),1,'duplicate pending resend blocked')
  assert.match(await evaluate('[...document.querySelectorAll("[role=status]")].at(-1).textContent'),/Demo resend successful.*No email was sent/)
  await select('resend-outcome','error');await evaluate('document.querySelector(".recovery-resend").click()');await delay(950)
  assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/Demo resend failed.*No email was sent/)
  await evaluate('history.back()');await delay(200);assert.ok(await evaluate('!!document.querySelector("#recovery-nic")'))
  await evaluate('history.forward()');await delay(200);assert.ok(await evaluate('!!document.querySelector("#check-email-heading")'))
  await send('Page.reload');await delay(400);assert.equal(await evaluate('document.querySelector("strong").textContent'),'d'+String.fromCodePoint(8226).repeat(3)+'@example.com')
  await evaluate('document.querySelector(".recovery-back").click()');await delay(200);assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back')
  await go('#/forgot-password');await fill('123456789012');await evaluate('document.querySelector("#recovery-email").click()');await evaluate('document.querySelector("#recovery-human").click();document.querySelector("button[type=submit]").click();location.hash="#/login"');await delay(1000);assert.equal(await evaluate('location.hash'),'#/login','late response does not redirect')
  const measurements=[]
  for(const [width,height] of [[1440,820],[1366,768],[1024,768],[900,900],[768,900],[375,667],[320,568]]){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false})
    for(const hash of ['#/forgot-password','#/check-email']){
      await go(hash)
      for(const state of ['initial','feedback']){
        if(state==='feedback'){
          if(hash==='#/forgot-password')await evaluate('document.querySelector("button[type=submit]").click()')
          else {await select('resend-outcome','error');await evaluate('document.querySelector(".recovery-resend").click()');await delay(950)}
        }
        const metrics=await evaluate('({overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth,images:[...document.images].every(i=>i.complete&&i.naturalWidth>0),inputs:[...document.querySelectorAll(".input-shell")].every(i=>i.getBoundingClientRect().height>=44),busy:document.querySelector("form").getAttribute("aria-busy")})')
        assert.equal(metrics.overflow,0);assert.ok(metrics.images);assert.ok(metrics.inputs);assert.equal(metrics.busy,'false')
        await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})');assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'))
        await evaluate('scrollTo(0,0)');measurements.push({width,height,hash,state,...metrics})
        if(state==='initial'&&[1440,375].includes(width)){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/recovery-'+hash.slice(2)+'-'+width+'.png',Buffer.from(shot.data,'base64'))}
      }
    }
  }
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[])
  assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  await writeFile('node_modules/.tmp/recovery-measurements.json',JSON.stringify(measurements,null,2))
  console.log('PASS: recovery validation/focus/ARIA, pending duplicate guard, request/resend success/error, keyboard resend, fixed synthetic address, navigation/history/refresh/unmount, 28 responsive cases, loaded images, no API/storage/cookies/runtime errors')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
