import { describe, expect, it } from 'bun:test'
import { buildUnsubscribeAction } from '../../client/shared/scripts/unsubscribe.js'

describe('buildUnsubscribeAction', () => {
  it('uses an RFC 8058 one-click POST when explicitly advertised', () => {
    expect(buildUnsubscribeAction({
      unsubscribeUrl: 'https://lists.example.com/unsubscribe/abc',
      unsubscribeOneClick: true,
      unsubscribeMailto: 'mailto:leave@example.com',
    })).toEqual({ type: 'post', url: 'https://lists.example.com/unsubscribe/abc' })
  })

  it('prefers a reviewable mailto over a non-one-click preference page', () => {
    expect(buildUnsubscribeAction({
      unsubscribeUrl: 'https://lists.example.com/preferences',
      unsubscribeMailto: 'mailto:leave@example.com',
    })).toEqual({ type: 'compose', to: ['leave@example.com'], subject: '', body: '' })
  })

  it('opens a secure preference page when mailto is unavailable', () => {
    expect(buildUnsubscribeAction({ unsubscribeUrl: 'https://lists.example.com/preferences' }))
      .toEqual({ type: 'open', url: 'https://lists.example.com/preferences' })
  })

  it('builds a compose prefill from a mailto fallback', () => {
    expect(buildUnsubscribeAction({
      unsubscribeMailto: 'mailto:leave@example.com?subject=Remove%20me&body=Please%20unsubscribe%20me',
    })).toEqual({
      type: 'compose',
      to: ['leave@example.com'],
      subject: 'Remove me',
      body: 'Please unsubscribe me',
    })
  })

  it('fails closed for unsafe or ambiguous client metadata', () => {
    expect(buildUnsubscribeAction({ unsubscribeUrl: 'http://internal.example/unsubscribe' })).toBeNull()
    expect(buildUnsubscribeAction({ unsubscribeMailto: 'mailto:one@example.com,two@example.com' })).toBeNull()
  })

  it('uses a validated body-link only after header methods are exhausted', () => {
    expect(buildUnsubscribeAction({
      unsubscribeBodyUrl: 'https://lists.example.com/body-link',
    })).toEqual({ type: 'open', url: 'https://lists.example.com/body-link' })
  })
})
