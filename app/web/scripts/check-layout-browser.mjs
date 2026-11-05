import { installAuthFixtures } from './integration/authFixtures.mjs'
import { spawn } from 'node:child_process'
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/layout-browser-profile-${process.pid}`)
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
  const fixture = await installAuthFixtures(socket, send)
  await send('DOM.enable')
  await send('CSS.enable')
  const key = async (name, modifiers = 0) => {
    const codes = { Tab: 9, Enter: 13, Escape: 27, ' ': 32 }
    const params = { key: name, code: name === ' ' ? 'Space' : name, windowsVirtualKeyCode: codes[name], modifiers }
    await send('Input.dispatchKeyEvent', { type: 'keyDown', ...params, ...(name === 'Enter' || name === ' ' ? { text: name === 'Enter' ? '\r' : ' ' } : {}) })
    await send('Input.dispatchKeyEvent', { type: 'keyUp', ...params })
  }
  const assetSizes = {}
  for (const name of await readdir('public/assets/figma/shell')) {
    const bytes = await readFile(`public/assets/figma/shell/${name}`)
    assert.ok(bytes.length > 0, `${name} nonempty`)
    if (name.endsWith('.svg')) {
      const root = bytes.toString().match(/<svg\b[^>]+>/)?.[0]
      assert.ok(root, `${name} valid SVG root`)
      assetSizes[name] = ['width', 'height'].map(axis => Number(root.match(new RegExp(`${axis}="([0-9.]+)"`))?.[1]))
    } else {
      assert.equal(bytes.subarray(1, 4).toString(), 'PNG', 'emblem PNG signature')
      assetSizes[name] = [57, 48]
    }
  }
  const seenAssets = new Set()

  const base = 'https://localhost:5173';
  await send('Page.navigate', {url: base + '/#/preview/home'});
  await delay(1000);
  const measurements = [];
  for (const page of ['home', 'recent']) {
    await evaluate('location.hash = ' + JSON.stringify('#/' + page));
    await delay(150);
    assert.equal(await evaluate('document.title'), page === 'home' ? 'GovAI | Home layout preview' : 'GovAI | Recent chats layout preview', 'page title identifies current layout');
    await evaluate('document.fonts.ready');
    for (const [width,height] of [[1440,820],[1920,1080],[1024,768],[900,900],[768,900],[375,667],[320,568],[720,410]]) {
      await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false});
      await delay(100);
      const metrics = await evaluate(`(() => {const header=document.querySelector('.application-header').getBoundingClientRect();const sidebar=document.querySelector('.desktop-sidebar').getBoundingClientRect();return {horizontal:document.documentElement.scrollWidth-innerWidth,header:header.height,sidebar:sidebar.width,accountRight:document.querySelector('.header-account').getBoundingClientRect().right, images:[...document.images].every(i=>i.complete&&i.naturalWidth>0), fonts:['GovAI Inter','GovAI Montserrat'].every(f=>document.fonts.check('700 16px "'+f+'"')),selected:document.querySelector('.desktop-sidebar [aria-current=page]')?.textContent}})()`);
      assert.equal(metrics.horizontal,0, page+' '+width+' overflow');
      assert.ok(metrics.images,'all image assets loaded');
      assert.ok(metrics.fonts,'local font families loaded');
      const assets = await evaluate(`([...document.querySelectorAll('img[src*="/figma/shell/"]')].filter(i => i.getBoundingClientRect().width > 0).map(i => ({name:i.src.split('/').pop(),width:i.getBoundingClientRect().width,height:i.getBoundingClientRect().height})))`)
      for (const asset of assets) {
        assert.deepEqual([asset.width, asset.height], assetSizes[asset.name], `${asset.name} exact export geometry at ${width}px`)
        seenAssets.add(asset.name)
      }
      assert.equal(await evaluate('document.querySelector(".application-content").scrollWidth > document.querySelector(".application-content").clientWidth'), false, 'no clipped horizontal content');
      assert.equal(metrics.selected, page === 'home' ? 'New chat' : 'Recent chats');
      const active = await evaluate(`(() => {const a=document.querySelector('.desktop-sidebar [aria-current=page]'); const s=getComputedStyle(a);return {background:s.backgroundColor,color:s.color,radius:s.borderRadius,weight:s.fontWeight}})()`)
      assert.deepEqual(active, page === 'home' ? {background:'rgb(245, 245, 245)',color:'rgb(20, 32, 51)',radius:'12px',weight:'600'} : {background:'rgb(252, 243, 245)',color:'rgb(163, 15, 45)',radius:'12px',weight:'600'});
      await evaluate(`(() => {const c=document.querySelector('.application-content');c.scrollTop=c.scrollHeight})()`)
      assert.ok(await evaluate(`(() => {const c=document.querySelector('.application-content').getBoundingClientRect();const last=document.querySelector('.application-content').lastElementChild.getBoundingClientRect();return last.bottom<=c.bottom+1&&last.top>=c.top})()`), 'last content reachable');
      await evaluate('document.querySelector(".application-content").scrollTop=0');
      if(width>900){assert.equal(metrics.header,76);assert.equal(metrics.sidebar,264);assert.equal(metrics.accountRight,width-32)}
      if(page==='recent') assert.equal(metrics.selected,'Recent chats');
      measurements.push({page,width,height,...metrics});
      if(width===1440){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/ga51-'+page+'.png',Buffer.from(shot.data,'base64'))}
      if(width===375){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/ga51-'+page+'-narrow.png',Buffer.from(shot.data,'base64'))}
    }
  }
  assert.deepEqual([...seenAssets].sort(), Object.keys(assetSizes).sort(), 'every downloaded shell asset rendered and verified');
  await send('Emulation.setDeviceMetricsOverride',{width:1440,height:820,deviceScaleFactor:1,mobile:false});
  await evaluate('location.hash = "#/preview/home"'); await delay(100);
  const documentNode = (await send('DOM.getDocument')).root.nodeId;
  for (const [selector, expected] of [['.home-hero h1','Inter'],['.header-brand .wordmark','Montserrat'],['.synthetic-user strong','Inter']]) {
    const {nodeId} = await send('DOM.querySelector',{nodeId:documentNode,selector});
    const {fonts} = await send('CSS.getPlatformFontsForNode',{nodeId});
    assert.ok(fonts.some(f=>f.isCustomFont&&f.familyName.includes(expected)&&f.glyphCount>0), `${expected} actually renders ${selector}`);
  }
  await send('Page.navigate', {url: base + '/#/preview/home'}); await delay(300);
  await key('Tab');
  assert.equal(await evaluate('document.activeElement.className'),'skip-link','first keyboard stop is skip link');
  assert.ok(await evaluate('getComputedStyle(document.activeElement).outlineStyle !== "none"'), 'visible keyboard focus');
  await key('Enter');
  assert.equal(await evaluate('document.activeElement.id'),'application-content','skip link focuses content without changing route');
  await evaluate(`[...document.querySelectorAll('.desktop-sidebar a')].find(a=>a.hash==='#/preview/recent').focus()`);
  await key('Enter'); await delay(100);
  assert.equal(await evaluate('location.hash'),'#/preview/recent','desktop keyboard navigation');
  await send('Emulation.setDeviceMetricsOverride',{width:375,height:667,deviceScaleFactor:1,mobile:false});
  await delay(100);
  await evaluate('document.querySelector(".menu-toggle").focus()');
  assert.equal(await evaluate('document.activeElement.className'),'menu-toggle','visible mobile menu receives keyboard focus');
  await key('Enter');
  await delay(50);
  assert.ok(await evaluate('document.querySelector("dialog").open'));
  assert.equal(await evaluate('document.activeElement.textContent'),'Close menu');
  await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Tab',code:'Tab',windowsVirtualKeyCode:9,modifiers:8});
  await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Tab',code:'Tab',windowsVirtualKeyCode:9,modifiers:8});
  assert.ok(await evaluate('document.querySelector("dialog").contains(document.activeElement)'),'focus trapped on Shift+Tab');
  assert.equal(await evaluate('document.activeElement.hash'),'#/login', 'reverse Tab wraps to last menu link: ' + await evaluate('document.activeElement.outerHTML.slice(0,350)'));
  await key('Tab');
  assert.equal(await evaluate('document.activeElement.textContent'),'Close menu','forward Tab wraps to first menu control');
  for (let i=0;i<8;i++) { await key('Tab'); assert.ok(await evaluate('document.querySelector("dialog").contains(document.activeElement)'), 'Tab stays inside modal'); }
  assert.equal(await evaluate('document.querySelector("dialog").scrollWidth > document.querySelector("dialog").clientWidth'),false,'menu has no horizontal overflow');
  await evaluate('document.querySelector("dialog .login-preview-link").scrollIntoView()');
  assert.ok(await evaluate('document.querySelector("dialog .login-preview-link").getBoundingClientRect().bottom<=innerHeight'), 'last menu link reachable');
  await send('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
  await send('Input.dispatchKeyEvent',{type:'keyUp',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
  assert.equal(await evaluate('document.querySelector("dialog").open'),false);
  assert.equal(await evaluate('document.activeElement.className'),'menu-toggle');
  await key(' ');
  assert.ok(await evaluate('document.querySelector("dialog").open'),'Space opens menu');
  await evaluate(`[...document.querySelectorAll('dialog a')].find(a=>a.hash==='#/preview/home').focus()`);
  await key('Enter');
  await delay(150);
  assert.equal(await evaluate('location.hash'),'#/preview/home');
  assert.equal(await evaluate('document.querySelector("dialog").open'),false);
  assert.equal(await evaluate('document.activeElement.className'),'menu-toggle','focus returns after menu selection');
  await evaluate('document.querySelector(".menu-toggle").click()');
  await evaluate('document.querySelector(".drawer-heading button").click()');
  assert.equal(await evaluate('document.querySelector("dialog").open'),false,'Close button dismisses');
  assert.equal(await evaluate('document.activeElement.className'),'menu-toggle','Close button returns focus');
  for (const [width,height] of [[320,568],[768,900],[900,900]]) {
    await send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:false}); await delay(100);
    await evaluate('document.querySelector(".menu-toggle").focus()'); await key('Enter');
    assert.equal(await evaluate('document.querySelector("dialog").open'),true,`${width}px menu opens`);
    assert.equal(await evaluate('document.querySelector("dialog").scrollWidth > document.querySelector("dialog").clientWidth'),false,`${width}px menu overflow`);
    await key('Tab',8);
    assert.equal(await evaluate('document.activeElement.hash'),'#/login',`${width}px reverse focus wrap`);
    if(width===320){const shot=await send('Page.captureScreenshot',{format:'png'});await writeFile('node_modules/.tmp/ga51-menu-narrow.png',Buffer.from(shot.data,'base64'))}
    await key('Escape');
    assert.equal(await evaluate('document.querySelector("dialog").open'),false,`${width}px Escape dismisses`);
    assert.equal(await evaluate('document.activeElement.className'),'menu-toggle',`${width}px focus returns`);
  }
  await evaluate('document.querySelector(".menu-toggle").click()');
  await send('Emulation.setDeviceMetricsOverride',{width:1440,height:820,deviceScaleFactor:1,mobile:false});
  await delay(100);
  assert.equal(await evaluate('document.querySelector("dialog").open'),false,'resize closes modal');
  assert.equal(await evaluate('document.activeElement.id'),'application-content','resize focuses visible content');
  await evaluate(`[...document.querySelectorAll('.desktop-sidebar a')].find(a=>a.hash==='#/preview/recent').click()`);
  await delay(100);
  assert.equal(await evaluate('location.hash'),'#/preview/recent');
  await evaluate('history.back()');await delay(200);
  assert.equal(await evaluate('location.hash'),'#/preview/home');
  await evaluate('history.forward()');await delay(200);
  assert.equal(await evaluate('location.hash'),'#/preview/recent');
  await evaluate('location.hash = "#/missing"');await delay(100);
  assert.equal(await evaluate('document.querySelector("h1").textContent'),'Preview unavailable');
  assert.equal(await evaluate('document.title'),'GovAI | Preview unavailable');
  await evaluate('location.hash = "#/login"');await delay(100);
  assert.equal(await evaluate('document.querySelector("h1").textContent'),'Welcome back');
  assert.equal(await evaluate('document.title'),'GovAI | Login preview');
  assert.ok(apiRequests.every(url=>/^https:\/\/localhost:5001\/api\/v1\/web\/(session|csrf|refresh)$/.test(url)),'only expected authentication reads');assert.deepEqual(fixture.errors,[]);
  assert.equal(exceptions.length,0,'no runtime exceptions');
  assert.deepEqual(await evaluate('({local:localStorage.length,session:sessionStorage.length,cookies:document.cookie})'),{local:0,session:0,cookies:''},'no session persistence');
  await writeFile('node_modules/.tmp/ga51-layout-measurements.json',JSON.stringify(measurements,null,2));
  console.log('PASS: 16 layout cases; desktop header/sidebar/right-aligned accounts and active colours; all 21 shell assets at exact exported dimensions; Inter/Montserrat actually render; keyboard skip link and navigation; 320/375/768/900px website menu, Tab/Shift+Tab containment, Escape/Close/selection focus return and resize; history/direct links/unknown preview; synthetic auth fixtures, no runtime errors or stored sessions');
  await send('Browser.close');
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
