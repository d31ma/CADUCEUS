import { describe, expect, it } from 'bun:test'
import {
  COMPOSER_FONT_OPTIONS,
  DEFAULT_COMPOSER_FONT,
  DEFAULT_INTERFACE_FONT,
  INTERFACE_FONT_OPTIONS,
  composerFontStack,
  interfaceFontStack,
  normalizeComposerFont,
  normalizeInterfaceFont,
} from '../../client/shared/scripts/font-preferences.js'
import {
  COMPOSER_FONT_IDS,
  INTERFACE_FONT_IDS,
} from '../../server/features/users/services/font-preferences.js'

describe('font preferences', () => {
  it('keeps the browser and persistence catalogs in lockstep', () => {
    expect(INTERFACE_FONT_OPTIONS.map(option => option.id)).toEqual([...INTERFACE_FONT_IDS])
    expect(COMPOSER_FONT_OPTIONS.map(option => option.id)).toEqual([...COMPOSER_FONT_IDS])
  })

  it('fails closed to local defaults for unknown and hostile values', () => {
    expect(normalizeInterfaceFont('url(https://fonts.example.test/leak.woff2)')).toBe(DEFAULT_INTERFACE_FONT)
    expect(normalizeInterfaceFont('serif; color:red')).toBe(DEFAULT_INTERFACE_FONT)
    expect(normalizeComposerFont('Local Font')).toBe(DEFAULT_COMPOSER_FONT)
  })

  it('contains only fixed local/system stacks and lets composer match the interface', () => {
    for (const option of [...INTERFACE_FONT_OPTIONS, ...COMPOSER_FONT_OPTIONS]) {
      expect(option.stack).not.toMatch(/url\s*\(|https?:|@import/i)
    }
    expect(interfaceFontStack('serif')).toContain('Charter')
    expect(composerFontStack('monospace')).toContain('IBM Plex Mono')
    expect(composerFontStack('interface')).toBe('var(--caduceus-interface-font)')
  })
})
