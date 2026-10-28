import { spawn } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'
const delay = (ms) => new Promise(r => setTimeout(r, ms))
const profile = resolve(`node_modules/.tmp/login-browser-profile-${process.pid}-${Date.now()}`)
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


  const demoSuccessAndLogout = async () => {
    assert.equal(await evaluate('location.hash'), '#/home')
    assert.match(await evaluate('document.querySelector(".demo-session-notice").textContent'), /Demo session.*not authenticated/)
    assert.equal(await evaluate('document.querySelector("#password")'), null, 'credentials removed on navigation')
    await evaluate('document.querySelector(".demo-session-notice button").click()');await delay(120)
    assert.equal(await evaluate('location.hash'), '#/login')
  }
  await send('Page.navigate', { url: 'http://127.0.0.1:5173' })
  await delay(1500)
  assert.equal(await evaluate('document.querySelector("h1")?.textContent'), 'Welcome back')
  assert.equal(await evaluate('document.querySelector("footer .demo-options #outcome") !== null && document.querySelector("form #outcome") === null'), true, 'Demo controls live beside the preview footer')
  assert.deepEqual(await evaluate('[...document.querySelectorAll("a")].map(a=>a.getAttribute("href"))'), ['#/forgot-password', '#/register'], 'only implemented recovery and registration navigation is linked')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'), 'phone')
  assert.equal(await evaluate('document.querySelectorAll(".field-error").length'), 3)
  assert.equal(await evaluate('document.querySelector("#phone").getAttribute("aria-describedby")'), 'phone-error')
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 })
  await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 })
  assert.equal(await evaluate('document.activeElement.id'), 'password')
  const fill = async (selector, value) => evaluate(`(() => { const element = document.querySelector(${JSON.stringify(selector)}); Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(element, ${JSON.stringify(value)}); element.dispatchEvent(new Event('input', { bubbles: true })); })()`)
  await fill('#phone', '07123')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.match(await evaluate('document.querySelector("#phone-error").textContent'), /07XXXXXXXX/)
  assert.equal(await evaluate('document.activeElement.id'), 'phone')
  await fill('#phone', '0771234567')
  await fill('#password', 'sample-only')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'), 'human-check')
  await evaluate('document.querySelector(".icon-button").click()')
  assert.equal(await evaluate('document.querySelector("#password").type'), 'text')
  await evaluate('document.querySelector(".icon-button").click()')
  assert.equal(await evaluate('document.querySelector("#password").type'), 'password')
  // Leave visibility enabled through submission to verify that retries conceal it.
  await evaluate('document.querySelector(".icon-button").click()')
  await evaluate('window.mockTimerCount = 0; const originalTimeout = window.setTimeout.bind(window); window.setTimeout = (callback, ms, ...args) => { if (ms === 800) window.mockTimerCount++; return originalTimeout(callback, ms, ...args) }')
  await evaluate('document.querySelector("#human-check").click(); document.querySelector("#password").focus()')
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
  await send('Input.dispatchKeyEvent', { type: 'char', text: '\r', key: 'Enter', windowsVirtualKeyCode: 13 })
  await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
  assert.equal(await evaluate('document.querySelector("button[type=submit]").disabled'), true)
  assert.equal(await evaluate('["#phone", "#password", "#human-check", "#outcome"].every(selector => document.querySelector(selector).disabled)'), true)
  // Programmatic duplicate submit also exercises the synchronous in-flight guard.
  await evaluate('document.querySelector("form").dispatchEvent(new Event("submit", { bubbles:true, cancelable:true }))')
  assert.equal(await evaluate('window.mockTimerCount'), 1)
  assert.match(await evaluate('document.querySelector("[role=status]").textContent'), /Simulating/)
  await delay(1000)
  await demoSuccessAndLogout()
  assert.equal(await evaluate('document.querySelector("#password").value'), '')
  assert.equal(await evaluate('document.querySelector("#password").type'), 'password')
  assert.equal(await evaluate('document.querySelector(".icon-button").getAttribute("aria-pressed")'), 'false')
  await fill('#phone','0771234567');await evaluate('document.querySelector("#human-check").click()')
  await evaluate('const outcome = document.querySelector("#outcome"); outcome.value="error"; outcome.dispatchEvent(new Event("change", {bubbles:true}))')
  await fill('#password', 'sample-only')
  await evaluate('document.querySelector("button[type=submit]").click()')
  await delay(1000)
  assert.match(await evaluate('document.querySelector("[role=alert]").textContent'), /Demo login failed/)
  assert.equal(await evaluate('document.querySelector("#password").value'), '', 'Failure clears password too')
  await evaluate('document.querySelector("#outcome").value="success"; document.querySelector("#outcome").dispatchEvent(new Event("change", {bubbles:true}))')
  await fill('#phone', '+94 77 123 4567')
  await fill('#password', 'sample-only')
  await evaluate('document.querySelector("button[type=submit]").click()')
  await delay(1000)
  await demoSuccessAndLogout()
  assert.deepEqual(apiRequests, [], 'Mock login sends no Fetch/XHR requests')
  for (const width of [320, 375, 768, 900, 901, 1024, 1440]) {
    await send('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 1, mobile: false })
    await delay(100)
    assert.equal(await evaluate('document.documentElement.scrollWidth <= innerWidth'), true, `No horizontal overflow at ${width}px`)
    assert.equal(await evaluate('getComputedStyle(document.querySelector(".photo-panel")).display === "none"'), width <= 900)
  }
  assert.equal(await evaluate('Array.from(document.images).every(image => image.complete && image.naturalWidth > 0)'), true)
  assert.equal(await evaluate('localStorage.length + sessionStorage.length'), 0)
  assert.deepEqual((await send('Network.getCookies')).cookies, [], 'No session cookies created')
  assert.deepEqual(exceptions, [])
  // NIC/email uses the same mock flow; switching must discard every prior form state.
  const switchMethod = async index => evaluate(`document.querySelectorAll('.login-method button')[${index}].click()`)
  const assertCleared = async () => {
    assert.equal(await evaluate('document.querySelector(".input-shell input").value'), '')
    assert.equal(await evaluate('document.querySelector("#password").value'), '')
    assert.equal(await evaluate('document.querySelector("#password").type'), 'password')
    assert.equal(await evaluate('document.querySelector("#human-check").checked'), false)
    assert.equal(await evaluate('document.querySelectorAll(".field-error, .status-message").length'), 0)
  }
  await switchMethod(1)
  await assertCleared()
  assert.equal(await evaluate('document.querySelectorAll(".login-method button")[1].getAttribute("aria-pressed")'), 'true')
  assert.equal(await evaluate('document.querySelector("#identifier").autocomplete'), 'username')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.equal(await evaluate('document.activeElement.id'), 'identifier')
  assert.equal(await evaluate('document.querySelector("#identifier").getAttribute("aria-describedby")'), 'identifier-error')
  assert.equal(await evaluate('document.querySelectorAll(".field-error").length'), 3)
  await fill('#identifier', 'demo @example.com')
  await evaluate('document.querySelector("button[type=submit]").click()')
  assert.match(await evaluate('document.querySelector("#identifier-error").textContent'), /valid email/)
  await switchMethod(0)
  await assertCleared()
  // Activate method switching through the keyboard, not just DOM clicks.
  await evaluate('document.querySelectorAll(".login-method button")[1].focus()')
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, text: '\r' })
  await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
  assert.ok(await evaluate('document.querySelector("#identifier") !== null'))
  for (const identifier of [' 123456789v ', '123456789X', ' 200012345678 ', ' Demo+tag@example.com ']) {
    await fill('#identifier', identifier)
    await fill('#password', ' exact demo password ')
    assert.equal(await evaluate('document.querySelector("#password").value'), ' exact demo password ', 'password unchanged')
    await evaluate('if (!document.querySelector("#human-check").checked) document.querySelector("#human-check").click()')
    await evaluate('document.querySelector("button[type=submit]").click()')
    assert.equal(await evaluate('document.querySelector("#identifier").value'), identifier.trim())
    assert.equal(await evaluate('[...document.querySelectorAll(".login-method button")].every(b=>b.disabled)'), true, 'switching disabled while pending')
    await switchMethod(0)
    assert.ok(await evaluate('document.querySelector("#identifier") !== null'), 'pending method cannot change')
    await evaluate('document.querySelector("form").dispatchEvent(new Event("submit", {bubbles:true,cancelable:true}))')
    await delay(1000)
    await demoSuccessAndLogout()
    assert.equal(await evaluate('document.querySelector("#password").value'), '')
    await switchMethod(1)
  }
  await fill('#identifier','demo@example.com');await evaluate('document.querySelector("#human-check").click()')
  await evaluate('document.querySelector("#outcome").value="error"; document.querySelector("#outcome").dispatchEvent(new Event("change", {bubbles:true}))')
  await fill('#password', ' ')
  await evaluate('document.querySelector(".icon-button").click(); document.querySelector("button[type=submit]").click()')
  await delay(1000)
  assert.match(await evaluate('document.querySelector("[role=alert]").textContent'), /Demo login failed/)
  assert.equal(await evaluate('document.querySelector("#password").type'), 'password')
  assert.equal(await evaluate('document.querySelector("#password").value'), '')
  for (const [width,height] of [[1440,820],[1024,768],[900,900],[768,900],[375,667],[320,568]]) {
    await send('Emulation.setDeviceMetricsOverride', {width,height,deviceScaleFactor:1,mobile:false})
    await evaluate('document.querySelector("button[type=submit]").click()')
    assert.ok(await evaluate('document.documentElement.scrollWidth <= innerWidth'), `NIC/email no overflow ${width}px`)
    await evaluate('document.querySelector("footer").scrollIntoView({block:"end"})')
    assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom<=innerHeight+1'), 'NIC/email footer reachable')
  }
  await switchMethod(0)
  await assertCleared()
  await fill('#phone','0771234567')
  await fill('#password','sample-only')
  await evaluate('document.querySelector(".icon-button").click()')
  await switchMethod(1)
  await assertCleared()
  assert.ok(await evaluate('[...document.images].every(i=>i.complete && i.naturalWidth>0)'), 'new method/mail icons load')
  assert.deepEqual(apiRequests, [], 'both methods send no credentials or API requests')
  assert.equal(await evaluate('localStorage.length + sessionStorage.length'),0)
  assert.deepEqual((await send('Network.getCookies')).cookies, [])
  assert.deepEqual(exceptions, [])
  for (const [width,height,label] of [[1440,900,'desktop'],[375,900,'narrow']]) {
    await send('Emulation.setDeviceMetricsOverride', {width,height,deviceScaleFactor:1,mobile:false})
    await evaluate('scrollTo(0,0)')
    for (const name of ['mobile-tab-inactive','nic-email-tab-active','mail']) {
      const geometry = await evaluate(`(() => { const i = document.querySelector('img[src$="/${name}.svg"]'); const r = i.getBoundingClientRect(); return {width:r.width,height:r.height,loaded:i.complete&&i.naturalWidth>0} })()`)
      assert.deepEqual(geometry,{width:18,height:18,loaded:true},`${name} exact design slot`)
    }
    const shot = await send('Page.captureScreenshot',{format:'png'})
    await writeFile(`node_modules/.tmp/nic-email-${label}.png`,Buffer.from(shot.data,'base64'))
  }
  await switchMethod(0)
  console.log('PASS: NIC/email formats, trimming, exact passwords, mock success/failure, disabled switching, keyboard switching, full method reset, six responsive cases and no API/storage/cookies')
  // Measure both axes with real error messages and the demo controls expanded.
  const measurements = []
  for (const [width, height] of [[1366, 768], [1536, 864], [1920, 1080], ...[320, 375, 768, 900, 901, 1024, 1440].map(width => [width, 900]), [320, 568], [375, 667]]) {
    await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
    await send('Page.reload')
    await delay(400)
    for (const state of ['initial', 'expanded', 'errors', 'errors+expanded']) {
      if (state === 'expanded') {
        await evaluate('document.querySelector("details summary").click()')
        assert.equal(await evaluate('document.querySelector("details").open'), true)
      }
      if (state === 'errors') {
        await evaluate('document.querySelector("details summary").click()')
        await evaluate('document.querySelector("button[type=submit]").click()')
        await delay(50)
        assert.equal(await evaluate('document.querySelectorAll(".field-error").length'), 3)
      }
      if (state === 'errors+expanded') {
        await evaluate('document.querySelector("details summary").click()')
        assert.equal(await evaluate('document.querySelector("details").open'), true, 'Disclosure expands on activation')
      }
      const metrics = await evaluate(`(() => {
        const root = document.documentElement;
        const card = document.querySelector('.login-card').getBoundingClientRect();
        return { horizontal: root.scrollWidth - root.clientWidth, vertical: root.scrollHeight - innerHeight,
          card: [Math.round(card.width), Math.round(card.height)],
          inputHeight: document.querySelector('.input-shell').getBoundingClientRect().height,
          buttonHeight: document.querySelector('button[type=submit]').getBoundingClientRect().height,
          footerWrapped: document.querySelector('footer .demo-options').getBoundingClientRect().top >= document.querySelector('footer > span').getBoundingClientRect().bottom,
          photo: getComputedStyle(document.querySelector('.photo-panel')).display !== 'none' };
      })()`)
      assert.equal(metrics.horizontal, 0, `No horizontal overflow: ${width}x${height} ${state}`)
      assert.ok(metrics.inputHeight >= 46 && metrics.buttonHeight >= 50, 'Control sizes preserved')
      assert.equal(metrics.photo, width > 900)
      await evaluate('document.querySelector("footer").scrollIntoView({ block: "end" })')
      assert.ok(await evaluate('document.querySelector("footer").getBoundingClientRect().bottom <= innerHeight + 1'), 'Footer reachable by scrolling')
      measurements.push({ viewport: `${width}x${height}`, state, ...metrics })
    }
  }
  console.table(measurements)
  await writeFile('node_modules/.tmp/login-layout-measurements.json', JSON.stringify(measurements, null, 2))
  await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false })
  const screenshot = await send('Page.captureScreenshot', { format: 'png' })
  await writeFile('node_modules/.tmp/login-desktop.png', Buffer.from(screenshot.data, 'base64'))
  console.log('PASS: rendering, no dead links, required/malformed phone errors and associations, first-invalid focus including human check, Tab navigation, Enter submission, password toggle/clearing, pending controls, duplicate submission guard, mock success/error/retry with international phone, no Fetch/XHR, 320/375/768/900/901/1024/1440px overflow and photo visibility, image loading, empty browser storage, no runtime exceptions')
  await send('Browser.close')
} finally {
  for (const call of pending.values()) clearTimeout(call.timer)
  socket?.close()
  browser.kill()
}
