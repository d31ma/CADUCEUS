// @ts-nocheck -- appended into a ServiceWorkerGlobalScope, not a Window.
// CADUCEUS_UNREAD_SERVICE_WORKER_V1
// Appended to Tachyon's generated root worker by tac.config.js. Keep this file
// classic-worker compatible: production registration does not require modules.
(() => {
  const notificationActions = new Set(['mark-read', 'archive'])

  const normalizeCount = value => {
    const number = typeof value === 'number' ? value : Number(value)
    return Number.isFinite(number) ? Math.max(0, Math.trunc(number)) : 0
  }

  const applyBadge = async (value, genericWhenMissing = false) => {
    const supplied = value !== undefined && value !== null && Number.isFinite(Number(value))
    if (!supplied && genericWhenMissing && typeof self.navigator?.setAppBadge === 'function') {
      await self.navigator.setAppBadge()
      return null
    }
    const count = normalizeCount(value)
    if (count > 0 && typeof self.navigator?.setAppBadge === 'function') {
      await self.navigator.setAppBadge(count)
    } else if (count === 0 && typeof self.navigator?.clearAppBadge === 'function') {
      await self.navigator.clearAppBadge()
    } else if (count === 0 && typeof self.navigator?.setAppBadge === 'function') {
      await self.navigator.setAppBadge(0)
    }
    return count
  }

  const readPayload = event => {
    try { return event.data?.json?.() || {} } catch {
      try { return JSON.parse(event.data?.text?.() || '{}') } catch { return {} }
    }
  }

  self.addEventListener('push', event => {
    const payload = readPayload(event)
    const count = normalizeCount(payload.badgeCount)
    const title = String(payload.title || 'New mail')
    const tag = String(payload.tag || 'caduceus-new-mail')
    const actions = Array.isArray(payload.actions)
      ? payload.actions
        .filter(item => item && notificationActions.has(item.action) && typeof item.title === 'string')
        .map(item => ({ action: item.action, title: item.title.slice(0, 40) }))
      : []
    const options = {
      body: String(payload.body || ''),
      tag,
      icon: '/shared/assets/icon-192.png',
      badge: '/shared/assets/icon-192.png',
      data: payload.data && typeof payload.data === 'object' ? payload.data : { url: '/inbox' },
      renotify: false,
      silent: payload.silent === true,
      ...(actions.length ? { actions } : {}),
    }
    event.waitUntil(Promise.all([
      applyBadge(payload.badgeCount, true).catch(() => count),
      (async () => {
        // `tag` normally replaces an older notification, but explicitly close
        // it first so platforms with incomplete tag replacement cannot stack
        // duplicate delivery retries.
        const existing = typeof self.registration.getNotifications === 'function'
          ? await self.registration.getNotifications({ tag }).catch(() => [])
          : []
        for (const notification of existing) notification.close?.()
        await self.registration.showNotification(title, options)
      })(),
    ]))
  })

  self.addEventListener('message', event => {
    if (event.data?.type !== 'caduceus:set-badge') return
    event.waitUntil((async () => {
      const count = await applyBadge(event.data.count).catch(() => normalizeCount(event.data.count))
      event.source?.postMessage?.({ type: 'caduceus:badge-applied', count })
    })())
  })

  self.addEventListener('sync', event => {
    if (event.tag !== 'caduceus-offline-writes') return
    event.waitUntil((async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      for (const client of windows) client.postMessage?.({ type: 'caduceus:flush-offline-writes' })
    })())
  })

  self.addEventListener('notificationclick', event => {
    event.notification.close()
    const emailId = typeof event.notification.data?.emailId === 'string'
      && /^[A-Za-z0-9._:-]{1,200}$/.test(event.notification.data.emailId)
      ? event.notification.data.emailId
      : ''
    const action = notificationActions.has(event.action) && emailId ? event.action : ''
    const requested = new URL(event.notification.data?.url || '/inbox', self.location.origin)
    const target = requested.origin === self.location.origin
      ? requested.href
      : new URL('/inbox', self.location.origin).href
    event.waitUntil((async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const existing = windows.find(client => new URL(client.url).origin === self.location.origin)
      if (existing) {
        if (action) {
          existing.postMessage?.({
            type: 'caduceus:notification-action',
            action,
            emailId,
          })
          return existing.focus?.()
        }
        await existing.navigate?.(target)
        return existing.focus?.()
      }
      if (action) {
        const actionTarget = new URL('/inbox', self.location.origin)
        actionTarget.searchParams.set('notificationAction', action)
        actionTarget.searchParams.set('notificationEmail', emailId)
        return self.clients.openWindow?.(actionTarget.href)
      }
      return self.clients.openWindow?.(target)
    })())
  })
})()
