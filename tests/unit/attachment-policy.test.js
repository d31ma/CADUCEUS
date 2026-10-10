import { describe, expect, it } from 'bun:test'
import {
  normalizeAttachmentContentId,
  sanitizeAttachmentFilename,
} from '@/shared/mailbox/attachment-policy.js'

describe('attachment server policy', () => {
  it('turns path-like and directionally deceptive filenames into portable names', () => {
    expect(sanitizeAttachmentFilename('../../invoice\u202Efdp.exe')).toBe('invoicefdp.exe')
    expect(sanitizeAttachmentFilename('folder\\quarter/report?.pdf')).toBe('report-.pdf')
    expect(sanitizeAttachmentFilename('\u0000\u001f')).toBe('attachment')
  })

  it('normalizes Content-IDs without leaking regex state between calls', () => {
    expect(normalizeAttachmentContentId('cid:<Logo@Example.test>')).toBe('Logo@Example.test')
    expect(normalizeAttachmentContentId('bad\u0000id')).toBeUndefined()
    expect(normalizeAttachmentContentId('bad\u0000id')).toBeUndefined()
    expect(normalizeAttachmentContentId('space id')).toBeUndefined()
    expect(normalizeAttachmentContentId('<valid@example.test>')).toBe('valid@example.test')
  })
})
