import { describe, expect, it } from 'bun:test'
import { ListUnsubscribe } from '@/features/delivery/services/list-unsubscribe.js'

describe('ListUnsubscribe.parse', () => {
  it('prefers a secure web endpoint while retaining a valid mailto fallback', () => {
    expect(ListUnsubscribe.parse({
      'list-unsubscribe': '<mailto:leave@example.com?subject=unsubscribe>, <https://lists.example.com/u/abc>',
      'list-unsubscribe-post': 'List-Unsubscribe=One-Click',
    })).toEqual({
      unsubscribeUrl: 'https://lists.example.com/u/abc',
      unsubscribeMailto: 'mailto:leave@example.com?subject=unsubscribe',
      unsubscribeBodyUrl: '',
      unsubscribeOneClick: true,
    })
  })

  it('accepts header names case-insensitively and normalizes the mailto mailbox', () => {
    expect(ListUnsubscribe.parse({
      'List-Unsubscribe': '<mailto:REMOVE@Example.COM>',
    })).toEqual({
      unsubscribeUrl: '',
      unsubscribeMailto: 'mailto:remove@example.com',
      unsubscribeBodyUrl: '',
      unsubscribeOneClick: false,
    })
  })

  it('rejects insecure, credentialed, malformed, and multi-recipient targets', () => {
    expect(ListUnsubscribe.parse({
      'list-unsubscribe': [
        '<http://lists.example.com/u/abc>',
        '<https://user:secret@lists.example.com/u/abc>',
        '<mailto:one@example.com,two@example.com>',
        '<javascript:alert(1)>',
      ].join(', '),
      'list-unsubscribe-post': 'List-Unsubscribe=One-Click',
    })).toEqual({
      unsubscribeUrl: '',
      unsubscribeMailto: '',
      unsubscribeBodyUrl: '',
      unsubscribeOneClick: false,
    })
  })

  it('does not treat a web endpoint as one-click without the RFC 8058 header', () => {
    expect(ListUnsubscribe.parse({
      'list-unsubscribe': '<https://lists.example.com/preferences>',
    })).toEqual({
      unsubscribeUrl: 'https://lists.example.com/preferences',
      unsubscribeMailto: '',
      unsubscribeBodyUrl: '',
      unsubscribeOneClick: false,
    })
  })

  it('falls back to a secure localized unsubscribe link in the HTML body', () => {
    expect(ListUnsubscribe.parse({}, `
      <p>Noticias semanales.</p>
      <p>Para darse de baja, <a href="https://lists.example.com/preferences/abc">pulse aquí</a>.</p>
    `)).toEqual({
      unsubscribeUrl: '',
      unsubscribeMailto: '',
      unsubscribeBodyUrl: 'https://lists.example.com/preferences/abc',
      unsubscribeOneClick: false,
    })
  })

  it('ignores body candidates inside quoted content and unsafe links', () => {
    expect(ListUnsubscribe.parse({}, `
      <blockquote><a href="https://old.example.com/unsubscribe">Unsubscribe</a></blockquote>
      <p><a href="http://lists.example.com/unsubscribe">Unsubscribe</a></p>
    `)).toEqual({
      unsubscribeUrl: '',
      unsubscribeMailto: '',
      unsubscribeBodyUrl: '',
      unsubscribeOneClick: false,
    })
  })
})
