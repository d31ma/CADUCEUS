import { describe, expect, it } from 'bun:test'
import { MailValidation } from '@/shared/delivery/MailValidation.js'

const base = { to: ['recipient@example.com'], subject: 'Hello', text: 'Body' }

describe('MailValidation.validateSendRequest replyTo', () => {
  it('normalizes an optional From identity for later ownership enforcement', () => {
    expect(MailValidation.validateSendRequest({ ...base, from: ' Sales@Example.com ' })).toEqual({
      ok: true,
      value: { ...base, from: 'sales@example.com' },
    })
    expect(MailValidation.validateSendRequest({ ...base, from: 'not-an-address' }).ok).toBe(false)
  })

  it('normalizes one optional Reply-To mailbox', () => {
    expect(MailValidation.validateSendRequest({
      ...base,
      replyTo: ['  Replies@Example.NET  '],
    })).toEqual({
      ok: true,
      value: {
        ...base,
        replyTo: ['replies@example.net'],
      },
    })
  })

  it('rejects malformed, multiple, and header-injection Reply-To values', () => {
    for (const replyTo of [
      'reply@example.net',
      [],
      ['one@example.net', 'two@example.net'],
      ['not-an-address'],
      ['safe@example.net\r\nBcc: victim@example.net'],
    ]) {
      const result = MailValidation.validateSendRequest({ ...base, replyTo })
      expect(result.ok).toBe(false)
      expect(result.error).toMatch(/replyTo/)
    }
  })
})

describe('MailValidation.validateSendRequest RFC threading', () => {
  it('normalizes a bounded reply lineage', () => {
    expect(MailValidation.validateSendRequest({
      ...base,
      inReplyTo: 'parent@outside.test',
      references: ['<root@outside.test>', 'parent@outside.test'],
    })).toEqual({
      ok: true,
      value: {
        ...base,
        inReplyTo: '<parent@outside.test>',
        references: ['<root@outside.test>', '<parent@outside.test>'],
      },
    })
  })

  it('rejects malformed, duplicate, and oversized reference input', () => {
    for (const patch of [
      { inReplyTo: 'not-an-id' },
      { references: '<root@test>' },
      { references: ['<same@test>', '<same@test>'] },
      { references: Array.from({ length: 101 }, (_, i) => `<m${i}@test>`) },
    ]) {
      expect(MailValidation.validateSendRequest({ ...base, ...patch }).ok).toBe(false)
    }
  })
})

describe('MailValidation.validateSendRequest tracking and recipients', () => {
  it('accepts explicit HTML tracking and normalizes Cc/Bcc recipients', () => {
    expect(MailValidation.validateSendRequest({
      to: ['recipient@example.com'],
      cc: [' Cc@Example.net '],
      bcc: [' Bcc@Example.net '],
      subject: 'Tracked',
      html: '<p>Body</p>',
      openTracking: true,
      linkTracking: true,
    })).toEqual({
      ok: true,
      value: {
        to: ['recipient@example.com'],
        cc: ['cc@example.net'],
        bcc: ['bcc@example.net'],
        subject: 'Tracked',
        html: '<p>Body</p>',
        openTracking: true,
        linkTracking: true,
      },
    })
  })

  it('rejects implicit, non-HTML, and malformed tracking requests', () => {
    expect(MailValidation.validateSendRequest({ ...base, openTracking: 'yes' }).error).toMatch(/boolean/)
    expect(MailValidation.validateSendRequest({ ...base, linkTracking: true }).error).toMatch(/HTML/)
    expect(MailValidation.validateSendRequest({ ...base, cc: ['not-an-address'] }).error).toMatch(/cc/)
    expect(MailValidation.validateSendRequest({ ...base, bcc: 'hidden@example.net' }).error).toMatch(/array/)
  })
})

describe('MailValidation.validateSendAttachments', () => {
  const attachment = {
    filename: '../invoice?.txt',
    contentType: 'text/plain',
    contentBase64: btoa('hello'),
    disposition: 'inline',
    contentId: '<note@example.test>',
  }

  it('sanitizes filenames and normalizes inline Content-IDs', () => {
    expect(MailValidation.validateSendAttachments([attachment])).toEqual({
      ok: true,
      value: [{
        filename: 'invoice-.txt',
        contentType: 'text/plain',
        contentBase64: attachment.contentBase64,
        disposition: 'inline',
        contentId: 'note@example.test',
      }],
    })
  })

  it('rejects malformed and duplicate Content-IDs', () => {
    expect(MailValidation.validateSendAttachments([{ ...attachment, contentId: 'bad id' }]).ok).toBe(false)
    expect(MailValidation.validateSendAttachments([
      attachment,
      { ...attachment, filename: 'second.txt', contentId: 'NOTE@example.test' },
    ]).ok).toBe(false)
  })
})
