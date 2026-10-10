import { describe, it, expect } from 'bun:test'
import { simpleParser } from 'mailparser'
import { buildOutboundMessageSource } from '@/features/mailbox/services/message-source.js'

describe('buildOutboundMessageSource', () => {
  it('builds a parseable Unicode multipart EML with attachments', async () => {
    const source = await buildOutboundMessageSource('alice@example.com', {
      to: ['bob@example.net'],
      cc: ['carol@example.net'],
      replyTo: ['replies@example.com'],
      messageId: '<reply-2@example.com>',
      inReplyTo: '<root@example.net>',
      references: ['<root@example.net>', '<reply-1@example.com>'],
      subject: 'Quarterly résumé',
      text: 'Plain version',
      html: '<p>HTML version</p>',
      attachments: [{
        filename: 'résumé.txt',
        contentType: 'text/plain',
        contentBase64: Buffer.from('attachment bytes').toString('base64'),
      }],
    }, 'sent-123', new Date('2026-08-11T12:00:00.000Z'))

    const parsed = await simpleParser(source)
    expect(parsed.from?.value[0].address).toBe('alice@example.com')
    expect(parsed.to?.value[0].address).toBe('bob@example.net')
    expect(parsed.subject).toBe('Quarterly résumé')
    expect(parsed.text).toContain('Plain version')
    expect(parsed.html).toContain('HTML version')
    expect(parsed.messageId).toBe('<reply-2@example.com>')
    expect(parsed.inReplyTo).toBe('<root@example.net>')
    expect(parsed.references).toEqual(['<root@example.net>', '<reply-1@example.com>'])
    expect(parsed.attachments[0].filename).toBe('résumé.txt')
    expect(parsed.attachments[0].content.toString()).toBe('attachment bytes')
  })
})
