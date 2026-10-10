import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { InboxController as Handler } from '@/controllers/InboxController.js'
import { call } from '../support/controller-call.mjs'

let app

beforeAll(async () => { app = await setupRoutes() })
afterAll(teardownRoutes)

describe('GET /inbox auth', () => {
  it('rejects a missing token', async () => {
    const res = await call(Handler, 'GET', req())
    expect(res.unauthorized).toBe(true)
  })

  it('returns emails for the caller domains', async () => {
    await app.emails.put({
      id: 'e1',
      domain: 'example.com',
      folder: 'inbox',
      from: 'bob@other.com',
      to: ['alice@example.com'],
      subject: 'Hello',
      text: 'hi',
      read: false,
      starred: false,
      receivedAt: new Date().toISOString(),
    })
    const res = await call(Handler, 'GET', req({ token: token() }))
    expect(Array.isArray(res)).toBe(true)
    expect(res.map(e => e.subject)).toContain('Hello')
  })

  it('never returns another mailbox copy from the same hosted domain', async () => {
    await app.emails.put({
      id: 'alice-owned-inbox',
      domain: 'example.com',
      ownerEmail: 'alice@example.com',
      recipient: 'alice@example.com',
      sender: 'sender@outside.test',
      subject: 'Alice private message',
      body: 'alice only',
      folder: 'inbox',
      read: false,
      starred: false,
      receivedAt: new Date().toISOString(),
      processed: true,
    })
    await app.emails.put({
      id: 'bob-owned-inbox',
      domain: 'example.com',
      ownerEmail: 'bob@example.com',
      recipient: 'bob@example.com',
      sender: 'sender@outside.test',
      subject: 'Bob private message',
      body: 'bob only',
      folder: 'inbox',
      read: false,
      starred: false,
      receivedAt: new Date().toISOString(),
      processed: true,
    })

    const result = await call(Handler, 'GET', req({ token: token() }))
    expect(result.map(message => message.id)).toContain('alice-owned-inbox')
    expect(result.map(message => message.id)).not.toContain('bob-owned-inbox')
  })
}, 300_000)

describe('Handler._parseFilters', () => {
  it('returns an empty filter set for an empty query', () => {
    expect(Handler._parseFilters({})).toEqual({})
  })

  it('reads q, falling back to search', () => {
    expect(Handler._parseFilters({ q: ' hi ' }).query).toBe('hi')
    expect(Handler._parseFilters({ search: 'yo' }).query).toBe('yo')
  })

  it('reads folder', () => {
    expect(Handler._parseFilters({ folder: 'sent' }).folder).toBe('sent')
  })

  it('coerces boolean filters', () => {
    expect(Handler._parseFilters({ read: 'true' }).read).toBe(true)
    expect(Handler._parseFilters({ starred: '0' }).starred).toBe(false)
    expect(Handler._parseFilters({ hasAttachment: true }).hasAttachment).toBe(true)
    expect(Handler._parseFilters({ attachments: 'false' }).hasAttachment).toBe(false)
  })

  it('rejects a non-boolean boolean filter', () => {
    expect(Handler._parseFilters({ read: 'yes' })).toBeInstanceOf(Error)
    expect(Handler._parseFilters({ starred: 'maybe' })).toBeInstanceOf(Error)
    expect(Handler._parseFilters({ hasAttachment: 'x' })).toBeInstanceOf(Error)
  })

  it('coerces numeric filters', () => {
    const f = Handler._parseFilters({ limit: '10', offset: '5' })
    expect(f.limit).toBe(10)
    expect(f.offset).toBe(5)
  })

  it('rejects negative and fractional numbers', () => {
    expect(Handler._parseFilters({ limit: '-1' })).toBeInstanceOf(Error)
    expect(Handler._parseFilters({ offset: '1.5' })).toBeInstanceOf(Error)
  })

  it('ignores empty and null values', () => {
    expect(Handler._parseFilters({ q: '  ', folder: '', read: '', limit: null })).toEqual({})
  })
})

describe('GET /inbox filter errors', () => {
  it('returns 400 for a bad filter', async () => {
    const res = await call(Handler, 'GET', req({ token: token(), query: { limit: 'abc' } }))
    expect(res.error).toMatch(/limit/)
  })
})
