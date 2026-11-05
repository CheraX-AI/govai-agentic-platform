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
  await send('Page.enable')
  await send('Network.enable')





  const production=process.argv[2]==='production', origin=production?'http://127.0.0.1:4173':'http://127.0.0.1:5173'
  const go=async(hash)=>{await send('Page.navigate',{url:'about:blank'});await delay(80);await send('Page.navigate',{url:origin+'/'+hash});await delay(700)}
  const ev=evaluate
  const fill=async(value)=>ev('(()=>{const t=document.querySelector("#chat-draft");Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value").set.call(t,'+JSON.stringify(value)+');t.dispatchEvent(new Event("input",{bubbles:true}))})()')
  const key=async(key,shift=false)=>{const code=key==='Enter'?13:key==='Tab'?9:key==='Escape'?27:40;await send('Input.dispatchKeyEvent',{type:'keyDown',key,code:key,windowsVirtualKeyCode:code,modifiers:shift?8:0,...(key==='Enter'?{text:'\r'}:{})});await send('Input.dispatchKeyEvent',{type:'keyUp',key,code:key,windowsVirtualKeyCode:code,modifiers:shift?8:0})}
  const count=()=>ev('document.querySelectorAll(".chat-message.citizen").length')
  const aiCount=()=>ev('document.querySelectorAll(".chat-message.ai").length')
  const route=async(hash)=>{await ev('location.hash='+JSON.stringify(hash));await delay(120)}
  const seed=()=>ev('(()=>{const now=new Date().getTime();sessionStorage.setItem("govai.dev.mock-session.v1",JSON.stringify({version:1,kind:"demo",userId:"synthetic-harshan",createdAt:now,expiresAt:now+900000}))})()')
  const submit=async(text)=>{await fill(text);await ev('document.querySelector(".chat-send").click()')}
  await go('#/app/chat');assert.match(await ev('location.hash'),/^#\/login\?returnTo=%23%2Fapp%2Fchat/);assert.equal(await count(),0)
  if(production){
    await seed();await go('#/app/chat');assert.match(await ev('location.hash'),/^#\/login\?/);assert.equal(await ev('!!document.querySelector(".chat-page")'),false)
    console.log('PASS production protected Chat rejects seeded synthetic sessions; no chat content')
  }else{
    // Login must preserve the newly allowlisted destination, not bypass its guard.
    for(const [id,value] of [['phone','0771234567'],['password','demo-only']])await ev('(()=>{const t=document.getElementById('+JSON.stringify(id)+');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(t,'+JSON.stringify(value)+');t.dispatchEvent(new Event("input",{bubbles:true}))})()')
    await ev('document.querySelector("#human-check").click();document.querySelector("button[type=submit]").click()');await delay(1200)
    assert.equal(await ev('location.hash'),'#/app/chat');assert.equal(await ev('document.title'),'GovAI | Chat - mock application');assert.equal(await count(),0)
    assert.match(await ev('document.querySelector(".chat-simulated-label").textContent'),/Simulated replies/)
    assert.equal(await ev('!!document.querySelector(".chat-content .layout-preview-label")'),false)
    // Compact single-line composer, automatic multiline cap and internal scrolling.
    await fill('Hello');await delay(40)
    let composerMetrics=await ev('(()=>{const f=document.querySelector(".chat-composer").getBoundingClientRect();const t=document.querySelector("#chat-draft");return {height:f.height,resize:getComputedStyle(t).resize,controls:[...document.querySelectorAll(".chat-composer > button")].map(b=>({width:b.getBoundingClientRect().width,height:b.getBoundingClientRect().height,centre:b.getBoundingClientRect().top+b.getBoundingClientRect().height/2-f.top-f.height/2}))}})()')
    assert.ok(composerMetrics.height>=60&&composerMetrics.height<=64,JSON.stringify(composerMetrics));assert.equal(composerMetrics.resize,'none');assert.ok(composerMetrics.controls.every(b=>b.width>=44&&b.height>=44&&Math.abs(b.centre)<1))
    await fill(Array(20).fill('Multiline draft').join('\n'));await delay(50)
    assert.equal(await ev('document.querySelector("#chat-draft").getBoundingClientRect().height'),160);assert.equal(await ev('getComputedStyle(document.querySelector("#chat-draft")).overflowY'),'auto')
    assert.ok(await ev('document.querySelector("#chat-draft").scrollHeight>document.querySelector("#chat-draft").clientHeight'))
    await fill('Short again');await delay(40);assert.equal(await ev('document.querySelector(".chat-composer").getBoundingClientRect().height'),62)
    await fill(' \n\t ');assert.equal(await ev('document.querySelector(".chat-send").disabled'),true)
    await ev('document.querySelector(".chat-composer").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))');assert.equal(await count(),0)
    await fill('NIC demo');await ev('document.querySelector("#chat-draft").focus()');await key('Enter',true)
    assert.equal(await count(),0);assert.match(await ev('document.querySelector("#chat-draft").value'),/\n/)
    await ev('(()=>{const t=document.querySelector("#chat-draft");t.dispatchEvent(new CompositionEvent("compositionstart",{bubbles:true}));t.dispatchEvent(new KeyboardEvent("keydown",{key:"Enter",isComposing:true,bubbles:true,cancelable:true}))})()');assert.equal(await count(),0)
    await ev('document.querySelector("#chat-draft").dispatchEvent(new CompositionEvent("compositionend",{bubbles:true}))')
    await fill('How do I renew my NIC?');await key('Enter');assert.equal(await count(),1,'citizen appears before async result');assert.equal(await aiCount(),0)
    assert.equal(await ev('document.querySelector(".chat-send").disabled'),true);assert.match(await ev('document.querySelector(".chat-feedback").textContent'),/preparing/)
    await ev('document.querySelector(".chat-composer").dispatchEvent(new Event("submit",{bubbles:true,cancelable:true}))');await key('Enter');assert.equal(await count(),1)
    await delay(1100);assert.equal(await aiCount(),1);assert.match(await ev('document.querySelector(".ai .message-bubble").textContent'),/not verified government guidance/)
    assert.equal(await ev('document.querySelector(".chat-sources a").href'),'https://example.com/');assert.equal(await ev('document.querySelector(".chat-sources a").rel'),'noopener noreferrer')
    await ev('document.querySelector(".chat-sources summary").focus()');await key('Enter');assert.equal(await ev('document.querySelector(".chat-sources").open'),false);await key('Enter')
    await send('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await delay(100)
    await ev('document.activeElement.blur();document.querySelector(".conversation-scroll").scrollTop=0');await delay(80)
    await writeFile('node_modules/.tmp/ga21-desktop.png',Buffer.from((await send('Page.captureScreenshot',{format:'png'})).data,'base64'))
    // Error/retry retains the original citizen message and never appends it twice.
    await ev('document.querySelector(".chat-demo-controls").open=true;var s=document.querySelector("#chat-outcome");s.value="error";s.dispatchEvent(new Event("change",{bubbles:true}))')
    await submit('passport demo');await delay(1100);assert.equal(await count(),2);assert.equal(await aiCount(),1);assert.match(await ev('document.querySelector(".chat-error").textContent'),/messages are still here/)
    await ev('var s=document.querySelector("#chat-outcome");s.value="success";s.dispatchEvent(new Event("change",{bubbles:true}));document.querySelector(".chat-error button").focus()');await key('Enter');await delay(1100);assert.equal(await count(),2);assert.equal(await aiCount(),2)
    // Raw HTML-looking user text remains escaped, no injected element.
    await submit('<img src=x onerror=alert(1)>\nsecond line');await delay(1100);assert.equal(await count(),3);assert.equal(await ev('!!document.querySelector(".message-bubble img")'),false)
    await route('#/app/recent');await route('#/app/home');await ev('document.querySelector(".home-chat-entry a").click()');await delay(120);assert.equal(await count(),3)
    await ev('history.back()');await delay(120);assert.equal(await ev('location.hash'),'#/app/home');await ev('history.forward()');await delay(120);assert.equal(await count(),3)
    // Reset during request cancels and ignores late result.
    await submit('cancel this');await ev('document.querySelector(".chat-new").click()');await delay(1150);assert.equal(await count(),0);assert.equal(await aiCount(),0)
    await route('#/app/home');await submit('Home shared flow');await delay(120);assert.equal(await ev('location.hash'),'#/app/chat');assert.equal(await count(),1);await delay(1100);assert.equal(await aiCount(),1)
    await route('#/app/home');await ev('document.querySelector(".suggestion-preview button").click()');await delay(120);assert.equal(await ev('location.hash'),'#/app/chat');assert.equal(await count(),2);await delay(1100)
    await submit('sidebar reset pending');await ev('document.querySelector(".desktop-sidebar .new-chat").click()');await delay(1150);assert.equal(await count(),0);assert.equal(await aiCount(),0)
    // Build actual overflow and exercise near-bottom versus reading-old-messages.
    for(let i=0;i<4;i++){await submit('long message '+i+'\n'+Array(18).fill('Synthetic transcript line for scrolling.').join('\n'));await delay(1050)}
    assert.equal(await count(),4)
    const scroller=()=>ev('(()=>{var n=document.querySelector(".conversation-scroll");return {top:n.scrollTop,height:n.scrollHeight,client:n.clientHeight}})()')
    let offsets=await scroller();assert.ok(offsets.height>offsets.client);assert.ok(offsets.height-offsets.top-offsets.client<85)
    // Header disclosure preserves the reading anchor, draft, messages and button focus.
    await fill('Draft survives header changes');await ev('var n=document.querySelector(".conversation-scroll");n.scrollTop=120;n.dispatchEvent(new Event("scroll"));window.readingAnchor=[...n.querySelectorAll(".chat-message")].find(m=>m.getBoundingClientRect().bottom>n.getBoundingClientRect().top);window.readingOffset=window.readingAnchor.getBoundingClientRect().top-n.getBoundingClientRect().top;document.querySelector(".chat-header-toggle").focus()');await key('Enter');await delay(80)
    assert.equal(await ev('document.querySelector(".chat-header-toggle").getAttribute("aria-expanded")'),'false')
    assert.equal(await ev('document.querySelector("#chat-header-details").hidden'),true)
    assert.equal(await ev('document.querySelector("#chat-title").getClientRects().length'),0)
    assert.equal(await ev('document.querySelector(".chat-date")'),null)
    assert.ok(await ev('!!document.querySelector(".chat-header-toggle svg")'))
    assert.equal(await ev('document.querySelector("#chat-draft").value'),'Draft survives header changes');assert.equal(await count(),4);assert.ok(await ev('Math.abs(window.readingAnchor.getBoundingClientRect().top-document.querySelector(".conversation-scroll").getBoundingClientRect().top-window.readingOffset)<1'),'same message reading anchor despite removed divider')
    assert.equal(await ev('document.activeElement.getAttribute("aria-label")'),'Expand conversation header')
    assert.equal(await ev('document.querySelector(".chat-simulated-label").getClientRects().length>0'),true)
    assert.equal(await ev('document.querySelectorAll(".chat-content .demo-session-notice").length'),0)
    await key('Enter');await delay(80);assert.equal((await scroller()).top,120);assert.equal(await ev('document.activeElement.getAttribute("aria-label")'),'Collapse conversation header')
    await submit('while reading');await ev('var n=document.querySelector(".conversation-scroll");n.scrollTop=0;n.dispatchEvent(new Event("scroll"))');await delay(1100)
    assert.equal((await scroller()).top,0);assert.equal(await ev('!!document.querySelector(".chat-latest")'),true)
    await ev('document.querySelector(".chat-latest").focus()');await key('Enter');offsets=await scroller();assert.ok(offsets.height-offsets.top-offsets.client<85);assert.equal(await ev('document.activeElement.className'),'conversation-scroll')
    const measurements=[]
    for(const width of [1440,1024,900,768,375,320]){
      await send('Emulation.setDeviceMetricsOverride',{width,height:820,deviceScaleFactor:1,mobile:false});await delay(150)
      await fill('Hi');await delay(40)
      assert.equal(await ev('document.querySelector(".chat-composer").getBoundingClientRect().height'),62,'compact height '+width)
      assert.ok(await ev('[...document.querySelectorAll(".chat-composer > button")].every(b=>b.getBoundingClientRect().width>=44&&b.getBoundingClientRect().height>=44)'))
      const measure=await ev('({width:innerWidth,overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth,scrollOverflow:document.querySelector(".conversation-scroll").scrollWidth-document.querySelector(".conversation-scroll").clientWidth})')
      assert.equal(measure.overflow,0,JSON.stringify(measure));assert.equal(measure.scrollOverflow,0);measurements.push(measure)
      await ev('document.querySelector(".application-content").scrollTop=100000;document.querySelector("#chat-draft").focus()')
      assert.ok(await ev('(()=>{const r=document.querySelector(".chat-send").getBoundingClientRect();return r.right<=innerWidth&&r.bottom<=innerHeight})()'),'composer reachable '+width)
      if(width===375)await writeFile('node_modules/.tmp/ga21-narrow.png',Buffer.from((await send('Page.captureScreenshot',{format:'png'})).data,'base64'))
      await ev('document.querySelector(".chat-header-toggle").focus()');await key('Enter');await delay(50)
      assert.equal(await ev('document.querySelector(".chat-header-toggle").getAttribute("aria-expanded")'),'false')
      assert.equal(await ev('document.documentElement.scrollWidth-document.documentElement.clientWidth'),0,'collapsed overflow '+width)
      assert.ok(await ev('(()=>{const r=document.querySelector(".chat-heading").getBoundingClientRect();const c=document.querySelector(".conversation-wrapper").getBoundingClientRect();const p=document.querySelector(".chat-page").getBoundingClientRect();return r.height===44&&r.width<180&&r.right<=p.right+1&&Math.abs(r.right-p.right)<1&&r.bottom<=c.top})()'),'compact right-aligned controls never overlap messages '+width)
      assert.equal(await ev('document.querySelector("#chat-title").getClientRects().length'),0)
      assert.equal(await ev('document.querySelector(".chat-date")'),null)
      assert.equal(await ev('document.querySelector(".chat-bottom .chat-simulated-label").getClientRects().length>0'),true)
      assert.equal(await ev('document.querySelector("#chat-draft").value'),'Hi')
      if(width===375)await writeFile('node_modules/.tmp/ga21-header-collapsed-narrow.png',Buffer.from((await send('Page.captureScreenshot',{format:'png'})).data,'base64'))
      await key('Enter');await delay(50);assert.equal(await ev('document.querySelector("#chat-header-details").hidden'),false)
      if(width===375){await ev('document.querySelector(".menu-toggle").focus()');await key('Enter');await delay(80);assert.equal(await ev('document.querySelector("dialog").open'),true);await key('Escape');assert.equal(await ev('document.activeElement.className'),'menu-toggle')}
    }
    await writeFile('node_modules/.tmp/ga21-measurements.json',JSON.stringify(measurements,null,2))
    // Reload clears transcript while restoring only allowed synthetic auth metadata.
    await send('Page.reload');await delay(600);assert.equal(await count(),0);assert.equal(await aiCount(),0)
    await submit('logout pending');await ev('document.querySelector(".synthetic-user").focus()');await key('Enter');await delay(80);assert.equal(await ev('document.activeElement.textContent'),'Demo logout');await key('Enter');await delay(1200);assert.equal(await ev('location.hash'),'#/login');assert.equal(await ev('sessionStorage.length'),0)
    await seed();await go('#/app/chat');assert.equal(await count(),0);assert.equal(await aiCount(),0)
    await submit('expiry pending');await ev('const actualNow=Date.now;Date.now=()=>actualNow()+901000;window.dispatchEvent(new Event("focus"))');await delay(1200);assert.match(await ev('location.hash'),/reason=expired/);assert.equal(await ev('!!document.querySelector(".chat-page")'),false)
    await seed();await go('#/app/chat');assert.equal(await count(),0);assert.equal(await aiCount(),0)
    await submit('unmount pending');await route('#/login');await delay(1100);await route('#/app/chat');assert.equal(await count(),0);assert.equal(await aiCount(),0)
    // Exercise the replaceable hook boundary with a deferred service, not fixture topic heuristics.
    await ev(`(async()=>{
      const React=await import('/node_modules/.vite/deps/react.js');const dom=await import('/node_modules/.vite/deps/react-dom_client.js');const createRoot=dom.createRoot??dom.default.createRoot;const {useConversation}=await import('/src/chat/useConversation.ts');
      window.contextRequests=[];window.contextPending=[];
      const service=(request,options)=>{window.contextRequests.push(structuredClone(request));return new Promise((resolve,reject)=>window.contextPending.push({resolve,reject,signal:options.signal}))};
      const host=document.createElement('div');document.body.append(host);window.contextRoot=createRoot(host);
      function Harness(){window.contextChat=useConversation(service);return null}window.contextRoot.render((React.createElement??React.default.createElement)(Harness));
    })()`);await delay(100)
    await ev('window.contextChat.submit("First free-text question")');await delay(30)
    assert.deepEqual(await ev('window.contextRequests[0].messages'),[{role:'user',content:'First free-text question'}])
    await ev('window.contextPending[0].resolve({text:"First assistant reply",sources:[]})');await delay(40)
    await ev('window.contextChat.submit("What about that previous answer?")');await delay(30)
    const expectedContext=[{role:'user',content:'First free-text question'},{role:'assistant',content:'First assistant reply'},{role:'user',content:'What about that previous answer?'}]
    assert.deepEqual(await ev('window.contextRequests[1].messages'),expectedContext)
    await ev('window.contextPending[1].reject(Error("controlled failure"))');await delay(40);await ev('window.contextChat.retry()');await delay(30)
    assert.deepEqual(await ev('window.contextRequests[2].messages'),expectedContext,'retry preserves full context without repeating user turn')
    await ev('window.contextPending[2].resolve({text:"Follow-up reply",sources:[]})');await delay(40)
    assert.equal(await ev('window.contextChat.messages.length'),4)
    await ev('window.contextChat.submit("cancelled follow-up")');await delay(20);await ev('window.contextChat.reset()');await delay(20)
    assert.equal(await ev('window.contextPending[3].signal.aborted'),true)
    await ev('window.contextPending[3].resolve({text:"late ignored reply",sources:[]})');await delay(40);assert.equal(await ev('window.contextChat.messages.length'),0)
    await ev('window.contextRoot.unmount()')
    assert.equal(await ev('localStorage.length'),0);assert.equal(await ev('Object.keys(JSON.parse(sessionStorage.getItem("govai.dev.mock-session.v1"))).sort().join(",")'),'createdAt,expiresAt,kind,userId,version')
    console.log('PASS Chat: protected login return, blank/Enter/Shift+Enter/IME, immediate message/duplicate guard, pending/success/error/retry, escaped text/synthetic safe sources, shared Home/Recent/Chat/history state, full user/assistant context at service boundary, retry context, reset/late reply cancellation, scrolling/latest focus, six responsive widths/screenshots/menu, reload/logout/expiry/unmount clearing')
  }
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[]);assert.deepEqual((await send('Network.getCookies')).cookies,[])
  await send('Browser.close')
} finally {
  for(const call of pending.values())clearTimeout(call.timer)
  socket?.close();browser.kill()
}
