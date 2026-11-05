import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Existing local certificates are server-only and never included in the bundle.
const certFile = fileURLToPath(new URL('../../infrastructure/keycloak/tls/api-cert.pem', import.meta.url))
const keyFile = fileURLToPath(new URL('../../infrastructure/keycloak/tls/api-key.pem', import.meta.url))
const hasCert = existsSync(certFile)
const hasKey = existsSync(keyFile)
if (hasCert !== hasKey) throw new Error('Local HTTPS requires both certificate and key files.')

export default defineConfig({
  plugins: [react()],
  server: {
    host: 'localhost',
    port: 5173,
    strictPort: true,
    https: hasCert ? { cert: readFileSync(certFile), key: readFileSync(keyFile) } : undefined,
    // Docker on Windows does not pass file-change events into containers, so the
    // dev container polls for changes (VITE_WATCH_POLLING=true in docker-compose.yml)
    watch: process.env.VITE_WATCH_POLLING === 'true' ? { usePolling: true, interval: 300 } : undefined,
  },
})
