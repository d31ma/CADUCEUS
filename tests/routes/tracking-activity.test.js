import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { ActivityController as Activity } from '@/controllers/ActivityController.js'
import { TrackClickController as TrackClick } from '@/controllers/TrackClickController.js'
import { TrackOpenController as TrackOpen } from '@/controllers/TrackOpenController.js'
import { TrackingTokens } from '@/shared/delivery/TrackingTokens.js'
import { call } from '../support/controller-call.mjs'

const secret = 'tracking-test-secret-0123456789abcdef'
const aliceTrackingId = 'alice-tracking-000001'
const bobTrackingId = 'bob-tracking-0000001'
const threadId = 'a'.repeat(64)
let app
let previousSecret

beforeAll(async () => {
  previousSecret = process.env.TRACKING_SECRET
  process.env.TRACKING_SECRET = secret
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: ['sales@example.com'], domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'bob@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.emails.put(email({
    id: 'sent-tracked-alice',
    ownerEmail: 'alice@example.com',
    domain: 'example.com',
    sender: 'sales@example.com',
    recipient: 'recipient@outside.test',
    to: ['recipient@outside.test'],
    subject: 'Tracked proposal',
    folder: 'sent',
    read: true,
    trackingId: aliceTrackingId,
    openTracking: true,
    linkTracking: true,
    trackedLinksJson: JSON.stringify([{ index: 0, url: 'https://example.org/proposal', title: 'Proposal' }]),
    opensJson: '[]',
    clicksJson: '[]',
    openCount: 0,
    clickCount: 0,
    threadId,
    receivedAt: new Date(Date.now() - 60_000).toISOString(),
  }))
  await app.emails.put(email({
    id: 'reply-alice',
    domain: 'example.com',
    recipient: 'alice@example.com',
    to: ['alice@example.com'],
    sender: 'recipient@outside.test',
    subject: 'Re: Tracked proposal',
    folder: 'inbox',
    threadId,
    receivedAt: new Date(Date.now() - 30_000).toISOString(),
  }))
  await app.emails.put(email({
    id: 'sent-tracked-bob',
    ownerEmail: 'bob@example.com',
    domain: 'example.com',
    sender: 'bob@example.com',
    recipient: 'private@outside.test',
    to: ['private@outside.test'],
    subject: 'Bob private campaign',
    folder: 'sent',
    read: true,
    trackingId: bobTrackingId,
    openTracking: true,
    linkTracking: false,
    trackedLinksJson: '[]',
    opensJson: JSON.stringify([{ id: 'open-bob', recipient: 'private@outside.test', openedAt: new Date().toISOString() }]),
    clicksJson: '[]',
    openCount: 1,
    clickCount: 0,
  }))
}, 300_000)

afterAll(async () => {
  await teardownRoutes()
  if (previousSecret === undefined) delete process.env.TRACKING_SECRET
  else process.env.TRACKING_SECRET = previousSecret
})

describe('public tracking capabilities', () => {
  it('always returns the same open pixel and records only valid open capabilities', async () => {
    const invalid = await call(TrackOpen, 'GET', req({ query: { t: 'not-a-token' } }))
    expect(invalid.status).toBe(200)
    expect(invalid.headers.get('Content-Type')).toBe('image/gif')

    const open = TrackingTokens.createTrackingToken({
      type: 'open', trackingId: aliceTrackingId, recipient: 'recipient@outside.test',
    }, { secret })
    expect((await call(TrackOpen, 'GET', req({ query: { t: open } }))).status).toBe(200)
    const [, stored] = await app.emails.findById('sent-tracked-alice')
    expect(stored.openCount).toBe(1)
    expect(stored.opens).toHaveLength(1)
    expect(stored.opens[0].recipient).toBe('recipient@outside.test')
  })

  it('rejects arbitrary, tampered, mismatched, and type-confused redirects', async () => {
    expect((await call(TrackClick, 'GET', req({ query: { url: 'https://attacker.example' } }))).status).toBe(404)
    const mismatch = TrackingTokens.createTrackingToken({
      type: 'click', trackingId: aliceTrackingId, recipient: 'recipient@outside.test', url: 'https://attacker.example/', linkIndex: 0,
    }, { secret })
    expect((await call(TrackClick, 'GET', req({ query: { t: mismatch } }))).status).toBe(404)
    const open = TrackingTokens.createTrackingToken({ type: 'open', trackingId: aliceTrackingId }, { secret })
    expect((await call(TrackClick, 'GET', req({ query: { t: open } }))).status).toBe(404)
    const valid = TrackingTokens.createTrackingToken({
      type: 'click', trackingId: aliceTrackingId, recipient: 'recipient@outside.test', url: 'https://example.org/proposal', linkIndex: 0,
    }, { secret })
    const tampered = `${valid.slice(0, -1)}${valid.endsWith('a') ? 'b' : 'a'}`
    expect((await call(TrackClick, 'GET', req({ query: { t: tampered } }))).status).toBe(404)
  })

  it('records an authenticated click and redirects only to the stored original', async () => {
    const click = TrackingTokens.createTrackingToken({
      type: 'click', trackingId: aliceTrackingId, recipient: 'recipient@outside.test', url: 'https://example.org/proposal', linkIndex: 0,
    }, { secret })
    const response = await call(TrackClick, 'GET', req({ query: { t: click } }))
    expect(response.status).toBe(302)
    expect(response.headers.get('Location')).toBe('https://example.org/proposal')
    expect(response.headers.get('Referrer-Policy')).toBe('no-referrer')
    const [, stored] = await app.emails.findById('sent-tracked-alice')
    expect(stored.clickCount).toBe(1)
    expect(stored.clicks[0]).toMatchObject({
      recipient: 'recipient@outside.test', url: 'https://example.org/proposal', title: 'Proposal', linkIndex: 0,
    })
  })

  it('serializes simultaneous events so neither update is lost', async () => {
    const one = TrackingTokens.createTrackingToken({ type: 'open', trackingId: aliceTrackingId, recipient: 'one@outside.test' }, { secret })
    const two = TrackingTokens.createTrackingToken({ type: 'open', trackingId: aliceTrackingId, recipient: 'two@outside.test' }, { secret })
    await Promise.all([
      call(TrackOpen, 'GET', req({ query: { t: one } })),
      call(TrackOpen, 'GET', req({ query: { t: two } })),
    ])
    const [, stored] = await app.emails.findById('sent-tracked-alice')
    expect(stored.openCount).toBe(3)
    expect(stored.opens.map(item => item.recipient)).toEqual([
      'recipient@outside.test', 'one@outside.test', 'two@outside.test',
    ])
  })
})

describe('GET /activity', () => {
  it('validates filters and requires authentication', async () => {
    expect((await call(Activity, 'GET', req())).unauthorized).toBe(true)
    expect(Activity.parseActivityQuery({ type: 'bounce' }).ok).toBe(false)
    expect(Activity.parseActivityQuery({ range: 'forever' }).ok).toBe(false)
    expect(Activity.parseActivityQuery({ limit: '101' }).ok).toBe(false)
    expect(Activity.parseActivityQuery({ from: '2026-08-14', to: '2026-08-13' }).ok).toBe(false)
  })

  it('returns an exact-owner feed, filters event type, and calculates local analytics', async () => {
    const since = new Date(Date.now() - 10 * 60_000).toISOString()
    const result = await call(Activity, 'GET', req({ token: token(), query: { range: '7d', since } }))
    expect(result.events.map(item => item.messageId)).toEqual([
      'sent-tracked-alice', 'sent-tracked-alice', 'sent-tracked-alice', 'sent-tracked-alice',
    ])
    expect(result.events.some(item => item.subject === 'Bob private campaign')).toBe(false)
    expect(result.unseenCount).toBe(4)
    expect(result.metrics).toMatchObject({
      sent: 1,
      received: 1,
      tracked: 1,
      opened: 1,
      clicked: 1,
      replied: 1,
      trackingRate: 100,
      openRate: 100,
      clickRate: 100,
      replyRate: 100,
    })
    expect(result.metrics.bySubject[0]).toMatchObject({ subject: 'Tracked proposal', sent: 1, opened: 1, clicked: 1, replied: 1 })
    expect(result.metrics.daily.length).toBeGreaterThan(0)

    const clicks = await call(Activity, 'GET', req({ token: token(), query: { type: 'click', range: '7d' } }))
    expect(clicks.events).toHaveLength(1)
    expect(clicks.events[0]).toMatchObject({ type: 'click', title: 'Proposal' })
  })

  it('does not reveal same-domain activity to another mailbox', async () => {
    const result = await call(Activity, 'GET', req({ token: token({ email: 'bob@example.com' }), query: { range: '7d' } }))
    expect(result.events).toHaveLength(1)
    expect(result.events[0].subject).toBe('Bob private campaign')
    expect(result.events.some(item => item.messageId === 'sent-tracked-alice')).toBe(false)
  })
})
