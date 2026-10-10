import { describe, expect, it } from 'bun:test'
import { externalDeliveryRequest, externalUndoToken, parseExternalUndoToken } from '../../server/shared/delivery/external-submission.js'
import { readTrackingToken } from '../../server/shared/delivery/tracking-tokens.js'

describe('external submission undo capabilities', () => {
  it('round-trips only bounded account, submission, and Message-ID values', () => {
    const value = externalUndoToken('ext_aaaaaaaaaaaaaaaaaaaa', 'sub_bbbbbbbbbbbbbbbbbbbb', '<one@example.net>')
    expect(parseExternalUndoToken(value)).toEqual({
      accountId: 'ext_aaaaaaaaaaaaaaaaaaaa', submissionId: 'sub_bbbbbbbbbbbbbbbbbbbb', messageId: '<one@example.net>',
    })
    expect(parseExternalUndoToken(value + 'corrupt')).toBeNull()
    expect(externalUndoToken('bad', 'sub_bbbbbbbbbbbbbbbbbbbb', '<one@example.net>')).toBe('')

  })

  it('carries an optional owner-reauthorized archive action', () => {
    const value = externalUndoToken('ext_aaaaaaaaaaaaaaaaaaaa', 'sub_bbbbbbbbbbbbbbbbbbbb', '<one@example.net>', 'mail-action-123')
    expect(parseExternalUndoToken(value)).toEqual({
      accountId: 'ext_aaaaaaaaaaaaaaaaaaaa', submissionId: 'sub_bbbbbbbbbbbbbbbbbbbb',
      messageId: '<one@example.net>', archiveActionId: 'mail-action-123',
    })
    expect(externalUndoToken('ext_aaaaaaaaaaaaaaaaaaaa', 'sub_bbbbbbbbbbbbbbbbbbbb', '<one@example.net>', '../foreign')).toBe('')
  })
})

describe('connected-account delivery rendering', () => {
  it('embeds exact-recipient tracking capabilities without changing the clean request', () => {
    const previousSecret = process.env.TRACKING_SECRET
    process.env.TRACKING_SECRET = 'external-submission-test-secret'
    try {
      const request = { to: ['recipient@example.net'], subject: 'Tracked', html: '<a href="https://example.net/path">Visit</a>', openTracking: true, linkTracking: true }
      const rendered = externalDeliveryRequest(request, 'tracking_abcdefghijklmnop', 'recipient@example.net')
      expect(request.html).toBe('<a href="https://example.net/path">Visit</a>')
      expect(rendered.html).toContain('/track/click?t=')
      expect(rendered.html).toContain('/track/open?t=')
      const tokens = [...rendered.html.matchAll(/[?&]t=([^"&]+)/g)].map(match => decodeURIComponent(match[1]))
      expect(tokens).toHaveLength(2)
      expect(tokens.map(token => readTrackingToken(token)?.recipient)).toEqual(['recipient@example.net', 'recipient@example.net'])
    } finally {
      if (previousSecret == null) delete process.env.TRACKING_SECRET
      else process.env.TRACKING_SECRET = previousSecret
    }
  })
})
