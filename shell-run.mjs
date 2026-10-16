// Serve the probe with a real identity owner, run the shell's browser checks
// against it, and stop everything. Run under heavy.sh: one heavy job.
import { startIdentity } from './tests/support/identity.mjs'
import { ServerEnvironment } from './server/platform/runtime/ServerEnvironment.js'
import { writeFileSync } from 'node:fs'
const [ty, checks, only = ''] = process.argv.slice(2)
const port = 18620
const identity = await startIdentity()
const env = { ...identity.env, INBOUND_WEBHOOK_SECRET: 'probe-inbound-secret-0123456789abcdef', NODE_ENV: 'test', CADUCEUS_ENABLE_TEST_ROUTES: 'true' }
writeFileSync('.env', ServerEnvironment.dotenv(env).text)
const server = Bun.spawn([ty, 'serve', '--no-watch', '--no-bundle', '--port', String(port)], { stdout: 'ignore', stderr: 'pipe', env: { PATH: process.env.PATH, HOME: process.env.HOME } })
const base = `http://127.0.0.1:${port}`
let up = false
for (let i = 0; i < 240 && !up; i++) { try { await fetch(base + '/'); up = true } catch { await Bun.sleep(500) } }
try {
  if (!up) throw new Error('server did not start: ' + (await new Response(server.stderr).text()).slice(-800))
  const run = Bun.spawn(['bun', checks, base, only], { stdout: 'inherit', stderr: 'inherit' })
  await run.exited
} finally {
  server.kill(); identity.stop()
}
