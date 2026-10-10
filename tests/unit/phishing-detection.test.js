import { describe, expect, it } from 'bun:test'
import { PhishingDetection } from '@/features/delivery/services/phishing-detection.js'

describe('PhishingDetection.analyze', () => {
  it('extracts display-name addresses without warning when Reply-To matches From', () => {
    expect(PhishingDetection.analyze({
      envelopeSender: 'bounce@mailer.example',
      headers: {
        from: 'Support Team <support@example.com>',
        'reply-to': 'Support Team <support@example.com>',
      },
    })).toEqual({
      headerFrom: 'support@example.com',
      replyTo: ['support@example.com'],
      securityWarnings: [],
    })
  })

  it('warns when replies are redirected away from the visible From address', () => {
    expect(PhishingDetection.analyze({
      envelopeSender: 'sender@example.com',
      headers: {
        from: 'Accounts <billing@example.com>',
        'reply-to': 'attacker@lookalike.test',
      },
    })).toEqual({
      headerFrom: 'billing@example.com',
      replyTo: ['attacker@lookalike.test'],
      securityWarnings: ['reply_to_mismatch'],
    })
  })

  it('falls back to the envelope sender and ignores invalid Reply-To values', () => {
    expect(PhishingDetection.analyze({
      envelopeSender: 'sender@example.com',
      headers: { 'reply-to': 'not an address' },
    })).toEqual({
      headerFrom: 'sender@example.com',
      replyTo: [],
      securityWarnings: [],
    })
  })
})
