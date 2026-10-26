import assert from 'node:assert/strict'
import { normalizePhone, validateLogin } from '../src/validation/loginValidation.ts'
import { submitMockLogin } from '../src/services/mockLoginService.ts'
assert.equal(normalizePhone('077 123-4567'), '+94771234567')
assert.equal(normalizePhone('+94 77 123 4567'), '+94771234567')
for (const value of ['', '07123', '+44771234567', '077abcdefg', '07712345678']) assert.equal(normalizePhone(value), undefined)
assert.deepEqual(validateLogin('', '', false), { phone: 'Enter your mobile number.', password: 'Enter your password.', humanCheck: 'Complete the simulated human check.' })
assert.deepEqual(validateLogin('0771234567', ' ', true), { phone: undefined, password: undefined, humanCheck: undefined })
globalThis.window = { setTimeout }
assert.equal(await submitMockLogin('success'), 'success')
assert.equal(await submitMockLogin('error'), 'error')
console.log('PASS: local/international phone normalisation, invalid formats, required fields, password preservation, simulated check, mock success/error')
