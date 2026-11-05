import { spawn } from 'node:child_process'
import { mkdir, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'
export const webRoot = fileURLToPath(new URL('../../', import.meta.url))
export const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
export const origin = process.env.GOVAI_WEB_TEST_URL || 'https://localhost:5173'
if (new URL(origin).protocol !== 'https:') throw Error('Browser checks require HTTPS.')

// Optional frontend-only fixture relay: HTTPS responses are fulfilled by CDP
// from a loopback Vite server. No certificate checks are disabled; no real API
// is reached. Leave unset for live browser checks requiring real HTTPS trust.

const fixtureFrontend = process.env.GOVAI_FIXTURE_FRONTEND_URL
if (fixtureFrontend) {
 const url = new URL(fixtureFrontend)
 if (url.protocol !== 'http:' || !['127.0.0.1','localhost','[::1]'].includes(url.hostname) || url.username || url.password) throw Error('Fixture frontend must be loopback HTTP.')
}
export const frontendFixturePatterns = fixtureFrontend ? [{urlPattern: origin+'/*'}] : []

export async function openBrowser() {
  const profile = resolve(webRoot, `node_modules/.tmp/auth-browser-${process.pid}-${Date.now()}`)
  await mkdir(profile, { recursive: true })
  const child = spawn(process.env.GOVAI_TEST_BROWSER || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', ['--headless=new', '--disable-extensions', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore', windowsHide: true })
  let launchError, socket, closed=false
  const pending = new Map(), exceptions = []
  child.on('error', error => { launchError = error })
  try {
    let target
    for(let i=0;i<50;i++) {
      if(launchError) throw launchError
      assert.equal(child.exitCode,null,'Test browser remains running')
      try {
        const port = (await readFile(resolve(profile,'DevToolsActivePort'),'utf8')).split('\n')[0].trim()
        target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`,{method:'PUT',signal:AbortSignal.timeout(1000)})).json()
        break
      } catch { await delay(200) }
    }
    assert.ok(target?.webSocketDebuggerUrl,'Browser debugger available')
    socket = new WebSocket(target.webSocketDebuggerUrl)
    await new Promise((ok,no)=>{socket.onopen=ok;socket.onerror=no})
    let id=0
    socket.addEventListener('message',({data})=>{
      const m=JSON.parse(data)
      if(m.method==='Runtime.exceptionThrown') exceptions.push(m.params)
      const entry=pending.get(m.id)
      if(entry){clearTimeout(entry.timer);pending.delete(m.id);if(m.error)entry.reject(Error(m.error.message));else entry.resolve(m.result)}
    })
    const send=(method,params={})=>new Promise((resolve,reject)=>{
      const callId=++id
      const timer=setTimeout(()=>{pending.delete(callId);reject(Error('Debugger timeout: '+method))},10000)
      pending.set(callId,{resolve,reject,timer});socket.send(JSON.stringify({id:callId,method,params}))
    })
    const evaluate=async expression=>{
      const r=await send('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true})
      if(r.exceptionDetails)throw Error('Browser evaluation failed; sensitive values withheld.')
      return r.result.value
    }
    await send('Page.bringToFront');await send('Runtime.enable');await send('Network.enable')
    if (fixtureFrontend) {
      socket.addEventListener('message', ({data}) => {
        const event=JSON.parse(data)
        if(closed || event.method!=='Fetch.requestPaused' || new URL(event.params.request.url).origin!==origin) return
        void (async()=>{
          try {
            const url=new URL(event.params.request.url)
            const response=await fetch(new URL(url.pathname+url.search,fixtureFrontend),{redirect:'error'})
            const body=Buffer.from(await response.arrayBuffer()).toString('base64')
            if(closed)return
            await send('Fetch.fulfillRequest',{requestId:event.params.requestId,responseCode:response.status,responseHeaders:[{name:'Content-Type',value:response.headers.get('content-type')||'application/octet-stream'},{name:'Cache-Control',value:'no-store'}],body})
          } catch {
            if(closed)return
            exceptions.push({message:'Frontend fixture relay failed'})
            try { await send('Fetch.failRequest',{requestId:event.params.requestId,errorReason:'Failed'}) } catch { /* Target may have closed during fixture cleanup. */ }
          }
        })()
      })
      await send('Fetch.enable',{patterns:frontendFixturePatterns})
    }

    const until=async(expression,label)=>{
      for(let i=0;i<60;i++){if(await evaluate(expression))return;await delay(100)}
      throw Error('Timed out: '+label)
    }
    const go=async(hash)=>{await send('Page.navigate',{url:new URL(hash,origin+'/').href});await until('Boolean(document.querySelector("h1"))','page heading')}
    const fill=async(id,value)=>evaluate(`(()=>{const i=document.getElementById(${JSON.stringify(id)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,${JSON.stringify(value)});i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
    const close=()=>{closed=true;for(const p of pending.values())clearTimeout(p.timer);socket.close();child.kill()}
    return {socket,send,evaluate,until,go,fill,close,exceptions}
  }catch(error){closed=true;socket?.close();child.kill();throw error}
}
