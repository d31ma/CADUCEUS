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

test('explicitly opts in and shows a sanitized public GitHub contact card', async ({ page }) => {
  const domain = `github-card-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  const participant = 'grace@outside.test'
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const alice = await getToken(mailbox)
  await seedDomain(alice.token, domain)
  await deliverEmail({
    recipient: mailbox,
    sender: participant,
    subject: 'Public profile context',
    body: 'This participant has an optional public profile.',
    messageId: `github-card-${Date.now()}`,
  })

  await page.setViewportSize({ width: 1440, height: 900 })
  await useTestApi(page)
  await loginAs(page, alice.token, alice.email, alice.role, alice.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  const preference = page.getByLabel('Public GitHub contact profiles')
  await expect(preference).not.toBeChecked()
  const saved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.githubProfileEnrichment === true)
  await preference.check()
  await saved
  await expect(preference).toBeChecked()

  const persistedResponse = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${alice.token}` },
  })
  expect(persistedResponse.status).toBe(200)
  expect((await persistedResponse.json()).githubProfileEnrichment).toBe(true)

  const profileRequests = []
  await page.route('http://localhost:19876/github/profile?*', async route => {
    profileRequests.push(route.request().url())
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        enabled: true,
        status: 'found',
        profile: {
          login: 'grace-hopper',
          profileUrl: 'https://github.com/grace-hopper',
          repositories: [
            {
              name: 'compiler',
              fullName: 'grace-hopper/compiler',
              url: 'https://github.com/grace-hopper/compiler',
              description: 'A public compiler repository',
              language: 'C++',
              stars: 4242,
            },
          ],
        },
      }),
    })
  })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText('Public profile context', { exact: true }).click()
  await expect(page.locator('.email-body')).toBeVisible()
  await page.getByRole('button', { name: 'Show contact sidebar' }).click()

  const github = page.getByRole('region', { name: 'Public GitHub profile' })
  await expect(github.getByRole('link', { name: '@grace-hopper' })).toHaveAttribute('href', 'https://github.com/grace-hopper')
  await expect(github.getByRole('link', { name: /grace-hopper\/compiler/ })).toHaveAttribute('href', 'https://github.com/grace-hopper/compiler')
  await expect(github.getByText('★ 4242')).toBeVisible()
  await expect(github.getByText('A public compiler repository')).toBeVisible()
  expect(profileRequests).toHaveLength(1)
  expect(new URL(profileRequests[0]).searchParams.get('email')).toBe(participant)
  expect(errors).toEqual([])
})
