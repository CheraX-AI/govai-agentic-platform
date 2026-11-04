import { spawn } from 'node:child_process'
import { mkdir, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'
export const webRoot = fileURLToPath(new URL('../../', import.meta.url))
export const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
export const origin = process.env.GOVAI_WEB_TEST_URL || 'https://localhost:5173'
if (new URL(origin).protocol !== 'https:') throw Error('Browser checks require HTTPS.')
export async function openBrowser() {
  const profile = resolve(webRoot, `node_modules/.tmp/auth-browser-${process.pid}-${Date.now()}`)
  await mkdir(profile, { recursive: true })
  const child = spawn(process.env.GOVAI_TEST_BROWSER || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', ['--headless=new', '--disable-extensions', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore', windowsHide: true })
  let launchError, socket
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
    await send('Runtime.enable');await send('Network.enable')
    const until=async(expression,label)=>{
      for(let i=0;i<60;i++){if(await evaluate(expression))return;await delay(100)}
      throw Error('Timed out: '+label)
    }
    const go=async(hash)=>{await send('Page.navigate',{url:origin+'/'+hash});await until('Boolean(document.querySelector("h1"))','page heading')}
    const fill=async(id,value)=>evaluate(`(()=>{const i=document.getElementById(${JSON.stringify(id)});Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(i,${JSON.stringify(value)});i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
    const close=()=>{for(const p of pending.values())clearTimeout(p.timer);socket.close();child.kill()}
    return {socket,send,evaluate,until,go,fill,close,exceptions}
  }catch(error){socket?.close();child.kill();throw error}
}
