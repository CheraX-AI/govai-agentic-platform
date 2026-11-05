import assert from 'node:assert/strict'
import { validateDemoPasswords, validateRegistration } from '../src/validation/registrationValidation.ts'
for(const password of ['abcdef1!','abcdef1'+String.fromCodePoint(0x1f600),' abcdef1! '])assert.deepEqual(validateDemoPasswords(password,password),{password:undefined,confirmation:undefined})
for(const password of ['','abcdef1 ','aaaaa1'+String.fromCodePoint(0x1f600),'abcdef'+String.fromCodePoint(0x0661)+'!'])assert.ok(validateDemoPasswords(password,password).password)
assert.ok(validateDemoPasswords('abcdef1!','different').confirmation);assert.ok(validateDemoPasswords(' abcdef1!','abcdef1!').confirmation)
const values={fullName:'Demo',nic:'123456789v',phone:'0771234567',email:'demo@example.com',terms:true,password:'abcdef1!',confirmation:'different'};assert.equal(validateRegistration(values).confirmation,validateDemoPasswords(values.password,values.confirmation).confirmation)
console.log('PASS shared registration/reset Unicode policy, exact confirmation/whitespace, required fields')
