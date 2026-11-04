import { runAuthBrowserCheck } from './authBrowserChecks.mjs'
for (const scenario of ['login', 'registration', 'recovery', 'reset', 'ui']) await runAuthBrowserCheck(scenario)
