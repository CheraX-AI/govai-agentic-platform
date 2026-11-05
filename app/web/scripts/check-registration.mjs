import assert from 'node:assert/strict'
import { validateRegistration } from '../src/validation/registrationValidation.ts'

const valid = { fullName: 'හර්ෂන් தமிழ் 漢字', nic: ' 123456789x ', phone: '+94 77 123 4567', email: ' demo@example.com ', password: 'abcdef1😀', confirmation: 'abcdef1😀', terms: true }
const check = value => validateRegistration({ ...valid, password: value, confirmation: value }).password
assert.ok(Object.values(validateRegistration(valid)).every(value => value === undefined))
for (const value of ['abcdef1!', 'abcdef1。', 'abcdef1😀', ' abcdef1! ', 'abcdefgh']) assert.equal(check(value), undefined)
for (const value of ['aaaaa1😀', 'abc1!', 'a'.repeat(1025)]) assert.ok(check(value), 'Reject password outside backend length bounds')
assert.ok(validateRegistration({ ...valid, fullName: ' \t ' }).fullName)
for (const [key,value] of [['nic','bad'],['phone','07123'],['email','123456789V'],['email','a b@example.com'],['terms',false],['confirmation','different']]) assert.ok(validateRegistration({...valid,[key]:value})[key])
assert.ok(validateRegistration({...valid,password:' abcdef1!',confirmation:'abcdef1!'}).confirmation)
assert.equal(valid.password,'abcdef1😀')
globalThis.window = { setTimeout }
console.log('PASS: required fields/consent, Unicode names/code-point boundaries, backend password length bounds, exact matching, reused NIC/phone/email rules, no truncation')

const { registerCitizen } = await import('../src/services/registrationApi.ts')
const payload = { full_name: 'Synthetic Test', nic: '123456789V', mobile_number: '+447911123456', email: 'test@example.test', password: 'abcdefgh' }
const originalFetch = globalThis.fetch
try {
  let calls = 0
  globalThis.fetch = async (url, options) => {
    calls++
    assert.equal(String(url), 'https://localhost:5001/api/v1/registrations')
    assert.equal(options.credentials, 'omit')
    assert.equal(options.headers['Idempotency-Key'], 'operation')
    assert.deepEqual(JSON.parse(options.body), payload)
    return new Response(JSON.stringify({status:'received'}), {status:202})
  }
  await registerCitizen(payload, 'operation')
  assert.equal(calls, 1)
  for (const status of [400,422,429,503]) {
    globalThis.fetch = async () => new Response('private upstream detail', {status})
    await assert.rejects(registerCitizen(payload, 'operation'), error => !error.message.includes('private upstream'))
  }
  globalThis.fetch = async () => { throw new Error('private transport detail') }
  await assert.rejects(registerCitizen(payload, 'operation'), /Unable to confirm/)
  await assert.rejects(registerCitizen(payload, 'operation', 'http://localhost:5001'), /configuration/)
} finally { globalThis.fetch = originalFetch }
console.log('PASS: registration API payload, receipt, safe errors, no cookies, no automatic retries, HTTPS boundary')
