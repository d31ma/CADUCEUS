import { describe, expect, it } from 'bun:test'
import {
  UnreadBadgeController,
  formatUnreadTitle,
  normalizeUnreadBadgePreference,
  normalizeUnreadCount,
  resolveUnreadBadgeCount,
  stripUnreadTitle,
} from '../../client/shared/scripts/unread-badge.js'

function harness({ title = '', screenshot = false, navigatorObject = {} } = {}) {
  const windowObject = new EventTarget()
  const documentObject = {
    title,
    documentElement: {
      getAttribute: name => name === 'data-screenshot-mode' && screenshot ? 'true' : null,
    },
  }
  const controller = new UnreadBadgeController({
    windowObject,
    documentObject,
    navigatorObject,
  }).install()
  return {
    controller,
    documentObject,
    windowObject,
    setScreenshot(value) { screenshot = value },
  }
}

describe('unread badge primitives', () => {
  it('normalizes counts and preserves one stable title prefix', () => {
    expect(normalizeUnreadCount(4.9)).toBe(4)
    expect(normalizeUnreadCount(-2)).toBe(0)
    expect(normalizeUnreadCount('7')).toBe(7)
    expect(normalizeUnreadCount('nope')).toBe(0)
    expect(stripUnreadTitle('(12) Inbox — CADUCEUS')).toBe('Inbox — CADUCEUS')
    expect(formatUnreadTitle('(2) Inbox — CADUCEUS', 5)).toBe('(5) Inbox — CADUCEUS')
    expect(formatUnreadTitle('', 0)).toBe('CADUCEUS')
  })

  it('normalizes account scope against assigned domains and resolves its exact count', () => {
    const domains = ['Personal.TEST', 'work.test']
    expect(normalizeUnreadBadgePreference({}, domains)).toEqual({ scope: 'all', domain: 'personal.test' })
    expect(normalizeUnreadBadgePreference({ scope: 'domain', domain: 'WORK.TEST' }, domains))
      .toEqual({ scope: 'domain', domain: 'work.test' })
    expect(normalizeUnreadBadgePreference({ scope: 'domain', domain: 'foreign.test' }, domains))
      .toEqual({ scope: 'all', domain: 'personal.test' })
    expect(resolveUnreadBadgeCount({
      unreadInbox: 7,
      unreadInboxByDomain: { 'personal.test': 2, 'work.test': 5 },
    }, { scope: 'all', domain: 'personal.test' })).toBe(7)
    expect(resolveUnreadBadgeCount({
      unreadInbox: 7,
      unreadInboxByDomain: { 'personal.test': 2, 'work.test': 5 },
    }, { scope: 'domain', domain: 'work.test' })).toBe(5)
    expect(resolveUnreadBadgeCount({ unreadInbox: 7, unreadInboxByDomain: {} }, {
      scope: 'domain',
      domain: 'work.test',
    })).toBe(0)
  })

  it('uses the native Badging API and clears it at zero without changing the title', async () => {
    const calls = []
    const { controller, documentObject } = harness({
      title: 'Inbox — CADUCEUS',
      navigatorObject: {
        setAppBadge: async count => { calls.push(['set', count]) },
        clearAppBadge: async () => { calls.push(['clear']) },
      },
    })
    await controller.set(3)
    await controller.clear()
    expect(calls).toEqual([['set', 3], ['clear']])
    expect(documentObject.title).toBe('Inbox — CADUCEUS')
  })

  it('falls back to the document title when native badging is absent or rejects', async () => {
    const missing = harness({ title: 'Inbox — CADUCEUS' })
    await missing.controller.set(3)
    expect(missing.documentObject.title).toBe('(3) Inbox — CADUCEUS')
    await missing.controller.set(1)
    expect(missing.documentObject.title).toBe('(1) Inbox — CADUCEUS')
    await missing.controller.clear()
    expect(missing.documentObject.title).toBe('Inbox — CADUCEUS')

    const rejected = harness({
      navigatorObject: { setAppBadge: async () => { throw new Error('not installed') } },
    })
    await rejected.controller.set(2)
    expect(rejected.documentObject.title).toBe('(2) CADUCEUS')
  })

  it('defers the fallback while screenshot mode masks the title', async () => {
    const state = harness({ title: 'CADUCEUS', screenshot: true })
    await state.controller.set(6)
    expect(state.documentObject.title).toBe('CADUCEUS')
    state.setScreenshot(false)
    state.windowObject.dispatchEvent(new CustomEvent('caduceus:screenshot-mode', { detail: { enabled: false } }))
    expect(state.documentObject.title).toBe('(6) CADUCEUS')
  })

  it('consumes authoritative folder events and mirrors them to the active worker', async () => {
    const posted = []
    const state = harness({
      navigatorObject: { serviceWorker: { controller: { postMessage: value => posted.push(value) } } },
    })
    state.windowObject.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
      detail: { counts: { inbox: 9 }, unreadInbox: 4 },
    }))
    await Promise.resolve()
    expect(state.controller.count).toBe(4)
    expect(posted).toEqual([{ type: 'caduceus:set-badge', count: 4 }])
    expect(state.documentObject.title).toBe('(4) CADUCEUS')
  })

  it('recomputes the current snapshot when the persisted account scope changes', async () => {
    const state = harness()
    state.windowObject.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
      detail: {
        unreadInbox: 6,
        unreadInboxByDomain: { 'personal.test': 2, 'work.test': 4 },
      },
    }))
    await Promise.resolve()
    expect(state.controller.count).toBe(6)

    state.windowObject.dispatchEvent(new CustomEvent('caduceus:unread-badge-preference', {
      detail: { scope: 'domain', domain: 'personal.test', domains: ['personal.test', 'work.test'] },
    }))
    await Promise.resolve()
    expect(state.controller.preference).toEqual({ scope: 'domain', domain: 'personal.test' })
    expect(state.controller.count).toBe(2)
    expect(state.documentObject.title).toBe('(2) CADUCEUS')
  })
})
