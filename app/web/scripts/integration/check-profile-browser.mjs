import assert from 'node:assert/strict'
import { writeFile, mkdir } from 'node:fs/promises'
import { openBrowser, origin, frontendFixturePatterns } from './browserHarness.mjs'

const b = await openBrowser()
const { send, evaluate, until, fill, go } = b
let profile = { full_name: 'Synthetic Citizen', nic: '200322812345', mobile_number: '+94771234567', email: 'citizen@example.test', name_si: '', name_ta: '', date_of_birth: '2003-08-15', gender: 'male', address: { line1: '42 Temple Road', line2: '', city: 'Colombo' }, revision: 0, has_photo: false, photo_url: null }
let writeStatus = 200, readStatus = 503, session = true
let photoWrites = 0
const writes = [], fixtureErrors = []
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII='
b.socket.addEventListener('message', ({ data }) => {
  const event = JSON.parse(data)
  if (event.method !== 'Fetch.requestPaused' || new URL(event.params.request.url).origin !== 'https://localhost:5001') return
  void (async () => {
    const { requestId, request } = event.params
    const path = new URL(request.url).pathname
    const headers = Object.fromEntries(Object.entries(request.headers).map(([key, value]) => [key.toLowerCase(), value]))
    const responseHeaders = [{ name: 'Access-Control-Allow-Origin', value: origin }, { name: 'Access-Control-Allow-Credentials', value: 'true' }, { name: 'Access-Control-Allow-Headers', value: 'X-GovAI-Web, X-CSRF-Token, Content-Type, If-Match' }, { name: 'Access-Control-Allow-Methods', value: 'GET, PUT, POST, OPTIONS' }, { name: 'Content-Type', value: 'application/json' }, { name: 'Cache-Control', value: 'no-store' }]
    let status = 200, body = {}
    if (request.method === 'OPTIONS') status = 204
    else {
      assert.equal(headers['x-govai-web'], '1')
      if (path.endsWith('/session')) { status = session ? 200 : 401; body = { subject: 'synthetic-owner' } }
      else if (path.endsWith('/csrf')) body = { csrf_token: 'A'.repeat(43) }
      else if (path.endsWith('/refresh')) status = session ? 204 : 401
      else if (path.endsWith('/logout')) { session = false; status = 204 }
      else if (path.endsWith('/profile/photo')) {
        if (request.method === 'PUT') {
          photoWrites++
          assert.equal(headers['x-csrf-token'], 'A'.repeat(43))
          assert.equal(headers['if-match'], `"${profile.revision}"`)
          profile = { ...profile, revision: profile.revision + 1, has_photo: true, photo_url: '/api/v1/web/profile/photo' }
          body = { revision: profile.revision, has_photo: true, photo_url: profile.photo_url }
        } else {
          responseHeaders[4].value = 'image/png'
          await send('Fetch.fulfillRequest', { requestId, responseCode: 200, responseHeaders, body: png })
          return
        }
      } else if (path.endsWith('/profile')) {
        if (request.method === 'PUT') {
          assert.equal(headers['x-csrf-token'], 'A'.repeat(43))
          const payload = JSON.parse(request.postData)
          assert.deepEqual(Object.keys(payload).sort(), ['address', 'date_of_birth', 'gender', 'name_si', 'name_ta', 'revision'])
          writes.push(payload)
          status = writeStatus
          if (status === 200) { assert.equal(payload.revision, profile.revision); profile = { ...profile, ...payload, revision: profile.revision + 1 }; body = profile }
          else body = { code: status === 409 ? 'revision_conflict' : 'dob_nic_mismatch', error: 'private upstream diagnostic' }
        } else { status = readStatus; body = status === 200 ? profile : { code: 'profile_unavailable', error: 'private upstream diagnostic' } }
      } else { status = 404; body = { code: 'not_found' } }
    }
    await send('Fetch.fulfillRequest', { requestId, responseCode: status, responseHeaders, body: status === 204 ? '' : Buffer.from(JSON.stringify(body)).toString('base64') })
  })().catch(error => {
    // Reload/unmount can cancel an intercepted request before fulfillment.
    if (error.message !== 'Invalid InterceptionId.') fixtureErrors.push(error.message)
  })
})
try {
  await send('Fetch.enable', { patterns: [...frontendFixturePatterns, { urlPattern: 'https://localhost:5001/*' }] })
  await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false })
  await go('/app/profile')
  await until('Boolean(document.querySelector(".profile-loading [role=alert]"))', 'initial load failure')
  assert.equal(await evaluate('document.body.innerText.includes("private upstream")'), false)
  assert.equal(await evaluate('Boolean(document.getElementById("profile-city"))'), false)
  readStatus = 200
  await evaluate('[...document.querySelectorAll("button")].find(x=>x.textContent==="Try again").click()')
  await until('Boolean(document.getElementById("profile-city"))', 'profile form')
  await evaluate('document.fonts.ready')
  assert.equal(await evaluate('document.getElementById("profile-full-name").readOnly && document.getElementById("profile-nic").readOnly && document.getElementById("profile-mobile").readOnly && document.getElementById("profile-email").readOnly'), true)
  assert.equal(await evaluate('document.querySelector(".profile-save").disabled'), true)
  await mkdir('node_modules/.tmp/profile-evidence', { recursive: true })
  const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false })
  await writeFile('node_modules/.tmp/profile-evidence/desktop.png', Buffer.from(shot.data, 'base64'))
  await fill('profile-city', 'Kandy')
  await evaluate('document.querySelector("form").requestSubmit();document.querySelector("form").requestSubmit()')
  await until('document.body.innerText.includes("personal information has been saved")', 'saved details')
  assert.equal(writes.length, 1)
  assert.equal(profile.address.city, 'Kandy')
  await send('Page.reload')
  await until('document.getElementById("profile-city")?.value === "Kandy"', 'persisted fixture details')
  writeStatus = 409
  profile = { ...profile, revision: profile.revision + 1 }
  await fill('profile-city', 'Galle')
  await evaluate('document.querySelector("form").requestSubmit()')
  await until('Boolean(document.querySelector(".profile-reconcile"))', 'revision conflict')
  assert.equal(await evaluate('document.getElementById("profile-city").value'), 'Galle')
  assert.equal(await evaluate('document.querySelector(".profile-save").disabled'), true)
  writeStatus = 200
  await evaluate('[...document.querySelectorAll("button")].find(x=>x.textContent==="Load latest and keep my edits").click()')
  await until('!document.querySelector(".profile-reconcile")', 'explicit reconcile')
  assert.equal(await evaluate('document.getElementById("profile-city").value'), 'Galle')
  await evaluate('document.querySelector("form").requestSubmit()')
  await until('document.body.innerText.includes("personal information has been saved")', 'save reconciled edits')
  writeStatus = 422
  await fill('profile-city', 'Matara')
  await evaluate('document.querySelector("form").requestSubmit()')
  await until('document.body.innerText.includes("date of birth must match")', 'validation error')
  assert.equal(await evaluate('document.getElementById("profile-city").value'), 'Matara')
  assert.equal(await evaluate('document.body.innerText.includes("private upstream")'), false)
  writeStatus = 503
  await evaluate('document.querySelector("form").requestSubmit()')
  await until('Boolean(document.querySelector(".profile-reconcile"))', 'uncertain save')
  await evaluate('[...document.querySelectorAll("button")].find(x=>x.textContent==="Discard edits and load latest").click()')
  await until('document.getElementById("profile-city").value === "Galle" && !document.querySelector(".profile-reconcile")', 'discard and reload')
  writeStatus = 200
  for (const [type, size, message] of [['text/plain', 8, 'valid JPEG or PNG'], ['image/png', 2 * 1024 * 1024 + 1, 'up to 2 MiB']]) {
    await evaluate(`(()=>{const f=new File([new Uint8Array(${size})],'invalid',{type:${JSON.stringify(type)}});const d=new DataTransfer();d.items.add(f);const i=document.querySelector('input[type=file]');i.files=d.files;i.dispatchEvent(new Event('change',{bubbles:true}))})()`)
    await until(`document.querySelector('.profile-photo-error')?.textContent.includes(${JSON.stringify(message)})`, 'invalid photo rejected')
    assert.equal(photoWrites, 0, 'invalid selection must not send an upload')
  }
  await evaluate(`(()=>{const f=new File([Uint8Array.from(atob('${png}'),c=>c.charCodeAt(0))],'synthetic.png',{type:'image/png'});const d=new DataTransfer();d.items.add(f);const i=document.querySelector('input[type=file]');i.files=d.files;i.dispatchEvent(new Event('change',{bubbles:true}))})()`)
  await until('document.body.innerText.includes("Photo selected")', 'photo preview')
  await evaluate('[...document.querySelectorAll("button")].find(x=>x.textContent==="Upload photo").click()')
  await until('document.body.innerText.includes("profile photo has been saved")', 'photo upload')
  await until('Boolean(document.querySelector(".profile-avatar img")?.naturalWidth)', 'authenticated photo displayed')
  assert.equal(photoWrites, 1)
  for (const [width, height] of [[1440, 1100], [1024, 900], [768, 900], [390, 844]]) {
    await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
    assert.equal(await evaluate('document.documentElement.scrollWidth <= innerWidth'), true, `no page overflow at ${width}`)
    assert.equal(await evaluate('document.querySelector(".profile-card").scrollWidth <= document.querySelector(".profile-card").clientWidth'), true)
  }
  assert.equal(await evaluate('localStorage.length + sessionStorage.length'), 0)
  assert.deepEqual(fixtureErrors, [])
  assert.equal(b.exceptions.length, 0)
  session = false
  await send('Page.reload')
  await until('location.pathname === "/login"', 'expired session leaves profile')
  console.log('PASS: initial load failure/retry, desktop profile render, read-only fields, save/duplicate guard, fixture persistence, 409 reconcile, 422/503 errors, invalid photo type/size, photo upload/display, responsive overflow, no browser storage and expired-session protection. Synthetic HTTP fixtures; no live integration claim.')
} catch (error) {
  console.error('Synthetic browser diagnostic:', JSON.stringify({ fixtureErrors, exceptions: b.exceptions }))
  throw error
} finally { b.close() }
