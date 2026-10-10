import { describe, expect, it } from 'bun:test'
import {
  applySignature,
  currentSignatureId,
  defaultSignatureForAddress,
  sanitizeSignatureHTML as sanitizeBrowserSignature,
  signatureHTMLFromClipboard,
} from '../../client/shared/scripts/composer-signatures.js'
import { SignatureContent } from '../../server/shared/mailbox/SignatureContent.js'
import { SenderAddresses } from '../../server/shared/users/SenderAddresses.js'

const signature = { id: 'sig-work', text: '<p>Regards,<br>Alice</p>', defaultFor: ['alice@example.com'] }

describe('Mailspring-compatible composer signatures', () => {
  it('inserts before quoted mail and detects only the editable signature', () => {
    const quote = '<blockquote class="gmail_quote">Earlier message</blockquote>'
    const result = applySignature(`<p>Reply</p>${quote}`, signature)
    expect(result.indexOf('<signature')).toBeLessThan(result.indexOf(quote))
    expect(currentSignatureId(result)).toBe('sig-work')
    expect(currentSignatureId(`${quote}<signature id="quoted">Old</signature>`)).toBe('')
  })

  it('switches and removes signatures without duplicating whitespace', () => {
    const first = applySignature('<p>Hello</p>', signature)
    const second = applySignature(first, { id: 'sig-personal', text: '<p>Cheers</p>' })
    expect(second.match(/<signature\b/g)).toHaveLength(1)
    expect(second).toContain('id="sig-personal"')
    expect(second).not.toContain('sig-work')
    expect(applySignature(second, null)).toBe('<p>Hello</p><br>')
  })

  it('resolves exact-address defaults case-insensitively', () => {
    expect(defaultSignatureForAddress([signature], 'Alice@Example.com')?.id).toBe('sig-work')
    expect(defaultSignatureForAddress([signature], 'alias@example.com')).toBeNull()
  })

  it('sanitizes executable markup at browser, clipboard, and storage boundaries', () => {
    const hostile = '<script>alert(1)</script><p onclick="steal()"><a href="javascript:steal()">Alice</a></p>'
    for (const html of [sanitizeBrowserSignature(hostile), SignatureContent.sanitizeSignatureHTML(hostile), signatureHTMLFromClipboard('Alice', hostile)]) {
      expect(html).not.toMatch(/script|onclick|javascript:/i)
      expect(html).toContain('Alice')
    }
    expect(signatureHTMLFromClipboard('<b>Copied source</b>', '<i>Rendered</i>')).toBe('<b>Copied source</b>')
  })

  it('validates exact owned defaults and generates stable duplicate suffixes', () => {
    const allowedAddresses = ['alice@example.com', 'alias@example.com']
    expect(SignatureContent.normalizeSignatureInput({ name: ' Work ', text: '<b>Alice</b>', defaultFor: ['ALIAS@example.com'] }, { allowedAddresses }))
      .toEqual({ name: 'Work', text: '<b>Alice</b>', defaultFor: ['alias@example.com'] })
    expect(SignatureContent.normalizeSignatureInput({ name: 'Work', text: 'x', defaultFor: ['other@example.com'] }, { allowedAddresses }))
      .toBeInstanceOf(Error)
    expect(SignatureContent.resolveUniqueSignatureName('Work', [{ id: '1', name: 'work' }, { id: '2', name: 'Work 1' }])).toBe('Work 2')
  })

  it('authorizes From identities by primary address and aliases, never domain membership', async () => {
    const users = { findByEmail: async () => [
      'doc', { email: 'alice@example.com', aliases: ['sales@example.com'], domains: ['example.com'] },
    ] }
    const addresses = await SenderAddresses.ownedSenderAddresses(/** @type {any} */ (users), 'alice@example.com')
    expect(addresses).toEqual(['alice@example.com', 'sales@example.com'])
    expect(SenderAddresses.resolveOwnedSender('sales@example.com', addresses, 'alice@example.com')).toBe('sales@example.com')
    expect(SenderAddresses.resolveOwnedSender('bob@example.com', addresses, 'alice@example.com')).toBeNull()
  })
})
