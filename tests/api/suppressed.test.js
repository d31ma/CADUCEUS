import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { startTestServer, TEST_SERVER_HOOK_TIMEOUT_MS } from './helpers.js'

let server
let admin

beforeAll(async () => {
  server = await startTestServer()
  admin = await server.token({ email: 'admin@example.com', domains: ['example.com'], role: 'admin' })
}, TEST_SERVER_HOOK_TIMEOUT_MS)

afterAll(() => server?.stop())

describe('suppressed recipients API', () => {
  it('requires an administrator', async () => {
    expect((await server.post('/suppressed', { address: 'bad@example.com' })).status).toBe(401)
    const viewer = await server.token({ email: 'viewer@example.com', domains: ['example.com'], role: 'viewer' })
    expect((await server.post('/suppressed', { address: 'bad@example.com' }, { token: viewer })).status).toBe(403)
  })

  it('adds a recipient to the suppression list', async () => {
    const response = await server.post('/suppressed', { address: 'bad@example.com', reason: 'bounce' }, { token: admin })
    expect(response.status).toBe(200)
    expect(await response.json()).toEqual({ suppressed: 'bad@example.com', reason: 'bounce' })
  })

  it('lists and removes suppressed recipients', async () => {
    await server.post('/suppressed', { address: 'remove@example.com', reason: 'complaint' }, { token: admin })
    const list = await server.get('/suppressed', { token: admin })
    expect((await list.json()).some(item => item.address === 'remove@example.com')).toBe(true)

    const removed = await server.delete('/suppressed/remove@example.com', { token: admin })
    expect(removed.status).toBe(200)
    expect(await removed.json()).toEqual({ removed: 'remove@example.com' })
  })
})
