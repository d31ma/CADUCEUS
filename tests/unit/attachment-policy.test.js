import { describe, expect, it } from 'bun:test'
import { AttachmentPolicy } from '@/shared/mailbox/AttachmentPolicy.js'

describe('attachment server policy', () => {
  it('turns path-like and directionally deceptive filenames into portable names', () => {
    expect(AttachmentPolicy.sanitizeAttachmentFilename('../../invoice\u202Efdp.exe')).toBe('invoicefdp.exe')
    expect(AttachmentPolicy.sanitizeAttachmentFilename('folder\\quarter/report?.pdf')).toBe('report-.pdf')
    expect(AttachmentPolicy.sanitizeAttachmentFilename('\u0000\u001f')).toBe('attachment')
  })

  it('normalizes Content-IDs without leaking regex state between calls', () => {
    expect(AttachmentPolicy.normalizeAttachmentContentId('cid:<Logo@Example.test>')).toBe('Logo@Example.test')
    expect(AttachmentPolicy.normalizeAttachmentContentId('bad\u0000id')).toBeUndefined()
    expect(AttachmentPolicy.normalizeAttachmentContentId('bad\u0000id')).toBeUndefined()
    expect(AttachmentPolicy.normalizeAttachmentContentId('space id')).toBeUndefined()
    expect(AttachmentPolicy.normalizeAttachmentContentId('<valid@example.test>')).toBe('valid@example.test')
  })
})
