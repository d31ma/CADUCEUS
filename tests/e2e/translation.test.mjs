import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

async function installTranslationStub(page) {
  await page.addInitScript(() => {
    delete globalThis.LanguageDetector
    delete globalThis.Translator
    globalThis.__CADUCEUS_OFFLINE_TRANSLATION_FACTORY = async ({ sourceLanguage, targetLanguage, onProgress }) => {
      onProgress?.(1)
      return {
        translate: async text => `[${sourceLanguage}→${targetLanguage}] ${text}`,
        destroy() {},
      }
    }
  })
}

test('translates received HTML and composer drafts on-device without changing protected content', async ({ page }) => {
  const errors = captureUiErrors(page)
  const domain = `translation-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const account = await getToken(mailbox)
  await seedDomain(account.token, domain)
  const messageId = `translation-${Date.now()}`
  await deliverEmail({
    recipient: mailbox,
    sender: 'carlos@example.test',
    subject: 'Mensaje para traducir',
    messageId,
    htmlBody: '<p>Hola, este mensaje contiene información importante.</p><p>https://example.test/reset?token=secret</p><p>Office, San Francisco, CA 94114</p><p style="font-family: monospace">API_TOKEN_VALUE</p><p style="font-size: 10px">Manage notification preferences</p><blockquote>No traduzcas esta cita.</blockquote>',
  })

  await useTestApi(page)
  await installTranslationStub(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.locator(`[data-email-id="${messageId}"] .email-avatar`).click()

  const readerTarget = page.getByLabel('Message translation').getByRole('combobox', { name: 'Translate to' })
  await expect(readerTarget.locator('option')).toHaveCount(93)
  await expect(readerTarget).toHaveValue('en')
  await page.getByLabel('Message translation').getByRole('button', { name: 'Translate', exact: true }).click()
  await expect(page.getByText('Translated from Spanish to English on this device.')).toBeVisible()
  const frame = page.frameLocator('iframe[title="Email HTML body"]')
  await expect(frame.getByText('[es→en] Hola, este mensaje contiene información importante.', { exact: true })).toBeVisible()
  await expect(frame.getByText('https://example.test/reset?token=secret', { exact: true })).toBeVisible()
  await expect(frame.getByText('Office, San Francisco, CA 94114', { exact: true })).toBeVisible()
  await expect(frame.getByText('API_TOKEN_VALUE', { exact: true })).toBeVisible()
  await expect(frame.getByText('Manage notification preferences', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Show quoted content' }).click()
  await expect(frame.getByText('No traduzcas esta cita.', { exact: true })).toBeVisible()
  await expect(frame.getByText('[es→en] No traduzcas esta cita.', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Show original' }).click()
  await expect(frame.getByText('Hola, este mensaje contiene información importante.', { exact: true })).toBeVisible()
  await page.reload()
  await waitForUiReady(page)
  await page.locator(`[data-email-id="${messageId}"] .email-avatar`).click()
  await expect(page.getByLabel('Message translation').getByRole('button', { name: 'Show translation' })).toBeVisible()
  await page.getByLabel('Message translation').getByRole('button', { name: 'Show translation' }).click()
  await expect(frame.getByText('[es→en] Hola, este mensaje contiene información importante.', { exact: true })).toBeVisible()

  await page.goto('/compose')
  await waitForUiReady(page)
  const editor = page.locator('[data-compose-editor]')
  await editor.fill('Hello, this draft contains enough text to detect reliably.')
  const composerTarget = page.getByLabel('Translate draft to')
  await expect(composerTarget.locator('option')).toHaveCount(93)
  await expect(composerTarget.locator('option[value="mrj"]')).toHaveText('Hill Mari')
  await composerTarget.selectOption('fr')
  await expect(editor).toHaveText('Hello, this draft contains enough text to detect reliably.')
  await page.getByRole('button', { name: 'Translate draft on this device' }).click()
  expect(errors).toEqual([])
  await expect(page.locator('[data-translation-status]')).toContainText('Translated from English to French.')
  await expect(editor).toHaveText('[en→fr] Hello, this draft contains enough text to detect reliably.')

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('button', { name: 'Translate draft on this device' })).toBeVisible()
  expect(errors).toEqual([])
})
