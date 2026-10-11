import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, email } from './helpers.js'
import { TestServer } from '../support/TestServer.mjs'

// GET /inbox/stream is a WebSocket on the Go server (STREAM_MAX_MS is 750
// and STREAM_POLL_MS 50 in route tests). Each message is JSON
// {event, data, cursor}; the server closes the socket after reconnect.

let app

/** Opens the stream and answers every message until the server closes it. */
function stream(query) {
  const url = new URL('/inbox/stream', TestServer.current.url)
  url.protocol = 'ws:'
  for (const [key, value] of Object.entries(query)) url.searchParams.set(key, value)
  return new Promise((resolve, reject) => {
    const messages = []
    const socket = new WebSocket(url)
    const timer = setTimeout(() => { socket.close(); reject(new Error(`stream did not close: ${JSON.stringify(messages)}`)) }, 20_000)
    socket.onmessage = event => messages.push(JSON.parse(String(event.data)))
    socket.onerror = () => { clearTimeout(timer); reject(new Error('stream failed')) }
    socket.onclose = event => { clearTimeout(timer); resolve({ messages, code: event.code }) }
  })
}

function decoded(cursor) {
  expect(cursor).toMatch(/^v1\.[A-Za-z0-9_-]+$/)
  return JSON.parse(Buffer.from(cursor.slice(3), 'base64url').toString('utf8'))
}

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('the /inbox/stream socket', () => {
  it('answers a plain HTTP request with 426 Upgrade Required', async () => {
    const response = await fetch(new URL('/inbox/stream', TestServer.current.url))
    expect(response.status).toBe(426)
  })

  it('sends one unauthorized message and closes without a valid token', async () => {
    for (const query of [{}, { token: 'not-a-token' }]) {
      const { messages } = await stream(query)
      expect(messages).toEqual([{ event: 'unauthorized', data: { error: 'Authentication required' }, cursor: null }])
    }
  })

  it('announces the cursor, replays newer inbox mail oldest first, then asks for a reconnect', async () => {
    const since = '2026-08-22T12:00:00.000Z'
    await app.emails.put(email({ id: 'stream-old', receivedAt: '2026-08-22T11:59:59.000Z' }))
    await app.emails.put(email({ id: 'stream-first', subject: 'First live', receivedAt: '2026-08-22T12:00:01.000Z', spamScore: 2 }))
    await app.emails.put(email({ id: 'stream-second', subject: 'Second live', receivedAt: '2026-08-22T12:00:02.000Z' }))
    await app.emails.put(email({ id: 'stream-archived', folder: 'archive', receivedAt: '2026-08-22T12:00:03.000Z' }))
    await app.emails.put(email({ id: 'stream-bob', ownerEmail: 'bob@example.com', recipient: 'bob@example.com', receivedAt: '2026-08-22T12:00:04.000Z' }))

    const { messages } = await stream({ token: token(), since })
    expect(messages[0].event).toBe('connected')
    expect(messages[0].cursor).toBeNull()
    expect(decoded(messages[0].data.since)).toEqual({ hosted: { at: since, id: '' }, external: { at: since, id: '' } })

    const fresh = messages.filter(message => message.event === 'new-email')
    expect(fresh.map(message => message.data.id)).toEqual(['stream-first', 'stream-second'])
    expect(fresh[0].data).toMatchObject({
      id: 'stream-first', domain: 'example.com', subject: 'First live', folder: 'inbox', systemFolder: 'inbox',
      read: false, starred: false, receivedAt: '2026-08-22T12:00:01.000Z', spamScore: 2,
    })
    expect(fresh[0].data.body).toBeUndefined()
    expect(fresh[1].data.spamScore).toBeUndefined()
    expect(decoded(fresh[1].cursor).hosted).toEqual({ at: '2026-08-22T12:00:02.000Z', id: 'stream-second' })

    expect(messages.some(message => message.event === 'ping' && message.cursor === null)).toBe(true)
    const last = messages.at(-1)
    expect(last.event).toBe('reconnect')
    expect(decoded(last.data.since).hosted).toEqual({ at: '2026-08-22T12:00:02.000Z', id: 'stream-second' })

    // A reconnect from the last cursor replays nothing that was already sent.
    const again = await stream({ token: token(), since: last.data.since })
    expect(again.messages.filter(message => message.event === 'new-email')).toEqual([])
  })
})
