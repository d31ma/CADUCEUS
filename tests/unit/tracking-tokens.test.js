import { describe, expect, it } from 'bun:test'
import {
  createTrackingToken,
  listTrackableLinks,
  prepareTrackedHtml,
  readTrackingToken,
  resolveTrackingBaseUrl,
} from '@/shared/delivery/tracking-tokens.js'

const secret = '0123456789abcdef0123456789abcdef'
const trackingId = '0123456789abcdef'
const baseUrl = 'https://mail.example.test'

describe('encrypted tracking capabilities', () => {
  it('round-trips open and click capabilities without exposing recipient or URL', () => {
    const open = createTrackingToken({ type: 'open', trackingId, recipient: 'Bob@Example.com' }, { secret, now: 1_000 })
    const click = createTrackingToken({
      type: 'click', trackingId, recipient: 'Bob@Example.com', url: 'https://example.com/path?q=1', linkIndex: 2,
    }, { secret, now: 1_000 })

    expect(open).not.toContain('bob')
    expect(click).not.toContain('example')
    expect(readTrackingToken(open, { secret, now: 2_000, expectedType: 'open' })).toMatchObject({
      type: 'open', trackingId, recipient: 'bob@example.com',
    })
    expect(readTrackingToken(click, { secret, now: 2_000, expectedType: 'click' })).toMatchObject({
      type: 'click', trackingId, recipient: 'bob@example.com', url: 'https://example.com/path?q=1', linkIndex: 2,
    })
  })

  it('fails closed on tampering, type confusion, expiry, malformed targets, and bad indices', () => {
    const token = createTrackingToken({ type: 'open', trackingId }, { secret, now: 1_000 })
    const tampered = `${token.slice(0, -1)}${token.endsWith('a') ? 'b' : 'a'}`
    expect(readTrackingToken(tampered, { secret, now: 2_000 })).toBeNull()
    expect(readTrackingToken(token, { secret, now: 2_000, expectedType: 'click' })).toBeNull()
    expect(readTrackingToken(token, { secret, now: 1_000 + 2 * 365 * 24 * 60 * 60 * 1000 + 1 })).toBeNull()
    expect(() => createTrackingToken({ type: 'click', trackingId, url: 'javascript:alert(1)', linkIndex: 0 }, { secret })).toThrow()
    expect(() => createTrackingToken({ type: 'click', trackingId, url: 'https://user:pass@example.com', linkIndex: 0 }, { secret })).toThrow()
    expect(() => createTrackingToken({ type: 'click', trackingId, url: 'https://example.com', linkIndex: -1 }, { secret })).toThrow()
  })
})

describe('per-recipient tracked HTML', () => {
  it('rewrites safe unquoted links and appends an invisible open pixel', () => {
    const source = [
      '<p><a href="https://example.com/a">Alpha</a></p>',
      '<p><a href="mailto:bob@example.com">Mail</a></p>',
      '<blockquote><a href="https://quoted.example/path">Quoted</a></blockquote>',
    ].join('')
    const result = prepareTrackedHtml(source, {
      trackingId,
      recipient: 'bob@example.com',
      openTracking: true,
      linkTracking: true,
      baseUrl,
      secret,
    })
    expect(result.links).toEqual([{ index: 0, url: 'https://example.com/a', title: 'Alpha' }])
    expect(result.html).toContain('https://quoted.example/path')
    expect(result.html).toContain('mailto:bob@example.com')
    expect(result.html).toContain('width="1"')
    expect(result.html).toContain('style="display:none"')

    const clickUrl = result.html.match(/href="(https:\/\/mail\.example\.test\/track\/click\?t=[^"]+)"/)?.[1]
    const openUrl = result.html.match(/src="(https:\/\/mail\.example\.test\/track\/open\?t=[^"]+)"/)?.[1]
    expect(clickUrl).toBeTruthy()
    expect(openUrl).toBeTruthy()
    expect(readTrackingToken(new URL(clickUrl).searchParams.get('t'), { secret, expectedType: 'click' })).toMatchObject({
      recipient: 'bob@example.com', url: 'https://example.com/a', linkIndex: 0,
    })
    expect(readTrackingToken(new URL(openUrl).searchParams.get('t'), { secret, expectedType: 'open' })).toMatchObject({
      recipient: 'bob@example.com',
    })
  })

  it('caps links, skips oversized HTML, and lists canonical originals', () => {
    const many = Array.from({ length: 120 }, (_, index) => `<a href="https://example.com/${index}">${index}</a>`).join('')
    expect(listTrackableLinks(many)).toHaveLength(100)
    expect(prepareTrackedHtml(many, { trackingId, linkTracking: true, baseUrl, secret }).links).toHaveLength(100)
    const oversized = `<p>${'x'.repeat(2 * 1024 * 1024)}</p>`
    expect(prepareTrackedHtml(oversized, { trackingId, openTracking: true, baseUrl, secret })).toEqual({ html: oversized, links: [] })
  })

  it('requires a safe production origin and a dedicated secret', () => {
    const previousNodeEnv = process.env.NODE_ENV
    const previousTracking = process.env.TRACKING_SECRET
    const previousInbound = process.env.INBOUND_WEBHOOK_SECRET
    try {
      process.env.NODE_ENV = 'production'
      delete process.env.TRACKING_SECRET
      delete process.env.INBOUND_WEBHOOK_SECRET
      expect(() => resolveTrackingBaseUrl('http://mail.example.test')).toThrow(/HTTPS/)
      expect(() => resolveTrackingBaseUrl('https://user:pass@mail.example.test')).toThrow(/credential-free/)
      expect(() => createTrackingToken({ type: 'open', trackingId })).toThrow(/TRACKING_SECRET/)
    } finally {
      if (previousNodeEnv === undefined) delete process.env.NODE_ENV
      else process.env.NODE_ENV = previousNodeEnv
      if (previousTracking === undefined) delete process.env.TRACKING_SECRET
      else process.env.TRACKING_SECRET = previousTracking
      if (previousInbound === undefined) delete process.env.INBOUND_WEBHOOK_SECRET
      else process.env.INBOUND_WEBHOOK_SECRET = previousInbound
    }
  })
})
