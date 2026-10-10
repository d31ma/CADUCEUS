import { describe, expect, it } from 'bun:test'
import {
  attachmentPreviewDataUrl,
  attachmentPreviewKind,
  decodeAttachmentText,
  normalizeCidReference,
} from '../../client/shared/scripts/attachment-content.js'

describe('attachment browser content policy', () => {
  it('normalizes encoded CID references and rejects malformed IDs', () => {
    expect(normalizeCidReference('cid:%3Clogo%40Example.TEST%3E')).toBe('logo@example.test')
    expect(normalizeCidReference('cid:logo example.test')).toBe('')
    expect(normalizeCidReference('cid:<broken>suffix')).toBe('')
  })

  it('previews inert raster, text, and PDF types but excludes active SVG and HTML', () => {
    expect(attachmentPreviewKind('image/png; name=logo.png')).toBe('image')
    expect(attachmentPreviewKind('text/plain')).toBe('text')
    expect(attachmentPreviewKind('application/json')).toBe('text')
    expect(attachmentPreviewKind('application/pdf')).toBe('pdf')
    expect(attachmentPreviewKind('image/svg+xml')).toBe('')
    expect(attachmentPreviewKind('text/html')).toBe('text')
  })

  it('creates data URLs only for valid inert binary preview types', () => {
    expect(attachmentPreviewDataUrl({ contentType: 'image/png', contentBase64: 'AA==' }))
      .toBe('data:image/png;base64,AA==')
    expect(attachmentPreviewDataUrl({ contentType: 'application/pdf', contentBase64: 'JVBERg==' }))
      .toBe('data:application/pdf;base64,JVBERg==')
    expect(attachmentPreviewDataUrl({ contentType: 'image/svg+xml', contentBase64: 'AA==' })).toBe('')
    expect(attachmentPreviewDataUrl({ contentType: 'image/png', contentBase64: '@@' })).toBe('')
  })

  it('decodes bounded UTF-8 text and rejects malformed or oversized content', () => {
    expect(decodeAttachmentText({ contentBase64: btoa('hello') })).toBe('hello')
    expect(() => decodeAttachmentText({ contentBase64: '@@' })).toThrow(/valid base64/)
    expect(() => decodeAttachmentText({ contentBase64: btoa('too large') }, 2)).toThrow(/too large/)
  })
})
