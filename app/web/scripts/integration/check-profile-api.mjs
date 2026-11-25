import assert from 'node:assert/strict'
import { getProfile, saveProfile, getProfilePhoto, uploadProfilePhoto, ProfileError, MAX_PHOTO_BYTES } from '../../src/services/profileApi.ts'

const original = globalThis.fetch
const signal = new AbortController().signal
const fields = { name_si: '', name_ta: '', date_of_birth: '', gender: 'male', address: { line1: 'Synthetic street', line2: '', city: 'Synthetic city' } }
const profile = { ...fields, full_name: 'Synthetic Citizen', nic: '200006100000', mobile_number: '+94770000000', email: null, revision: 3, has_photo: false, photo_url: null }
let calls = []
function stub(respond) {
  calls = []
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), options })
    assert.equal(new URL(url).origin, 'https://localhost:5001')
    assert.equal(options.credentials, 'include')
    assert.equal(options.cache, 'no-store')
    assert.equal(options.redirect, 'error')
    assert.equal(options.headers['X-GovAI-Web'], '1')
    assert.equal(options.headers.Authorization, undefined)
    if (String(url).endsWith('/csrf')) return Response.json({ csrf_token: 'A'.repeat(43) })
    return respond(url, options)
  }
}
try {
  stub(() => Response.json({ ...profile, subject: 'must-not-forward' }))
  assert.deepEqual(await getProfile(signal), profile)
  assert.equal(calls.length, 1)

  stub((url, options) => {
    assert.equal(new URL(url).pathname, '/api/v1/web/profile')
    assert.equal(options.method, 'PUT')
    assert.equal(options.headers['X-CSRF-Token'], 'A'.repeat(43))
    assert.deepEqual(JSON.parse(options.body), { revision: 3, ...fields })
    return Response.json({ ...profile, revision: 4 })
  })
  await saveProfile({ ...fields, full_name: 'FORGED', nic: 'FORGED', subject: 'other', address: { ...fields.address, owner: 'other' } }, 3, signal)
  assert.equal(calls.length, 2)

  const photo = new Blob([new Uint8Array(MAX_PHOTO_BYTES)], { type: 'image/jpeg' })
  stub((url, options) => {
    assert.equal(new URL(url).pathname, '/api/v1/web/profile/photo')
    assert.equal(options.body, photo)
    assert.equal(options.headers['Content-Type'], 'image/jpeg')
    assert.equal(options.headers['X-CSRF-Token'], 'A'.repeat(43))
    assert.equal(options.headers['If-Match'], '"3"')
    return Response.json({ revision: 4, has_photo: true, photo_url: '/api/v1/web/profile/photo' })
  })
  await uploadProfilePhoto(photo, 3, signal)
  assert.equal(calls.length, 2)
  for (const [blob, status] of [[new Blob(['x'], { type: 'image/svg+xml' }), 415], [new Blob([], { type: 'image/png' }), 415], [new Blob([new Uint8Array(MAX_PHOTO_BYTES + 1)], { type: 'image/png' }), 413]]) {
    await assert.rejects(uploadProfilePhoto(blob, 3, signal), error => error.status === status)
  }
  assert.equal(calls.length, 2)

  stub(() => new Response(new Uint8Array([1, 2]), { headers: { 'Content-Type': 'image/png' } }))
  assert.equal((await getProfilePhoto(signal)).type, 'image/png')
  stub(() => new Response('<svg/>', { headers: { 'Content-Type': 'image/svg+xml' } }))
  await assert.rejects(getProfilePhoto(signal), error => error.status === 502)

  for (const status of [401, 403, 404, 409, 413, 415, 422, 428, 429, 503]) {
    stub(() => Response.json({ code: 'revision_conflict', error: 'private provider information' }, { status }))
    await assert.rejects(saveProfile(fields, 3, signal), error => error instanceof ProfileError && error.status === status && !error.message.includes('private'))
    assert.equal(calls.length, 2, 'No automatic retry of a write or conflict')
  }
  for (const bad of [{ ...profile, revision: -1 }, { ...profile, revision: Number.MAX_SAFE_INTEGER + 1 }, { ...profile, photo_url: 'https://other.test/private' }, { ...profile, gender: 'other' }, { ...profile, address: null }]) {
    stub(() => Response.json(bad))
    await assert.rejects(getProfile(signal), error => error.status === 502)
  }
  globalThis.fetch = async () => Response.json({ csrf_token: 'bad' })
  await assert.rejects(saveProfile(fields, 3, signal), error => error.status === 502)
  let attempts = 0
  globalThis.fetch = async () => { attempts++; throw Error('private network detail') }
  await assert.rejects(saveProfile(fields, 3, signal), error => error.status === 503 && !error.message.includes('private'))
  assert.equal(attempts, 1)
  stub(() => { throw Error('unknown outcome after write') })
  await assert.rejects(saveProfile(fields, 3, signal), error => error.status === 503 && error.message.includes('check whether your last change was saved'))
  assert.equal(calls.length, 2, 'An ambiguous write must not be retried')
  const controller = new AbortController()
  controller.abort()
  await assert.rejects(getProfile(controller.signal), error => error.name === 'AbortError')
} finally { globalThis.fetch = original }
console.log('PASS: profile cookie/CSRF transport, editable-field allowlist, photo raw upload and limits, authenticated image fetch, conflicts without retry, safe errors and malformed response rejection. Mock HTTP only; no live backend or UI claim.')
