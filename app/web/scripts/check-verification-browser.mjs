import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/registration-browser-profile-${process.pid}`)
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
  const rawGo=async(hash)=>{await send('Page.navigate',{url:origin+'/'+hash});await delay(150);await send('Page.reload');await delay(400);for(let attempt=0;attempt<40;attempt++){if(await evaluate('!!document.querySelector("h1") && !document.querySelector("vite-error-overlay")'))break;await delay(100)}}
  const fill=async(id,value)=>evaluate('(()=>{const i=document.getElementById('+JSON.stringify(id)+');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(i,'+JSON.stringify(value)+');i.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const key=async(key,code)=>{await send('Input.dispatchKeyEvent',{type:'keyDown',key,code:key,windowsVirtualKeyCode:code,...(key==='Enter'?{text:'\r'}:{})});await send('Input.dispatchKeyEvent',{type:'keyUp',key,code:key,windowsVirtualKeyCode:code})}
  const paste=async(text,index=0)=>evaluate('(()=>{const data=new DataTransfer();data.setData("text",'+JSON.stringify(text)+');document.getElementById("code-'+index+'").dispatchEvent(new ClipboardEvent("paste",{clipboardData:data,bubbles:true,cancelable:true}))})()')
  const advance=async(ms)=>{await evaluate('window.demoOffset=(window.demoOffset||0)+'+ms+';if(!window.originalNow){window.originalNow=Date.now;Date.now=()=>window.originalNow()+window.demoOffset}');await delay(320)}
  const select=async(value)=>evaluate('(()=>{const s=document.querySelector("#code-outcome");s.value='+JSON.stringify(value)+';s.dispatchEvent(new Event("change",{bubbles:true}))})()')

  const go=async(hash)=>{
    if(hash!=='#/verify-code'){await rawGo(hash);return}
    await rawGo('#/forgot-password');await fill('recovery-nic','123456789v');await evaluate('document.querySelector("#recovery-phone").click();document.querySelector("#recovery-human").click();document.querySelector("button[type=submit]").click()');await delay(950)
  }
  await go('#/verify-code')
  assert.equal(await evaluate('location.hash'),'#/verify-code');assert.equal(await evaluate('document.title'),'GovAI | Recovery code verification preview');assert.equal(await evaluate('document.activeElement.id'),'verification-heading');assert.equal(await evaluate('!!document.querySelector("#password")'),false)
  assert.equal(await evaluate('document.querySelector(".verification-warning").textContent'),'Demo - no code was sent.');assert.match(await evaluate('document.querySelector(".heading p").textContent'),/Synthetic number.*0000/)
  assert.ok(await evaluate('document.querySelector(".verification-resend").disabled'));assert.match(await evaluate('document.querySelector(".verification-timer").textContent'),/4:5\d|5:00/)
  await evaluate('document.querySelector("#code-0").focus()')
  for(const digit of '123456')await send('Input.insertText',{text:digit})
  assert.equal(await evaluate('[...document.querySelectorAll(".verification-code input")].map(i=>i.value).join("")'),'123456','keyboard text entry auto-advances across all six slots')
  for(let index=0;index<6;index++)await fill('code-'+index,'')
  await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('document.activeElement.id'),'code-0');assert.match(await evaluate('document.querySelector("#code-error").textContent'),/Enter the six/)
  await fill('code-0','1');assert.equal(await evaluate('document.activeElement.id'),'code-1');await key('Backspace',8);assert.equal(await evaluate('document.activeElement.id'),'code-0');assert.equal(await evaluate('document.querySelector("#code-0").value'),'')
  await paste('12');await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('document.activeElement.id'),'code-2');assert.match(await evaluate('document.querySelector("#code-error").textContent'),/all six/);assert.equal(await evaluate('document.querySelector("#code-2").getAttribute("aria-describedby")'),'code-hint code-error')
  await paste('12x456');assert.equal(await evaluate('[...document.querySelectorAll(".verification-code input")].map(i=>i.value).join("")'),'12','non-digit paste rejected')
  await paste('000000',3);assert.equal(await evaluate('[...document.querySelectorAll(".verification-code input")].map(i=>i.value).join("")'),'000000','full paste from any slot replaces all')
  await key('ArrowLeft',37);assert.equal(await evaluate('document.activeElement.id'),'code-4');await key('ArrowRight',39);assert.equal(await evaluate('document.activeElement.id'),'code-5')
  await evaluate('window.calls=0;const oldTimeout=window.setTimeout.bind(window);window.setTimeout=(f,ms,...args)=>{if(ms===800)window.calls++;return oldTimeout(f,ms,...args)};document.querySelector("button[type=submit]").click();document.querySelector("form").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))')
  assert.equal(await evaluate('window.calls'),1);assert.ok(await evaluate('[...document.querySelectorAll(".verification-code input,#code-outcome")].every(i=>i.matches(":disabled"))'));assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/Simulating/);await delay(950);assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/Invalid demo code/)
  await paste('123456');await select('expired');await evaluate('document.querySelector("button[type=submit]").click()');await delay(950);assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/expired/);assert.ok(await evaluate('document.querySelector("button[type=submit]").disabled'))
  await advance(46000);await evaluate('document.querySelector(".verification-resend").click();document.querySelector(".verification-resend").click()');assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/Simulating resend/);await delay(950)
  assert.equal(await evaluate('window.calls'),3,'one invalid verify, one expired verify and one resend only');assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/challenge reset.*No code was sent/);assert.equal(await evaluate('[...document.querySelectorAll(".verification-code input")].map(i=>i.value).join("")'),'');assert.equal(await evaluate('document.activeElement.id'),'code-0');assert.match(await evaluate('document.querySelector(".verification-timer").textContent'),/4:5\d|5:00/);assert.ok(await evaluate('document.querySelector(".verification-resend").disabled'))
  await advance(301000);assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/expired/);await evaluate('document.querySelector(".verification-resend").click()');await delay(950);await select('invalid');await paste('123456');await evaluate('document.querySelector("button[type=submit]").click()');await delay(950);assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/Invalid/)
  await select('success');await evaluate('document.querySelector("button[type=submit]").click()');await delay(900);assert.match(await evaluate('document.querySelector("[role=status]").textContent'),/verification successful.*not authenticated/);await delay(700)

  assert.equal(await evaluate('location.hash'),'#/reset-password');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false)
  assert.equal(await evaluate('document.title'),'GovAI | Password reset preview');assert.equal(await evaluate('document.activeElement.id'),'reset-heading')
  assert.deepEqual(await evaluate('["#reset-password","#reset-confirmation"].map(s=>document.querySelector(s).type)'),['password','password'])
  assert.equal(await evaluate('document.querySelectorAll(".icon-button")[1].getAttribute("aria-label")'),'Show confirm new password');await evaluate('document.querySelectorAll(".icon-button")[1].click()')
  assert.equal(await evaluate('document.querySelector("#reset-password").type'),'password');assert.equal(await evaluate('document.querySelector("#reset-confirmation").type'),'text')
  await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('document.activeElement.id'),'reset-password')
  await fill('reset-password','abcdef1!');await fill('reset-confirmation','wrong');await evaluate('document.querySelector("button[type=submit]").click()');assert.equal(await evaluate('document.activeElement.id'),'reset-confirmation')
  await fill('reset-password',' abcdef1! ');await fill('reset-confirmation',' abcdef1! ');assert.equal(await evaluate('document.querySelector("#reset-password").value'),' abcdef1! ')
  await evaluate('var s=document.querySelector("#reset-outcome");s.value="error";s.dispatchEvent(new Event("change",{bubbles:true}));document.querySelector("button[type=submit]").click()');await delay(950)
  assert.match(await evaluate('document.querySelector("[role=alert]").textContent'),/Demo reset failed.*No password was changed/);assert.equal(await evaluate('document.querySelector("#reset-password").value'),'');assert.equal(await evaluate('location.hash'),'#/reset-password')
  await fill('reset-password','abcdef1!');await fill('reset-confirmation','abcdef1!');await evaluate('var s=document.querySelector("#reset-outcome");s.value="success";s.dispatchEvent(new Event("change",{bubbles:true}));document.querySelector("button[type=submit]").click();document.querySelector("form").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))')
  assert.ok(await evaluate('[...document.querySelectorAll("form input,form button")].every(i=>i.disabled)'));await delay(950)
  assert.equal(await evaluate('document.querySelector("[role=status]").textContent'),'Demo reset successful. No password was changed.');assert.equal(await evaluate('document.querySelector("#reset-password").value'),'');assert.equal(await evaluate('document.querySelector("#reset-confirmation").value'),'')
  await evaluate('[...document.querySelectorAll("a")].find(a=>a.textContent==="Go to login").focus()');await key('Enter',13);await delay(150);assert.equal(await evaluate('location.hash'),'#/login');await evaluate('history.back()');await delay(150);assert.equal(await evaluate('!!document.querySelector("#recovery-required-heading")'),true,'leaving reset invalidates progression')
  await go('#/home');assert.equal(await evaluate('!!document.querySelector(".demo-session-notice")'),false,'recovery never creates a demo session')
  await go('#/verify-code');await evaluate('document.querySelector(".verification-back").focus()');await key('Enter',13);await delay(150);assert.equal(await evaluate('location.hash'),'#/forgot-password')
  await go('#/verify-code');await paste('123456');await evaluate('document.querySelector("button[type=submit]").click();location.hash="#/register"');await delay(1600);assert.equal(await evaluate('location.hash'),'#/register','late verification cannot redirect')
  for(const hash of ['#/verify-code','#/reset-password']){await rawGo(hash);assert.ok(await evaluate('!!document.querySelector("#recovery-required-heading")'));assert.equal(await evaluate('document.querySelector(".primary-button").hash'),'#/forgot-password')}
  await evaluate('document.querySelector(".primary-button").focus()');await key('Enter',13);await delay(150);assert.equal(await evaluate('location.hash'),'#/forgot-password','guard recovery link works from keyboard')
  await go('#/verify-code');await send('Page.reload');await delay(400);assert.ok(await evaluate('!!document.querySelector("#recovery-required-heading")'),'refresh discards recovery progress')
  await go('#/verify-code')
  await send('DOM.enable');await send('CSS.enable');const {root}=await send('DOM.getDocument');const {nodeId}=await send('DOM.querySelector',{nodeId:root.nodeId,selector:'h1'});const {fonts}=await send('CSS.getPlatformFontsForNode',{nodeId});assert.ok(fonts.some(f=>f.isCustomFont&&f.familyName.includes('Inter')))
  const measurements=[]
  for(const [width,height] of [[1440,820],[1366,768],[1024,768],[768,900],[375,667],[320,568]]){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false});await go('#/verify-code')
    for(const state of ['initial','errors']){
      if(state==='errors')await evaluate('document.querySelector("button[type=submit]").click()')
      const m=await evaluate('({overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth,boxes:[...document.querySelectorAll(".verification-code input")].map(i=>({w:i.getBoundingClientRect().width,h:i.getBoundingClientRect().height})),images:[...document.images].every(i=>i.complete&&i.naturalWidth>0)})');assert.equal(m.overflow,0);assert.ok(m.images);assert.ok(m.boxes.every(b=>b.w>=30&&b.h>=44))
      await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})');assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'));await evaluate('scrollTo(0,0)');measurements.push({width,height,state,...m})
      if([1440,375].includes(width)&&state==='initial'){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/verification-'+width+'.png',Buffer.from(shot.data,'base64'))}
    }
  }

  for(const [width,height] of [[1440,820],[1024,768],[768,900],[375,667],[320,568]]){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false});await go('#/verify-code');await paste('123456');await evaluate('document.querySelector("button[type=submit]").click()');await delay(1550);assert.equal(await evaluate('location.hash'),'#/reset-password')
    for(const state of ['initial','errors']){
      if(state==='errors')await evaluate('document.querySelector("button[type=submit]").click()')
      assert.equal(await evaluate('document.documentElement.scrollWidth-document.documentElement.clientWidth'),0,'reset overflow '+width)
      assert.ok(await evaluate('[...document.querySelectorAll(".input-shell")].every(e=>e.getBoundingClientRect().height>=44)'))
      await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})');assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'));await evaluate('scrollTo(0,0)')
      if(state==='initial'&&[1440,375].includes(width)){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/reset-'+width+'.png',Buffer.from(shot.data,'base64'))}
    }
  }
  await send('Page.reload');await delay(400);assert.ok(await evaluate('!!document.querySelector("#recovery-required-heading")'),'reset refresh needs recovery progression')
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[]);assert.equal(await evaluate('localStorage.length+sessionStorage.length'),0);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  await writeFile('node_modules/.tmp/verification-measurements.json',JSON.stringify(measurements,null,2));console.log('PASS '+(production?'production':'development')+': phone recovery->verification->reset(no session), keyboard/paste/validation/ARIA, mock invalid/expired/success,5-minute deadline/45-second cooldown/resend reset/duplicates, no session, reset policy/error/success/clearing, history/direct/refresh/unmount,12 code plus10 reset responsive cases/Inter/assets/no API/storage/errors')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
