import assert from 'node:assert/strict'
import { validateRecovery } from '../src/validation/recoveryValidation.ts'
import { submitMockRecovery } from '../src/services/mockRecoveryService.ts'
assert.ok(validateRecovery('', false).nic)
for (const nic of [' 123456789v ', '123456789X', '123456789012']) assert.deepEqual(validateRecovery(nic, true), { nic: undefined, humanCheck: undefined })
for (const nic of ['123456789z', '123 456789v', '12345678901', 'demo@example.com']) assert.ok(validateRecovery(nic, true).nic)
assert.ok(validateRecovery('123456789v', false).humanCheck)
assert.equal(await submitMockRecovery('success'), 'success')
assert.equal(await submitMockRecovery('error'), 'error')
console.log('PASS: recovery required/trimmed/legacy/modern NIC formats, simulated check, mock outcomes')
