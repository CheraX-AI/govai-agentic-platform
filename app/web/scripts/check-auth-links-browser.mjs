import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/registration-browser-profile-${process.pid}`)
await mkdir(profile, { recursive: true })
const browser = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', ['--headless=new', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore', windowsHide: true })
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
  const contrast=(a,b)=>{const lum=c=>{const s=c.map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4});return s[0]*.2126+s[1]*.7152+s[2]*.0722};return (Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05)}
  assert.ok(contrast([163,15,45],[255,255,255])>=4.5);assert.ok(contrast([128,4,15],[255,255,255])>=4.5)
  const evidence=[]
  for(const [hash,selector,destination] of [['#/login','a[href="#/forgot-password"]','#/forgot-password'],['#/login','a[href="#/register"]','#/register'],['#/register','.create-account a','#/login'],['#/forgot-password','.recovery-back','#/login']]){
    await send('Page.navigate',{url:'http://127.0.0.1:5173/'+hash});await delay(150);await send('Page.reload');await delay(350);await evaluate('document.fonts.ready')
    const {root}=await send('DOM.getDocument');const {nodeId}=await send('DOM.querySelector',{nodeId:root.nodeId,selector})
    const styles=()=>evaluate('(()=>{const e=document.querySelector('+JSON.stringify(selector)+'),s=getComputedStyle(e);return {color:s.color,offset:s.textUnderlineOffset,thickness:s.textDecorationThickness,skip:s.textDecorationSkipInk,line:s.textDecorationLine,size:s.fontSize,outline:s.outlineStyle,width:s.outlineWidth,href:e.hash}})()')
    const normal=await styles();assert.equal(normal.color,'rgb(163, 15, 45)');assert.equal(normal.skip,'auto');assert.equal(normal.line,'underline');assert.equal(normal.href,destination);assert.ok(Math.abs(parseFloat(normal.offset)-parseFloat(normal.size)*.2)<.01);assert.ok(Math.abs(parseFloat(normal.thickness)-parseFloat(normal.size)*.08)<.01)
    await send('CSS.forcePseudoState',{nodeId,forcedPseudoClasses:['hover']});assert.equal((await styles()).color,'rgb(128, 4, 15)')
    await send('CSS.forcePseudoState',{nodeId,forcedPseudoClasses:[]})
    await evaluate('document.querySelector('+JSON.stringify(selector)+').focus()')
    await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Tab',code:'Tab',windowsVirtualKeyCode:9});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Tab',code:'Tab',windowsVirtualKeyCode:9})
    await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Tab',code:'Tab',windowsVirtualKeyCode:9,modifiers:8});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Tab',code:'Tab',windowsVirtualKeyCode:9,modifiers:8})
    assert.equal(await evaluate('document.activeElement===document.querySelector('+JSON.stringify(selector)+')'),true)
    const focused=await styles();assert.equal(focused.color,'rgb(163, 15, 45)');assert.equal(focused.outline,'solid');assert.ok(parseFloat(focused.width)>=3)
    await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r'});await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});await delay(150);assert.equal(await evaluate('location.hash'),destination)
    await evaluate('history.back()');await delay(150);assert.equal(await evaluate('location.hash'),hash)
    const visited=await styles();assert.equal(visited.color,'rgb(163, 15, 45)')
    assert.ok(await evaluate('[...document.styleSheets].some(sheet=>[...sheet.cssRules].some(rule=>rule.selectorText===".login-layout a:not(.primary-button):visited"&&rule.style.color==="var(--brand)"))'),'explicit visited rule, same accessible brand colour; browser hides visited computed-state distinctions')
    evidence.push({hash,selector,normal,focused,visited})
  }
  await send('Page.navigate',{url:'http://127.0.0.1:5173/#/check-email'});await delay(350);assert.equal(await evaluate('document.querySelector(".recovery-back").hash'),'#/login','confirmation primary action destination preserved')
  assert.deepEqual(apiRequests,[]);assert.deepEqual(exceptions,[])
  await writeFile('node_modules/.tmp/auth-link-states.json',JSON.stringify({normalContrast:contrast([163,15,45],[255,255,255]),hoverContrast:contrast([128,4,15],[255,255,255]),evidence},null,2))
  console.log('PASS: auth text-link normal/visited/hover/keyboard-focus styles, .2em offset/.08em thickness/skip-ink, >=4.5:1 contrast, Enter activation/destinations/history unchanged; primary action preserved')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
