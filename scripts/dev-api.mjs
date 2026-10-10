/**
 * Local full-stack development server with a seeded login bypass.
 *
 * The standalone Tachyon CLI detects `client/`, `server/`, and `db/`, so one
 * `ty serve` process now owns the frontend, API, HMR, and FYLO runtime on the
 * same origin. Pass `--seed-emails` (or set `SEED_EMAILS=1`) to populate the
 * inbox after the development user is created.
 */
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const projectRoot = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')
const port = Number(process.env.DEV_API_PORT) || 9876
const appUrl = `http://127.0.0.1:${port}`
const seedEmails = process.argv.includes('--seed-emails') || process.env.SEED_EMAILS === '1'
const fyloRoot = process.env.FYLO_ROOT || join(projectRoot, 'db')
const inboundWebhookSecret = process.env.INBOUND_WEBHOOK_SECRET || 'caduceus-e2e-inbound-secret'
const adminEmail = 'admin@caduceus.com'
const developmentCredentialKey = Buffer.alloc(32, 0x43).toString('base64url')

const server = spawn('ty', ['serve'], {
  cwd: projectRoot,
  stdio: 'inherit',
  env: {
    ...process.env,
    FYLO_ROOT: fyloRoot,
    FYLO_SCHEMA: process.env.FYLO_SCHEMA || join(projectRoot, 'db', 'schemas'),
    EXTERNAL_ACCOUNT_CREDENTIAL_KEYS: process.env.EXTERNAL_ACCOUNT_CREDENTIAL_KEYS || JSON.stringify({ 1: developmentCredentialKey }),
    EXTERNAL_ACCOUNT_ACTIVE_KEY: process.env.EXTERNAL_ACCOUNT_ACTIVE_KEY || '1',
    INBOUND_WEBHOOK_SECRET: inboundWebhookSecret,
    CADUCEUS_ENABLE_TEST_ROUTES: 'true',
    NODE_ENV: 'test',
    SMTP_ADAPTER: 'console',
    YON_PORT: String(port),
    YON_HOST: '127.0.0.1',
  },
})

let shuttingDown = false
function shutdown(signal = 'SIGTERM') {
  if (shuttingDown) return
  shuttingDown = true
  if (!server.killed) server.kill(/** @type {NodeJS.Signals} */ (signal))
}

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => {
    shutdown(signal)
    process.exit(0)
  })
}

server.on('exit', code => {
  if (!shuttingDown) process.exit(code ?? 0)
})

async function waitForServer() {
  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    try {
      // `ty serve` owns the transitional development listener and serves the
      // compiled root as its readiness boundary. `/healthz` belongs to the Go
      // production runtime and is intentionally not mounted by Tachyon.
      const response = await fetch(`${appUrl}/`, {
        signal: AbortSignal.timeout(500),
      })
      if (response.ok) return
    } catch {
      await Bun.sleep(150)
    }
  }
  throw new Error(`Development server did not become ready at ${appUrl}`)
}

await waitForServer()

let token = ''
try {
  const response = await fetch(`${appUrl}/test/seed/user`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: adminEmail,
      role: 'admin',
      domains: ['caduceus.com'],
    }),
  })
  token = (await response.json()).token || ''
} catch (error) {
  console.warn('[dev-api] admin seed failed:', error instanceof Error ? error.message : error)
}

if (seedEmails && token) {
  console.log('[dev-api] Seeding sample emails…')
  spawn('bun', ['scripts/seed-emails.mjs'], {
    cwd: projectRoot,
    stdio: 'inherit',
    env: {
      ...process.env,
      SEED_API: appUrl,
      INBOUND_WEBHOOK_SECRET: inboundWebhookSecret,
    },
  })
}

const snippet = token
  ? `sessionStorage.setItem('caduceus_token',${JSON.stringify(token)});` +
    `sessionStorage.setItem('caduceus_email','${adminEmail}');` +
    `sessionStorage.setItem('caduceus_role','admin');` +
    `sessionStorage.setItem('caduceus_domains','["caduceus.com"]');location.href='/inbox'`
  : ''

console.log(`\n[dev-api] full-stack dev server ready → ${appUrl}`)
console.log(`[dev-api] data ${fyloRoot}`)
if (snippet) {
  console.log('\n[dev-api] Login bypass — paste this in the DevTools console:\n')
  console.log(`  ${snippet}\n`)
}
if (!seedEmails) console.log('[dev-api] Tip: pass --seed-emails to populate the inbox.\n')
