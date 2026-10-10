import { describe, expect, it } from 'bun:test'
import {
  createInlineContentId,
  isNeutralPastedColor,
  isInlineImageFile,
  normalizeComposeLink,
  normalizePlainTextForPaste,
  quotedMessageHTML,
} from '../../client/shared/scripts/rich-compose-editor.js'

describe('rich composer policy', () => {
  it('accepts safe link schemes and rejects active or ambiguous URLs', () => {
    expect(normalizeComposeLink('www.example.com/path')).toBe('https://www.example.com/path')
    expect(normalizeComposeLink('mailto:alice@example.test')).toBe('mailto:alice@example.test')
    expect(normalizeComposeLink('javascript:alert(1)')).toBe('')
    expect(normalizeComposeLink('data:text/html,bad')).toBe('')
  })

  it('limits inline authoring to inert raster formats and creates RFC content ids', () => {
    expect(isInlineImageFile({ type: 'image/png' })).toBe(true)
    expect(isInlineImageFile({ type: 'image/svg+xml' })).toBe(false)
    expect(createInlineContentId('ABC-123')).toBe('caduceus.ABC-123@inline')
  })

  it('escapes untrusted source messages before producing a reply quote', () => {
    const html = quotedMessageHTML({
      sender: '<img src=x onerror=bad()>',
      subject: '<script>bad()</script>',
      receivedAt: '2026-01-01T12:00:00.000Z',
      body: '<b onclick=bad()>not html</b>\nsecond line',
    }, 'forward')
    expect(html).toContain('data-caduceus-quote="forward"')
    expect(html).toContain('&lt;b onclick=bad()&gt;not html&lt;/b&gt;<br>second line')
    expect(html).not.toContain('<script>')
    expect(html).not.toContain('<img')
  })

  it('drops theme-default pasted colors while retaining intentional colors', () => {
    for (const color of ['black', 'white', 'transparent', '#fff', '#1f1f1f', 'rgb(236, 236, 241)', 'rgba(255, 255, 255, .5)']) {
      expect(isNeutralPastedColor(color)).toBe(true)
    }
    for (const color of ['#222222', 'rgb(200, 20, 20)', '#336699', 'currentColor']) {
      expect(isNeutralPastedColor(color)).toBe(false)
    }
  })

  it('normalizes Windows and legacy line endings before plain-text paste', () => {
    expect(normalizePlainTextForPaste('First\r\n\r\nSecond\rThird')).toBe('First\n\nSecond\nThird')
  })
})
