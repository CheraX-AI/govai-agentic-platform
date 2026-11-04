// Current HTTPS/authentication expectations replace the retired demo flow.
import { runAuthBrowserCheck } from './integration/authBrowserChecks.mjs'
await runAuthBrowserCheck('login')
