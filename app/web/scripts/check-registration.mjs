import assert from 'node:assert/strict'
import { validateRegistration } from '../src/validation/registrationValidation.ts'
import { submitMockRegistration } from '../src/services/mockRegistrationService.ts'

const valid = { fullName: 'හර්ෂන් தமிழ் 漢字', nic: ' 123456789x ', phone: '+94 77 123 4567', email: ' demo@example.com ', password: 'abcdef1😀', confirmation: 'abcdef1😀', terms: true }
const check = value => validateRegistration({ ...valid, password: value, confirmation: value }).password
assert.ok(Object.values(validateRegistration(valid)).every(value => value === undefined))
for (const value of ['abcdef1!', 'abcdef1。', 'abcdef1😀', ' abcdef1! ', 'a'.repeat(10000) + '1!']) assert.equal(check(value), undefined)
for (const value of ['aaaaa1😀', 'abcdef1 ', 'abcdef١!', 'abcdef１!', 'abcdefgh', 'abc1!']) assert.ok(check(value), 'Reject password outside approved demo policy')
assert.ok(validateRegistration({ ...valid, fullName: ' \t ' }).fullName)
for (const [key,value] of [['nic','bad'],['phone','07123'],['email','123456789V'],['email','a b@example.com'],['terms',false],['confirmation','different']]) assert.ok(validateRegistration({...valid,[key]:value})[key])
assert.ok(validateRegistration({...valid,password:' abcdef1!',confirmation:'abcdef1!'}).confirmation)
assert.equal(valid.password,'abcdef1😀')
globalThis.window = { setTimeout }
assert.equal(await submitMockRegistration('success'),'success')
assert.equal(await submitMockRegistration('error'),'error')
console.log('PASS: required fields/consent, Unicode names/code-point boundaries, ASCII digit and Unicode punctuation/symbol policy, exact matching, reused NIC/phone/email rules, no truncation, mock outcomes')
