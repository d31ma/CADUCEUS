import { describe, expect, it } from 'bun:test'
import { FontPreferencesUtils } from '../../client/shared/utils/FontPreferencesUtils.js'
import { FontPreferences } from '../../server/shared/users/FontPreferences.js'

const COMPOSER_FONT_OPTIONS = FontPreferencesUtils.composerFontOptions()
const DEFAULT_COMPOSER_FONT = FontPreferencesUtils.defaultComposerFont()
const DEFAULT_INTERFACE_FONT = FontPreferencesUtils.defaultInterfaceFont()
const INTERFACE_FONT_OPTIONS = FontPreferencesUtils.interfaceFontOptions()
const composerFontStack = (...args) => FontPreferencesUtils.composerFontStack(...args)
const interfaceFontStack = (...args) => FontPreferencesUtils.interfaceFontStack(...args)
const normalizeComposerFont = (...args) => FontPreferencesUtils.normalizeComposerFont(...args)
const normalizeInterfaceFont = (...args) => FontPreferencesUtils.normalizeInterfaceFont(...args)

describe('font preferences', () => {
  it('keeps the browser and persistence catalogs in lockstep', () => {
    expect(INTERFACE_FONT_OPTIONS.map(option => option.id)).toEqual([...FontPreferences.INTERFACE_FONT_IDS])
    expect(COMPOSER_FONT_OPTIONS.map(option => option.id)).toEqual([...FontPreferences.COMPOSER_FONT_IDS])
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
