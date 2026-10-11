/**
 * Escape hatch: the installed-app integrations that only the page can reach.
 *
 * - The Badging API, with the tab-title fallback: AppBadgeChanged sets
 *   navigator.setAppBadge, or prefixes the title with "(n)" where the API is
 *   missing, and tells the push worker the count.
 * - The push service worker. Tachyon generates the root worker and no longer
 *   lets an application append to it, so the CADUCEUS push handler in
 *   shared/service-worker/unread.js is registered on its own scope. Its
 *   notification buttons arrive as NotificationActionReceived, and so do the
 *   notificationAction/notificationEmail parameters of a launch URL.
 * - Update detection for the notification centre: a newly installed root
 *   worker becomes UpdateReady.
 */
@Service
export default class DeviceService {
  static _WORKER = '/shared/service-worker/unread.js'
  static _WORKER_SCOPE = '/shared/service-worker/'
  static _TITLE_PREFIX = /^\(\d+\)\s+/
  static _ACTION = 'notificationAction'
  static _EMAIL = 'notificationEmail'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false

  @OnStart
  start() {
    const workers = this._workers(navigator.serviceWorker, window.__CADUCEUS_DISABLE_SW === true)
    workers?.register(DeviceService._WORKER, { scope: DeviceService._WORKER_SCOPE }).catch(() => {})
    workers?.addEventListener('message', event => this._message(event.data))
    workers?.ready.then(registration => {
      registration.addEventListener('updatefound', () => this._installed(this._worker(registration), Boolean(workers.controller)))
      this._installed(this._worker(registration), Boolean(workers.controller))
    }).catch(() => {})
    this._launchAction(new URLSearchParams(location.search))
    this.ready = true
  }

  /** @param {AppBadgeChanged} changed */
  @OnEvent({"replay":true})
  badge(changed) {
    navigator.serviceWorker?.controller?.postMessage({ type: 'caduceus:set-badge', count: changed.count })
    this._applyBadge(changed.count).catch(() => this._title(changed.count))
  }

  /** @param {ServiceWorkerContainer | undefined} workers @param {boolean} disabled */
  _workers(workers, disabled) {
    return disabled ? undefined : workers
  }

  /** @param {ServiceWorkerRegistration} registration */
  _worker(registration) {
    return registration.installing ?? registration.waiting
  }

  /** @param {any} data */
  _message(data) {
    if (data?.type === 'caduceus:notification-action') this._action(String(data.action), String(data.emailId))
  }

  /** @param {URLSearchParams} params */
  _launchAction(params) {
    if (![DeviceService._ACTION, DeviceService._EMAIL].some(key => params.has(key))) return
    this._action(String(params.get(DeviceService._ACTION)), String(params.get(DeviceService._EMAIL)))
    this._cleanAddress(params)
  }

  /** @param {URLSearchParams} params */
  _cleanAddress(params) {
    params.delete(DeviceService._ACTION)
    params.delete(DeviceService._EMAIL)
    history.replaceState(history.state, '', [location.pathname, params.toString()].filter(Boolean).join('?') + location.hash)
  }

  /** @param {string} action @param {string} emailId */
  _action(action, emailId) {
    /** @type {NotificationActionReceived} */
    const received = { action, emailId }
    this.applicationEventPublisher.publishEvent(received, false)
  }

  /** @param {ServiceWorker | null} worker @param {boolean} controlled */
  _installed(worker, controlled) {
    if (![worker, controlled].every(Boolean)) return
    /** @type {ServiceWorker} */ (worker).addEventListener('statechange', () => this._reveal(/** @type {ServiceWorker} */ (worker).state))
    this._reveal(/** @type {ServiceWorker} */ (worker).state)
  }

  /** @param {string} state */
  _reveal(state) {
    /** @type {UpdateReady} */
    const ready = { version: 'ready' }
    ;[ready].filter(() => state === 'installed').forEach(value => this.applicationEventPublisher.publishEvent(/** @type {UpdateReady} */ (value), false))
  }

  /** @param {number} count */
  async _applyBadge(count) {
    if (typeof navigator.setAppBadge !== 'function') throw new Error('The Badging API is not available.')
    const apply = [() => navigator.clearAppBadge?.(), () => navigator.setAppBadge(count)][Number(count > 0)]
    await apply()
  }

  /** @param {number} count */
  _title(count) {
    const base = this._base(document.title)
    document.title = [base, `(${count}) ${base}`][Number(count > 0)]
  }

  /** @param {string} title */
  _base(title) {
    return title.replace(DeviceService._TITLE_PREFIX, '').trim() || 'CADUCEUS'
  }
}
