import { test, expect } from '@playwright/test'
import { captureUiErrors, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

function localDateTimeValue(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

test('schedules, inspects, edits, reschedules, and cancels a message', async ({ page }) => {
  test.setTimeout(60_000)
  const domain = `send-later-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  const subject = `Scheduled browser ${Date.now()}`
  await page.goto('/compose')
  await waitForUiReady(page)
  await page.locator('[data-compose-to] input').fill('recipient@outside.test')
  await page.locator('[data-compose-subject] input').fill(subject)
  await page.locator('[data-compose-editor]').fill('Original scheduled body')

  await page.getByRole('button', { name: 'Send later', exact: true }).click()
  const picker = page.getByRole('dialog', { name: 'Send later' })
  await expect(picker).toBeVisible()
  await expect(picker).toContainText(Intl.DateTimeFormat().resolvedOptions().timeZone)
  await expect(picker.getByRole('button', { name: 'Tomorrow, 8:00 AM' })).toBeVisible()
  const createResponse = page.waitForResponse(response => response.url().endsWith('/send') && response.request().method() === 'POST')
  await picker.getByRole('button', { name: 'Tomorrow, 8:00 AM' }).click()
  expect((await createResponse).status()).toBe(200)
  await page.waitForURL(/\/scheduled\/?$/)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()
  await expect(page.getByText('Queued', { exact: true })).toBeVisible()

  const detailResponse = page.waitForResponse(response => /\/send\/scheduled\/scheduled-/.test(response.url()) && response.request().method() === 'GET')
  await page.locator('.scheduled-item').filter({ hasText: subject }).getByRole('button', { name: 'Edit' }).click()
  expect((await detailResponse).status()).toBe(200)
  await page.waitForURL(/\/compose\/?$/)
  await waitForUiReady(page)
  await expect(page.getByText('Edit scheduled message', { exact: true })).toBeVisible()
  await expect(page.locator('[data-compose-subject] input')).toHaveValue(subject)
  await expect(page.locator('[data-compose-editor]')).toContainText('Original scheduled body')
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toHaveCount(0)

  const updatedSubject = `${subject} updated`
  await page.locator('[data-compose-subject] input').fill(updatedSubject)
  await page.getByRole('button', { name: 'Update schedule', exact: true }).click()
  const updatePicker = page.getByRole('dialog', { name: 'Update delivery time' })
  await expect(updatePicker).toBeVisible()
  const later = new Date(Date.now() + 2 * 60 * 60_000)
  await updatePicker.locator('[data-send-later-custom]').fill(localDateTimeValue(later))
  const updateResponse = page.waitForResponse(response => /\/send\/scheduled\/scheduled-/.test(response.url()) && response.request().method() === 'PUT')
  await updatePicker.getByRole('button', { name: 'Update schedule', exact: true }).click()
  const updated = await updateResponse
  expect(updated.status()).toBe(200)
  await page.waitForURL(/\/scheduled\/?$/)
  await expect(page.getByText(updatedSubject, { exact: true })).toBeVisible()

  page.once('dialog', dialog => dialog.accept())
  await page.locator('.scheduled-item').filter({ hasText: updatedSubject }).getByRole('button', { name: 'Cancel' }).click()
  await expect(page.getByText(updatedSubject, { exact: true })).toHaveCount(0)
  expect(errors).toEqual([])
})
