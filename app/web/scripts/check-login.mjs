import assert from 'node:assert/strict'
import { validateNicEmailLogin } from '../src/validation/loginValidation.ts'
for (const identifier of ['123456789V', '123456789x', '200012345678', ' 123456789v ', ' demo@example.com ', 'demo+tag@example.co.uk']) {
  assert.deepEqual(validateNicEmailLogin(identifier, ' '), { identifier: undefined, password: undefined })
}
for (const identifier of ['12345678V', '123456789A', '123 456789V', '1234567890123', 'a b@example.com', 'a@example .com', 'a@@example.com', 'a@example', 'a@.com']) {
  assert.ok(validateNicEmailLogin(identifier, 'exact password ').identifier, `Reject invalid identifier ${identifier}`)
}
assert.deepEqual(validateNicEmailLogin('  ', ''), { identifier: 'Enter your email or NIC number.', password: 'Enter your password.' })
console.log('PASS: email/NIC formats, invalid identifiers, required fields, password preservation')
