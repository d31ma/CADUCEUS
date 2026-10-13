import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('persists mail preferences and applies them to reader, keyboard, and composer behavior', async ({ page }) => {
  const domain = `mail-prefs-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  const subject = `Manual read ${Date.now()}`
  const swipeSubject = `Swipe policy ${Date.now()}`
  await deliverEmail({ recipient: email, sender: 'sender@outside.test', subject, body: 'Preference behavior.' })
  await deliverEmail({ recipient: email, sender: 'touch@outside.test', subject: swipeSubject, body: 'Swipe behavior.' })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  const section = page.locator('[data-mail-preferences-settings]')
  await expect(page.getByRole('button', { name: 'Save mail preferences' })).toBeEnabled()
  await section.locator('[data-mail-preference="workspaceLayout"]').selectOption('split-vertical')
  await section.locator('[data-mail-preference="interfaceScale"]').selectOption('1.2')
  await section.locator('[data-mail-preference="markAsReadDelay"]').selectOption('-1')
  await section.locator('[data-mail-preference="backspaceAction"]').selectOption('trash')
  await section.locator('[data-mail-preference="conversationOrder"]').selectOption('descending')
  await section.locator('[data-mail-preference="emailRenderMode"]').selectOption('dark')
  await section.locator('[data-mail-preference="defaultReplyAction"]').selectOption('reply-all')
  await section.locator('[data-mail-preference="defaultSendAction"]').selectOption('send-and-archive')
  await section.locator('[data-mail-preference="undoSendDelay"]').selectOption('15000')
  await section.locator('[data-mail-preference="spellcheckLanguage"]').selectOption('en-GB')
  await section.locator('[data-auto-recipient-kind="cc"]').fill('audit@outside.test')
  await expect(section.locator('[data-mail-preference="workspaceLayout"]')).toHaveValue('split-vertical')
  for (const name of ['use24HourClock', 'detailedHeaders', 'detailedNames', 'restrictMessageWidth']) {
    await section.locator(`[data-mail-preference="${name}"]`).check()
    await expect(section.locator('[data-mail-preference="workspaceLayout"]'), `after ${name}`).toHaveValue('split-vertical')
  }
  for (const name of ['swipeEnabled', 'displayAttachmentPreviews', 'richText', 'spellcheck', 'unsnoozeToTop']) {
    await section.locator(`[data-mail-preference="${name}"]`).uncheck()
  }
  await section.locator('[data-find-in-mail-domain]').check()
  const saved = page.waitForResponse(response => response.url().endsWith('/users/me/preferences')
    && response.request().method() === 'PUT')
  await page.getByRole('button', { name: 'Save mail preferences' }).click()
  const savedResponse = await saved
  expect(savedResponse.status()).toBe(200)
  expect(savedResponse.request().postDataJSON()?.mailPreferences).toMatchObject({
    workspaceLayout: 'split-vertical', interfaceScale: 1.2, markAsReadDelay: -1,
    backspaceAction: 'trash', conversationOrder: 'descending', emailRenderMode: 'dark',
    richText: false, spellcheck: false, spellcheckLanguage: 'en-GB',
    defaultSendAction: 'send-and-archive', undoSendDelay: 15000,
    autoCcBySender: { [email]: ['audit@outside.test'] },
    displayAttachmentPreviews: false, findInMailDisabledDomains: [domain],
  })
  await expect(section.getByText('Mail preferences saved.', { exact: true })).toBeVisible()

  await expect(page.locator('html')).toHaveAttribute('data-mail-layout', 'split-vertical')
  await expect(page.locator('html')).toHaveAttribute('data-message-width', 'restricted')
  await expect(page.locator('html')).toHaveAttribute('data-message-theme', 'dark')
  await expect(page.locator('html')).toHaveAttribute('data-rich-composer', 'disabled')
  await page.reload()
  await waitForUiReady(page)
  await expect(section.locator('[data-mail-preference="workspaceLayout"]')).toHaveValue('split-vertical')
  await expect(section.locator('[data-mail-preference="markAsReadDelay"]')).toHaveValue('-1')

  await page.goto('/inbox')
  await waitForUiReady(page)
  const inboxMessage = await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.find(message => message.subject === subject))
  await page.goto(`/email/${inboxMessage.id}`)
  await waitForUiReady(page)
  await expect(page.locator('.email-detailed-headers')).toContainText('Message-ID')
  await page.waitForTimeout(750)
  const stillUnread = await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.find(message => message.subject === subject))
  expect(stillUnread.read).toBe(false)

  await page.getByRole('button', { name: 'Reply', exact: true }).first().click()
  await page.waitForURL(/\/compose\/?$/)
  await waitForUiReady(page)
  await expect(page.locator('[data-compose-to] input')).toHaveValue(`sender@outside.test, ${email}`)
  await expect(page.locator('[data-compose-cc] input')).toHaveValue('audit@outside.test')
  await page.getByRole('button', { name: 'Discard', exact: true }).click()
  await page.waitForURL(/\/inbox\/?$/)

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.locator(`[data-email-open="${inboxMessage.id}"]`).focus()
  await page.keyboard.press('Backspace')
  await expect.poll(async () => fetch(`${API}/inbox?folder=trash`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.some(message => message.subject === subject))).toBe(true)

  const swipeMessage = await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.find(message => message.subject === swipeSubject))
  const swipeRow = page.locator(`[data-email-id="${swipeMessage.id}"]`)
  const dispatchSwipe = (from, to) => swipeRow.evaluate((row, points) => {
    row.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerType: 'touch', pointerId: 41, clientX: points.from, clientY: 20 }))
    row.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, pointerType: 'touch', pointerId: 41, clientX: points.to, clientY: 22 }))
  }, { from, to })
  await dispatchSwipe(220, 120)
  await page.waitForTimeout(150)
  expect(await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.some(message => message.id === swipeMessage.id))).toBe(true)
  // Turn swiping back on the way a user does: in Settings, then back to the inbox.
  await page.goto('/settings')
  await waitForUiReady(page)
  await section.locator('[data-mail-preference="swipeEnabled"]').check()
  const swipeSaved = page.waitForResponse(response => response.url().endsWith('/users/me/preferences')
    && response.request().method() === 'PUT')
  await page.getByRole('button', { name: 'Save mail preferences' }).click()
  expect((await swipeSaved).status()).toBe(200)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.locator('html')).toHaveAttribute('data-mail-swipe', 'enabled')
  await dispatchSwipe(120, 220)
  await expect.poll(async () => fetch(`${API}/inbox?folder=archive`, { headers: { Authorization: `Bearer ${account.token}` } })
    .then(response => response.json())
    .then(messages => messages.some(message => message.id === swipeMessage.id))).toBe(true)
  await page.reload()
  await waitForUiReady(page)

  await page.goto('/compose')
  await waitForUiReady(page)
  await expect(page.locator('.compose-toolbar')).toBeHidden()
  await expect(page.locator('[data-compose-editor]')).toHaveJSProperty('spellcheck', false)
  expect(await page.locator('[data-compose-editor]').evaluate(editor => editor.lang)).toBe('en-GB')

  await page.locator('[data-compose-to] input').fill('recipient@outside.test')
  await page.locator('[data-compose-subject] input').fill('Plain-text preference send')
  await page.locator('[data-compose-editor]').fill('Plain text only.')
  const sendResponsePromise = page.waitForResponse(response => response.url().endsWith('/send') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  const sendResponse = await sendResponsePromise
  expect(sendResponse.status()).toBe(200)
  expect(sendResponse.request().postDataJSON()).toMatchObject({
    to: ['recipient@outside.test'], cc: ['audit@outside.test'], text: 'Plain text only.', delayMs: 15000,
  })
  expect(sendResponse.request().postDataJSON()).not.toHaveProperty('html')
  await page.waitForURL(/\/inbox\/?$/)
  const undo = page.waitForResponse(response => response.url().endsWith('/send/undo') && response.request().method() === 'POST')
  await page.locator('#caduceus-toast-action').click()
  expect((await undo).status()).toBe(200)

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted.mailPreferences).toMatchObject({
    workspaceLayout: 'split-vertical', interfaceScale: 1.2, markAsReadDelay: -1,
    detailedHeaders: true, restrictMessageWidth: true, backspaceAction: 'trash',
    conversationOrder: 'descending', emailRenderMode: 'dark', richText: false,
    spellcheck: false, spellcheckLanguage: 'en-GB', defaultSendAction: 'send-and-archive',
    undoSendDelay: 15000, autoCcBySender: { [email]: ['audit@outside.test'] },
    displayAttachmentPreviews: false, unsnoozeToTop: false, findInMailDisabledDomains: [domain],
  })
  expect(errors).toEqual([])
})
