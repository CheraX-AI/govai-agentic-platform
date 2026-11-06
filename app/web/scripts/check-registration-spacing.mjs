import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/registration-spacing-profile-${process.pid}`)
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


  const measurements=[]
  const sizes=[[1440,820],[1366,768],[1536,864],[1920,1080],[1024,768],[768,900],[375,667],[320,568]]
  if(process.argv[3]&&process.argv[4])sizes.unshift([Number(process.argv[3]),Number(process.argv[4])])
  await send('Page.navigate',{url:'http://127.0.0.1:5173/#/register'});await delay(600)
  await send('Input.dispatchKeyEvent',{type:'keyDown',key:'0',code:'Digit0',windowsVirtualKeyCode:48,modifiers:2})
  await send('Input.dispatchKeyEvent',{type:'keyUp',key:'0',code:'Digit0',windowsVirtualKeyCode:48,modifiers:2})
  for(const [width,height] of sizes){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false})
    for(const state of ['initial','errors','expanded-demo','errors+expanded-demo']){
      await send('Page.reload');await delay(250)
      if(state.includes('errors'))await evaluate('document.querySelector("button[type=submit]").click()')
      if(state.includes('expanded'))await evaluate('document.querySelector("details summary").click()')
      const metrics=await evaluate(`(() => {const root=document.documentElement,c=document.querySelector('.login-card'),p=document.querySelector('.form-panel'),s=getComputedStyle(c);return {viewport:[innerWidth,innerHeight],vertical:Math.max(0,root.scrollHeight-innerHeight),horizontal:root.scrollWidth-root.clientWidth,card:[c.offsetWidth,c.offsetHeight],cardPadding:s.padding,cardGap:s.gap,panelPadding:getComputedStyle(p).padding,panelGap:getComputedStyle(p).gap,inputHeight:document.querySelector('.input-shell').getBoundingClientRect().height,buttonHeight:document.querySelector('button[type=submit]').getBoundingClientRect().height,visualScale:visualViewport.scale,cssZoom:getComputedStyle(root).zoom,errors:document.querySelectorAll('.field-error').length,detailsOpen:document.querySelector('details').open}})()`)
      assert.equal(metrics.horizontal,0)
      assert.equal(metrics.visualScale,1)
      assert.equal(metrics.cssZoom,'1')
      assert.ok(metrics.inputHeight>=46&&metrics.buttonHeight>=50,'control sizes preserved')
      assert.equal(metrics.errors,state.includes('errors')?7:0)
      assert.equal(metrics.detailsOpen,state.includes('expanded'))
      await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})')
      assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'),'footer reachable')
      measurements.push({width,height,state,...metrics})
    }
  }
  const output=process.argv[2]||'node_modules/.tmp/registration-spacing-after.json'
  await writeFile(output,JSON.stringify(measurements,null,2))
  console.table(measurements.filter(m=>m.width>=1024).map(({width,height,state,vertical,card})=>({width,height,state,vertical,card})))
  console.log('PASS: separate initial/error/disclosure states at 100% scale; zero horizontal overflow; full-size controls and natural scroll reachability; '+output)
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
