import { runAuthBrowserCheck } from './authBrowserChecks.mjs'
for (const scenario of ['login', 'registration', 'recovery', 'reset', 'chat', 'ui']) await runAuthBrowserCheck(scenario)
