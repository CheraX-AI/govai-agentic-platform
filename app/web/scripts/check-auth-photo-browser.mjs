import { installAuthFixtures } from './integration/authFixtures.mjs'
import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/auth-photo-profile-${process.pid}`)
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
      if (message.error) call.reject(new Error(JSON.stringify(message.error))); else call.resolve(message.result)
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
  const fixture = await installAuthFixtures(socket, send)


  const phase=process.argv[2]||'after',measurements=[]
  for(const page of ['login','register','forgot-password','check-email'])for(const [width,height] of [[1440,820],[1366,768],[1920,1080],[1024,768],[901,600],[1440,600],[375,667]]){
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false})
    for(const state of ['initial','errors','expanded','errors+expanded']){
      await send('Page.navigate',{url:'https://localhost:5173/#/'+page});await delay(150);await send('Page.reload');await delay(250);for(let ready=0;ready<60;ready++){if(await evaluate('Boolean(document.querySelector(".login-photo")?.naturalWidth)'))break;await delay(100)}
      if(state.includes('errors')) {
        if(page==='check-email'){await evaluate('document.querySelector(".recovery-resend").click()')}
        else await evaluate('document.querySelector("button[type=submit]").click()')
      }
      if(state.includes('expanded'))await evaluate('document.querySelector("details summary")?.click()')
      await evaluate('scrollTo(0,0)')
      if(!await evaluate('Boolean(document.querySelector(".login-photo"))')) throw Error('Photo page not ready: '+page+' '+state+' '+width)
      const m=await evaluate(`(() => {const p=document.querySelector('.photo-panel'),i=document.querySelector('.login-photo'),c=document.querySelector('.login-card'),s=getComputedStyle(i),r=e=>{const b=e.getBoundingClientRect();return {x:b.x,y:b.y,w:b.width,h:b.height,bottom:b.bottom}},ir=r(i),pr=r(p);const scale=Math.max(ir.w/i.naturalWidth,ir.h/i.naturalHeight);return {panel:pr,imageBox:ir,natural:[i.naturalWidth,i.naturalHeight],objectFit:s.objectFit,objectPosition:s.objectPosition,offsets:[s.left,s.top],scale,rendered:[i.naturalWidth*scale,i.naturalHeight*scale],caption:r(document.querySelector('.slide-caption')),badge:r(document.querySelector('.help-badge')),form:r(c),vertical:document.documentElement.scrollHeight-innerHeight,horizontal:document.documentElement.scrollWidth-document.documentElement.clientWidth,visible:getComputedStyle(p).display!=='none'}})()`)
      assert.equal(m.horizontal,0)
      assert.deepEqual(m.natural,[985,977])
      if(phase==='after'&&m.visible){assert.equal(m.panel.h,height,'viewport-height photo independent of form');assert.equal(m.objectFit,'cover');assert.equal(m.objectPosition,'100% 0%');assert.equal(m.imageBox.w,m.panel.w);assert.equal(m.imageBox.h,m.panel.h,'no oversized image box');
        // Source coordinates in the documented coastal crop; top/right cover offsets.
        for(const [name,x1,y1,x2,y2] of [['flag',790,17,950,135],['lighthouse top',685,48,760,140]]) {
          const left=m.panel.w-m.rendered[0];
          assert.ok(left+x1*m.scale>=0 && left+x2*m.scale<=m.panel.w+1 && y1*m.scale>=0 && y2*m.scale<=m.panel.h, name+' fully retained at '+width+'x'+height);
        }
        assert.ok(m.caption.y>=0&&m.caption.bottom<m.badge.y,'caption fits above badge');assert.ok(m.badge.bottom<=height&&m.badge.y>=0,'badge in initial viewport')}
      if((width===1440&&height===820&&(state==='initial'||state==='errors+expanded'))||(width===1024&&state==='initial')){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/photo-'+phase+'-'+page+'-'+width+'-'+state+'.png',Buffer.from(shot.data,'base64'))}
      await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})')
      assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'),'natural form scrolling')
      if(phase==='after'&&m.visible){const bottom=await evaluate('document.querySelector(".help-badge").getBoundingClientRect().bottom');assert.ok(bottom<=height&&bottom>0,'sticky badge reachable after scroll')}
      measurements.push({page,width,height,state,...m})
    }
  }
  await writeFile('node_modules/.tmp/auth-photo-'+phase+'.json',JSON.stringify(measurements,null,2))
  console.log('PASS: '+measurements.length+' auth-photo '+phase+' cases; natural dimensions/crop/fit recorded; form scroll reachable'+(phase==='after'?'; viewport photo and nonoverlapping overlays verified':''))
  assert.deepEqual(fixture.errors,[])
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
