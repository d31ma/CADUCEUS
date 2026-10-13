import { test, expect } from '@playwright/test'
import {
  API,
  getToken,
  seedUser,
} from './helpers.mjs'

test.skip(process.env.CADUCEUS_TEST_REAL_TRANSLATION !== 'true', 'Runs the 839.7 MiB checksum-pinned model qualification only on demand.')

test.describe.configure({ mode: 'serial' })

test('qualifies the real self-hosted WASM translation model', async ({ page }) => {
  test.setTimeout(10 * 60_000)
  page.on('console', message => console.log(`[translation-browser:${message.type()}] ${message.text()}`))
  const domain = `translation-runtime-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const account = await getToken(mailbox)
  const original = 'Hola, este mensaje contiene información importante para la reunión de mañana.'

  // WebKit does not honor Playwright's bypassCSP for Tachyon's split-port E2E
  // API. Proxy only this authenticated asset route through the page origin so
  // every browser exercises the same production handler without weakening CSP.
  await page.route('**/__translation_api/**', async route => {
    const requestUrl = new URL(route.request().url())
    const suffix = requestUrl.pathname.slice('/__translation_api'.length) + requestUrl.search
    const response = await route.fetch({ url: `${API}${suffix}` })
    await route.fulfill({ response })
  })

  await page.goto('/')
  await page.waitForFunction(() => typeof window._caduceus?.apiFetch === 'function')
  // The normal translation E2E covers reader/composer behavior with a
  // deterministic engine. This call isolates the production model runtime.
  const results = await page.evaluate(async ({ text, token }) => {
    window._caduceus.apiFetch = (path, init = {}) => {
      if (!String(path).startsWith('/translation-assets')) {
        throw new Error('The runtime qualification permits only translation assets.')
      }
      const headers = new Headers(init.headers)
      headers.set('Authorization', `Bearer ${token}`)
      return fetch(`/__translation_api${path}`, { ...init, headers })
    }
    globalThis.__CADUCEUS_TRANSLATION_DIAGNOSTIC = stage => console.log(`engine-stage:${stage}`)
    const { translateText } = await import('/shared/scripts/on-device-translation.js')
    const english = await translateText(text, {
      sourceLanguage: 'es',
      targetLanguage: 'en',
      onProgress: progress => console.log(`model-progress:${Math.round(progress * 100)}`),
    })
    const finnishSource = 'This message contains important information for tomorrow morning.'
    const finnish = await translateText(finnishSource, {
      sourceLanguage: 'en',
      targetLanguage: 'fi',
    })
    return { english, finnish, finnishSource }
  }, { text: original, token: account.token })
  expect(results.english.sourceLanguage).toBe('es')
  expect(results.english.targetLanguage).toBe('en')
  expect(results.english.text.trim()).not.toBe(original)
  expect(results.english.text.toLowerCase()).toContain('message')
  expect(results.finnish.sourceLanguage).toBe('en')
  expect(results.finnish.targetLanguage).toBe('fi')
  expect(results.finnish.text.trim()).not.toBe(results.finnishSource)
})

test('qualifies both compact OPUS fallback directions in WebAssembly', async ({ page }) => {
  test.setTimeout(10 * 60_000)
  page.on('console', message => console.log(`[translation-browser:${message.type()}] ${message.text()}`))
  page.on('response', response => {
    if (response.url().includes('__translation_api') && !response.ok()) {
      console.log(`[translation-browser:asset-error] ${response.status()} ${response.url()}`)
    }
  })
  const domain = `translation-opus-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const account = await getToken(mailbox)

  await page.route('**/__translation_api/**', async route => {
    const requestUrl = new URL(route.request().url())
    const suffix = requestUrl.pathname.slice('/__translation_api'.length) + requestUrl.search
    const response = await route.fetch({ url: `${API}${suffix}` })
    await route.fulfill({ response })
  })

  await page.goto('/')
  await page.waitForFunction(() => typeof window._caduceus?.apiFetch === 'function')
  const results = await page.evaluate(async token => {
    window._caduceus.apiFetch = (path, init = {}) => {
      if (!String(path).startsWith('/translation-assets')) {
        throw new Error('The runtime qualification permits only translation assets.')
      }
      const headers = new Headers(init.headers)
      headers.set('Authorization', `Bearer ${token}`)
      return fetch(`/__translation_api${path}`, { ...init, headers })
    }
    globalThis.__CADUCEUS_TRANSLATION_DIAGNOSTIC = stage => console.log(`engine-stage:${stage}`)
    const { translateText } = await import('/shared/scripts/on-device-translation.js')
    const targetResults = await Promise.all(['eo', 'eu', 'mt'].map(targetLanguage => translateText(
      'This message contains important information for tomorrow.',
      { sourceLanguage: 'en', targetLanguage },
    )))
    const english = await translateText('Ĉi tiu mesaĝo enhavas gravajn informojn por morgaŭ.', {
      sourceLanguage: 'eo',
      targetLanguage: 'en',
    })
    return { esperanto: targetResults[0], targetResults, english }
  }, account.token)

  expect(results.esperanto.sourceLanguage).toBe('en')
  expect(results.esperanto.targetLanguage).toBe('eo')
  expect(results.esperanto.text.toLowerCase()).toContain('mesaĝ')
  expect(results.targetResults.map(result => result.targetLanguage)).toEqual(['eo', 'eu', 'mt'])
  expect(results.targetResults.every(result => result.text.trim() && !result.text.includes('>>'))).toBe(true)
  expect(results.english.sourceLanguage).toBe('eo')
  expect(results.english.targetLanguage).toBe('en')
  expect(results.english.text.toLowerCase()).toContain('message')
})
