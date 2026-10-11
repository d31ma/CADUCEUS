/**
 * Shared helpers for Playwright E2E tests.
 * Handles: test server seeding and page setup.
 */
import { createHmac } from 'node:crypto'

export const API = 'http://localhost:19875'
const INBOUND_WEBHOOK_SECRET = 'caduceus-e2e-inbound-secret'

// ── Unique test email ─────────────────────────────────────────────────────────

let _counter = 0
const _tokens = new Map()

/** Returns a unique email address for each call — keeps tests independent. */
export function uniqueEmail() {
  return `test-${Date.now()}-${++_counter}@example.com`
}

// ── Test server seed helpers ──────────────────────────────────────────────────

async function post(path, data) {
  const headers = { 'Content-Type': 'application/json' }
  if (path === '/inbound/webhook') {
    const body = JSON.stringify(data ?? {})
    headers['X-Caduceus-Signature'] = createHmac('sha256', INBOUND_WEBHOOK_SECRET).update(body).digest('hex')
  }
  const res = await fetch(`${API}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Seed ${path} failed: ${await res.text()}`)
  return res.json()
}

/**
 * Creates a user in the test backend.
 */
export async function seedUser(email, { domains = ['example.com'], role = 'admin' } = {}) {
  const result = await post('/test/seed/user', { email, domains, role })
  _tokens.set(email, result)
  return result
}

export async function seedDomain(token, domain = 'example.com') {
  const res = await fetch(`${API}/domains`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      domain,
      routes: [{ id: `route-${Date.now()}`, match: `*@${domain}`, action: { type: 'store' }, enabled: true }],
      inboundEnabled: true,
    }),
  })
  if (!res.ok) throw new Error(`Seed domain failed: ${await res.text()}`)
  return res.json()
}

export async function deliverEmail({ recipient, sender = 'sender@other.com', subject, body = '', htmlBody, messageId }) {
  const mimeBody = htmlBody
    ? [
      'MIME-Version: 1.0',
      'Content-Type: text/html; charset=utf-8',
      'Content-Transfer-Encoding: 8bit',
      '',
      htmlBody,
    ].join('\r\n')
    : body
  return post('/inbound/webhook', { recipient, sender, subject, body: mimeBody, messageId })
}

/** Record a recipient signal through a production-excluded, authenticated test route. */
export async function seedTrackingEvent(token, data) {
  const res = await fetch(`${API}/test/seed/tracking`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Seed tracking event failed: ${await res.text()}`)
  return res.json()
}

// ── Auth helpers ─────────────────────────────────────────────────────────────

/**
 * Gets the token the test-only seed route obtained by performing a real login.
 */
export async function getToken(email) {
  const data = _tokens.get(email) || await seedUser(email)
  if (!data.token) throw new Error(`getToken failed: ${JSON.stringify(data)}`)
  return data
}

// ── Page setup ────────────────────────────────────────────────────────────────

/**
 * Intercepts /assets/config.js to point the app at the local test server.
 * Must be called before page.goto().
 */
export async function useTestApi(page) {
  await page.addInitScript(() => {
    window.__CADUCEUS_DISABLE_SW = true
  })
  await page.addInitScript(api => {
    window.CADUCEUS_CONFIG = { apiUrl: api }
  }, API)
  await page.route('http://localhost:19875/**', async route => {
    if (route.request().resourceType() !== 'document') return route.fallback()
    const response = await route.fetch()
    const headers = { ...response.headers() }
    const policy = headers['content-security-policy'] || "default-src 'self'"
    headers['content-security-policy'] = `${policy}; connect-src 'self' ${API}`
    await route.fulfill({ response, headers })
  })
  await page.route('**/assets/config.js', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/javascript',
      body: `window.CADUCEUS_CONFIG={apiUrl:"${API}"};`,
    })
  )
}

/**
 * Seeds sessionStorage to simulate a logged-in user, bypassing the login flow.
 * Must be called before page.goto().
 */
export async function loginAs(page, token, email, role = 'admin', domains = ['example.com']) {
  await page.addInitScript(({ token, email, role, domains }) => {
    // Authentication belongs to the top-level application document. Opaque
    // extension sandboxes intentionally have no storage origin.
    if (window.top !== window) return
    if (sessionStorage.getItem('caduceus_e2e_session_seeded')) return
    sessionStorage.setItem('caduceus_token', token)
    sessionStorage.setItem('caduceus_email', email)
    sessionStorage.setItem('caduceus_role', role)
    sessionStorage.setItem('caduceus_domains', JSON.stringify(domains))
    sessionStorage.setItem('caduceus_e2e_session_seeded', 'true')
  }, { token, email, role, domains })
}

/**
 * Waits for Tachyon content, DuVay custom-element upgrades, fonts, and two
 * stable animation frames. This is the single readiness boundary for UI tests.
 */
export async function waitForUiReady(page, { shell = true } = {}) {
  if (shell) await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15_000 })

  await page.evaluate(async () => {
    await window.__caduceusDuvayReady

    const tags = [...new Set(
      Array.from(document.querySelectorAll('*'), element => element.localName)
        .filter(name => name.startsWith('w-'))
    )]
    await Promise.all(tags.map(tag => customElements.whenDefined(tag)))
    if (document.fonts?.ready) await document.fonts.ready
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))
  })
}

/** Collects actionable browser errors for an entire test. */
export function captureUiErrors(page) {
  const errors = []
  page.on('pageerror', error => {
    // Chrome rejects an interrupted native cross-document view transition
    // with this message. Tachyon opts routes into that progressive platform
    // feature; the navigation still completes and no application exception
    // occurred, so keep it out of the UI error ledger.
    if (error.message === 'Transition was skipped') return
    errors.push(`pageerror: ${error.message}`)
  })
  page.on('console', message => {
    if (message.type() !== 'error') return
    const value = message.text()
    if (/favicon\.ico|Failed to load resource.*\/sw\.js/.test(value)) return
    // WebKit does not recognize Chromium's progressive speculation-rules CSP
    // token. It ignores only that token and enforces the rest of the policy.
    if (/invalid source: ''inline-speculation-rules''/.test(value)) return
    errors.push(`console: ${value}`)
  })
  return errors
}

/**
 * Returns a component/layout fingerprint used to catch a control changing
 * shape after hydration or after an unrelated interaction.
 */
export async function uiFingerprint(page, selector = 'body') {
  return page.locator(selector).evaluate(root => {
    const round = value => Math.round(value * 10) / 10
    const visible = element => {
      const style = getComputedStyle(element)
      const rect = element.getBoundingClientRect()
      return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0
    }

    return Array.from(root.querySelectorAll('w-btn,w-text-field,w-textarea,w-select,w-chip,button,a[href],[role="button"]'))
      .filter(visible)
      .map(element => {
        const rect = element.getBoundingClientRect()
        return [
          element.localName,
          element.getAttribute('aria-label') || element.textContent?.trim().replace(/\s+/g, ' ').slice(0, 48) || '',
          round(rect.x), round(rect.y), round(rect.width), round(rect.height),
        ].join('|')
      })
  })
}

/** Fails when DuVay hosts are undefined or render no usable child control. */
export async function expectDuVayIntegrity(page, expect) {
  const failures = await page.evaluate(() => Array.from(document.querySelectorAll('*'))
    .filter(element => element.localName.startsWith('w-'))
    .flatMap(element => {
      if (!customElements.get(element.localName)) return [`${element.localName}: undefined`]
      if (!element.matches('w-btn,w-text-field,w-textarea,w-select')) return []
      if (!element.querySelector('button,input,textarea,select')) return [`${element.localName}: missing native control`]
      return []
    }))
  expect(failures).toEqual([])
}
