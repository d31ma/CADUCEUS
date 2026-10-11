/**
 * The one server the Playwright suites run against: `ty serve` with the Go
 * server and the client on one origin (http://localhost:19875), backed by a
 * temporary Go FYLO owner and SESAME deployment. The test routes
 * (/test/seed/*, /test/reset) are on, and the inbound webhook takes the
 * secret tests/e2e/helpers.mjs signs with.
 *
 * CADUCEUS_SERVE_ROOT serves another project directory, TACHYON_BIN picks
 * the ty to run (see tests/support/TestServer.mjs).
 */
import { startIdentity } from '../tests/support/identity.mjs'
import { TestContainer } from '../tests/support/TestContainer.mjs'
import { TestServer } from '../tests/support/TestServer.mjs'

const port = Number(process.env.E2E_PORT) || 19875
const identity = await startIdentity()
let stopping = false
function stop(code = 0) {
  if (stopping) return
  stopping = true
  try {
    TestServer.stop()
    void TestContainer.close()
  } finally {
    identity.stop()
    process.exit(code)
  }
}
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => stop(0))

try {
  // The process that starts the server creates the FYLO collections; the
  // test container does, through the same bridge, as the route suites do.
  Object.assign(process.env, identity.env)
  await TestContainer.open()
  await TestServer.start({
    ...identity.env,
    NODE_ENV: 'test',
    CADUCEUS_ENABLE_TEST_ROUTES: 'true',
    INBOUND_WEBHOOK_SECRET: 'caduceus-e2e-inbound-secret',
    STREAM_MAX_MS: '750',
    STREAM_POLL_MS: '50',
  }, { port })
} catch (error) {
  console.error('[e2e] server failed to start:', error instanceof Error ? error.message : error)
  stop(1)
}
console.log(`[e2e] Go server and client on http://localhost:${port}`)
