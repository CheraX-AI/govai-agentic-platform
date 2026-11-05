import assert from 'node:assert/strict'
import { allowedReturn, readRoute, loginDestination } from '../src/routing/routes.ts'
for (const path of ['https://example.com', '#/app/home', '//example.com', 'javascript:alert(1)', '/app/home/extra', undefined]) assert.equal(allowedReturn(path), '/app/home')
assert.equal(allowedReturn('/app/recent'), '/app/recent')
assert.equal(readRoute('/login', '?returnTo=%2Fapp%2Frecent').returnTo, '/app/recent')
assert.equal(readRoute('/', '').path, '/login')
assert.equal(readRoute('/register/', '').path, '/register')
assert.equal(readRoute('/login', '?reason=expired').expired, true)
assert.equal(loginDestination('/app/recent', true), '/login?returnTo=%2Fapp%2Frecent&reason=expired')
console.log('PASS internal return allowlist, root opens login, trailing slash, expired login link')
