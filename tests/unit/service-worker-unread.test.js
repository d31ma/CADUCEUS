import { describe, expect, it } from 'bun:test'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../../client/shared/service-worker/unread.js', import.meta.url), 'utf8')

function runtime({ badgeSupported = true, existingNotifications = [] } = {}) {
  const handlers = new Map()
  const badgeCalls = []
  const notifications = []
  const clients = []
  const lifecycle = []
  const self = {
    location: { origin: 'https://mail.example.test' },
    navigator: badgeSupported ? {
      setAppBadge: async count => { badgeCalls.push(['set', count]) },
      clearAppBadge: async () => { badgeCalls.push(['clear']) },
    } : {},
    registration: {
      showNotification: async (title, options) => { notifications.push({ title, options }) },
      getNotifications: async ({ tag }) => existingNotifications.filter(notification => notification.tag === tag),
    },
    clients: {
      matchAll: async () => clients,
      openWindow: async url => ({ url }),
      claim: async () => { lifecycle.push('claim') },
    },
    skipWaiting: async () => { lifecycle.push('skipWaiting') },
    addEventListener: (name, handler) => handlers.set(name, handler),
  }
  new Function('self', source)(self)
  return { handlers, badgeCalls, notifications, clients, existingNotifications, lifecycle }
}

async function dispatch(handler, event) {
  const waits = []
  handler({ ...event, waitUntil: promise => waits.push(promise) })
  await Promise.all(waits)
}

describe('unread service-worker extension', () => {
  it('activates immediately and claims its clients as a separately registered worker', async () => {
    const state = runtime()
    state.handlers.get('install')({})
    await dispatch(state.handlers.get('activate'), {})
    expect(state.lifecycle).toEqual(['skipWaiting', 'claim'])
  })

  it('shows new mail and applies its authoritative unread count', async () => {
    const state = runtime()
    await dispatch(state.handlers.get('push'), {
      data: { json: () => ({
        title: 'New mail from sender@example.test',
        body: 'Quarterly review',
        tag: 'mail:email-1',
        actions: [{ action: 'mark-read', title: 'Mark read' }, { action: 'evil', title: 'Do evil' }],
        silent: true,
        badgeCount: 8,
        data: { url: '/email/email-1', emailId: 'email-1' },
      }) },
    })
    expect(state.badgeCalls).toEqual([['set', 8]])
    expect(state.notifications).toHaveLength(1)
    expect(state.notifications[0]).toMatchObject({
      title: 'New mail from sender@example.test',
      options: {
        body: 'Quarterly review',
        tag: 'mail:email-1',
        data: { url: '/email/email-1', emailId: 'email-1' },
        actions: [{ action: 'mark-read', title: 'Mark read' }],
        silent: true,
        renotify: false,
      },
    })
  })

  it('closes an existing notification with the same delivery tag before showing its retry', async () => {
    let closed = false
    const state = runtime({
      existingNotifications: [{ tag: 'mail:email-1', close: () => { closed = true } }],
    })
    await dispatch(state.handlers.get('push'), {
      data: { json: () => ({ title: 'New mail', tag: 'mail:email-1' }) },
    })
    expect(closed).toBe(true)
    expect(state.notifications).toHaveLength(1)
  })

  it('uses a generic platform badge if the server count is temporarily unavailable', async () => {
    const state = runtime()
    await dispatch(state.handlers.get('push'), {
      data: { json: () => ({ title: 'New mail' }) },
    })
    expect(state.badgeCalls).toEqual([['set', undefined]])
    expect(state.notifications).toHaveLength(1)
  })

  it('accepts foreground reconciliation messages and acknowledges normalized state', async () => {
    const state = runtime()
    const replies = []
    await dispatch(state.handlers.get('message'), {
      data: { type: 'caduceus:set-badge', count: 3.9 },
      source: { postMessage: value => replies.push(value) },
    })
    expect(state.badgeCalls).toEqual([['set', 3]])
    expect(replies).toEqual([{ type: 'caduceus:badge-applied', count: 3 }])
  })

  it('asks every open app client to flush encrypted offline writes on Background Sync', async () => {
    const state = runtime({ badgeSupported: false })
    const messages = []
    state.clients.push({ url: 'https://mail.example.test/inbox', postMessage: value => messages.push(value) })
    await dispatch(state.handlers.get('sync'), { tag: 'caduceus-offline-writes' })
    expect(messages).toEqual([{ type: 'caduceus:flush-offline-writes' }])
  })

  it('focuses an existing app window at the notification deep link', async () => {
    const state = runtime({ badgeSupported: false })
    const navigated = []
    const messages = []
    let focused = false
    // The push worker does not control the page, so it asks the page to navigate
    // ('caduceus:navigate') instead of calling WindowClient.navigate().
    state.clients.push({
      url: 'https://mail.example.test/inbox',
      postMessage: message => messages.push(message),
      navigate: async url => { navigated.push(url) },
      focus: async () => { focused = true },
    })
    let closed = false
    await dispatch(state.handlers.get('notificationclick'), {
      notification: {
        data: { url: '/email/email-1' },
        close: () => { closed = true },
      },
    })
    expect(closed).toBe(true)
    expect(messages).toEqual([{ type: 'caduceus:navigate', url: 'https://mail.example.test/email/email-1' }])
    expect(navigated).toEqual([])
    expect(focused).toBe(true)
  })

  it('refuses a cross-origin notification target', async () => {
    const state = runtime({ badgeSupported: false })
    const navigated = []
    const messages = []
    state.clients.push({
      url: 'https://mail.example.test/settings',
      postMessage: message => messages.push(message),
      navigate: async url => { navigated.push(url) },
      focus: async () => {},
    })
    await dispatch(state.handlers.get('notificationclick'), {
      notification: {
        data: { url: 'https://attacker.test/phish' },
        close: () => {},
      },
    })
    expect(messages).toEqual([{ type: 'caduceus:navigate', url: 'https://mail.example.test/inbox' }])
    expect(navigated).toEqual([])
  })

  it('forwards an allow-listed action to an authenticated app window without navigating it', async () => {
    const state = runtime({ badgeSupported: false })
    const messages = []
    const navigated = []
    let focused = false
    state.clients.push({
      url: 'https://mail.example.test/settings',
      postMessage: message => messages.push(message),
      navigate: async url => { navigated.push(url) },
      focus: async () => { focused = true },
    })
    await dispatch(state.handlers.get('notificationclick'), {
      action: 'archive',
      notification: {
        data: { url: '/email/email-1', emailId: 'email-1' },
        close: () => {},
      },
    })
    expect(messages).toEqual([{
      type: 'caduceus:notification-action',
      action: 'archive',
      emailId: 'email-1',
    }])
    expect(navigated).toEqual([])
    expect(focused).toBe(true)
  })
})
