import { afterEach, describe, expect, it } from 'bun:test'
import {
  INTERFACE_LOCALE_IDS,
  normalizeInterfaceLocale as normalizeServerLocale,
  validateInterfaceLocale,
} from '../../server/shared/users/locale-preferences.js'
import {
  INTERFACE_LOCALE_OPTIONS,
  applyInterfaceLocale,
  formatDate,
  formatNumber,
  normalizeInterfaceLocale,
  resolveInterfaceLocale,
  translate,
  validateInterfaceLiteralAsset,
} from '../../client/shared/scripts/localization.js'
import {
  firstTranslation,
  safeGeneratedTranslation,
} from '../../scripts/publish-interface-locales.mjs'

const originalDocument = globalThis.document
const originalStorage = globalThis.localStorage

afterEach(() => {
  globalThis.document = originalDocument
  globalThis.localStorage = originalStorage
})

describe('interface localization', () => {
  it('keeps the browser and persistence locale catalogs in lockstep', () => {
    expect(INTERFACE_LOCALE_OPTIONS.map(option => option.id)).toEqual([...INTERFACE_LOCALE_IDS])
  })

  it('normalizes and validates only fixed catalog identifiers', () => {
    for (const value of ['auto', 'en', 'de', 'es', 'es-419', 'fr', 'ko', 'pl', 'ja', 'zh-CN', 'zh-TW', 'pt', 'pt-BR', 'gl', 'tr', 'ru', 'cs', 'uk', 'hu', 'he', 'ar']) {
      expect(normalizeInterfaceLocale(value)).toBe(value)
      expect(normalizeServerLocale(value)).toBe(value)
      expect(validateInterfaceLocale(value)).toEqual({ ok: true, locale: value })
    }
    for (const value of ['fr-CA', '../ar', 'en<script>', 42, null]) {
      expect(normalizeInterfaceLocale(value)).toBe('auto')
      expect(normalizeServerLocale(value)).toBe('auto')
      expect(validateInterfaceLocale(value).ok).toBe(false)
    }
  })

  it('resolves automatic language negotiation with an English fallback', () => {
    expect(resolveInterfaceLocale('auto', ['fr-CA', 'en-US'])).toBe('fr')
    expect(resolveInterfaceLocale('auto', ['ar-EG'])).toBe('ar')
    expect(resolveInterfaceLocale('auto', ['de-DE', 'en'])).toBe('de')
    expect(resolveInterfaceLocale('auto', ['pt-BR'])).toBe('pt-BR')
    expect(resolveInterfaceLocale('auto', ['zh-TW'])).toBe('zh-TW')
    expect(resolveInterfaceLocale('auto', ['eo', 'mt'])).toBe('en')
    expect(resolveInterfaceLocale('fr', ['ar'])).toBe('fr')
  })

  it('translates catalog messages, interpolates values, and pluralizes safely', () => {
    expect(translate('shell.inbox', {}, 'fr')).toBe('Boîte de réception')
    expect(translate('shell.inbox', {}, 'ar')).toBe('الوارد')
    expect(translate('notification.count', { count: 1 }, 'en')).toBe('1 notification')
    expect(translate('notification.count', { count: 3 }, 'en')).toBe('3 notifications')
    expect(translate('notification.count', { count: 2 }, 'fr')).toBe('2 notifications')
    expect(translate('missing.message', { unsafe: '<script>' }, 'fr')).toBe('missing.message')
  })

  it('applies language and direction without accepting caller-defined DOM values', () => {
    const attrs = new Map()
    globalThis.document = {
      documentElement: { setAttribute: (name, value) => attrs.set(name, value) },
    }
    globalThis.localStorage = { setItem() {}, getItem() { return null } }

    expect(applyInterfaceLocale('ar')).toMatchObject({ choice: 'ar', locale: 'ar', direction: 'rtl' })
    expect(Object.fromEntries(attrs)).toEqual({
      lang: 'ar', dir: 'rtl', 'data-interface-locale': 'ar', 'data-interface-locale-choice': 'ar',
    })

    expect(applyInterfaceLocale('he')).toMatchObject({ choice: 'he', locale: 'he', direction: 'rtl' })
  })

  it('uses the selected locale for Intl number and date formatting', () => {
    expect(formatNumber(1234.5, {}, 'fr')).toContain(',')
    expect(formatDate('2026-08-21T12:00:00Z', { timeZone: 'UTC', dateStyle: 'long' }, 'en'))
      .toContain('August')
    expect(formatDate('not-a-date', {}, 'fr')).toBe('')
  })

  it('rejects incomplete generated catalog assets before they can define literals', async () => {
    expect(() => validateInterfaceLiteralAsset({ schemaVersion: 1, inventory: [], catalogs: {} }))
      .toThrow('invalid schema or inventory size')
    const asset = await Bun.file('client/shared/assets/interface-locales.json').json()
    expect(validateInterfaceLiteralAsset(asset).inventory.length).toBeGreaterThanOrEqual(500)
    expect(asset.catalogs.de.Cancel).toBe('Abbrechen')
    expect(asset.catalogs.he.Archive).toBe('ארכיון')
    expect(asset.catalogs.ja.Save).toBe('保存')
    expect(asset.catalogs.ko.Compose).toBe('작성')
    expect(asset.catalogs['zh-TW']['Account settings']).toBe('帳戶設定')
  })

  it('extracts one bounded translation without publishing decoder continuation', () => {
    expect(firstTranslation('Actions', 'Les actions.')).toBe('Les actions')
    expect(firstTranslation('Save', 'Sauver !')).toBe('Sauver')
    expect(firstTranslation('Write your message…', 'Écrire votre message.')).toBe('Écrire votre message…')
    expect(firstTranslation('Actions', 'Aktionen Aktivitäten Handlungen und die Action')).toBe('Aktionen')
    expect(safeGeneratedTranslation('Actions', 'الإجراءات.', 'ar')).toBe('الإجراءات')
    expect(safeGeneratedTranslation('Undo {action}', 'Annuler.', 'fr')).toBe('')
  })
})
