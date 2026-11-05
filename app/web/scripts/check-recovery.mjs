import assert from 'node:assert/strict'
import { validateRecovery } from '../src/validation/recoveryValidation.ts'
assert.ok(validateRecovery('').nic)
for (const nic of [' 123456789v ', '123456789X', '123456789012']) assert.deepEqual(validateRecovery(nic), { nic: undefined })
for (const nic of ['123456789z', '123 456789v', '12345678901', 'demo@example.com']) assert.ok(validateRecovery(nic).nic)
console.log('PASS: recovery required/trimmed/legacy/modern NIC formats')
