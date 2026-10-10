/**
 * The browser half of new-mail Web Push: whether this browser supports it,
 * the notification permission prompt, and the service worker's push
 * subscription. PushNotificationsService registers the subscription with
 * the server; Tachyon's Notifications API has no push subscription.
 */
@Service
export default class PushSubscriptionService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;
  active = true

  @OnStart
  start() {
    const supported = PushSubscriptionService._supported()
    /** @type {PushSupport} */
    const support = { supported, permission: PushSubscriptionService._permission(supported) }
    this.applicationEventPublisher.publishEvent(support, true)
  }

  /** @param {PushPermissionRequested} _requested */
  @OnEvent
  async ask(_requested) {
    /** @type {PushPermissionAnswered} */
    const answered = { permission: await Notification.requestPermission().catch(() => 'default') }
    this.applicationEventPublisher.publishEvent(answered, false)
  }

  /** @param {PushSubscribeRequested} requested */
  @OnEvent
  async subscribe(requested) {
    const created = await PushSubscriptionService._subscribe(requested.publicKey).catch(() => ({ subscription: {}, problem: 'subscribe' }))
    this.applicationEventPublisher.publishEvent(/** @type {PushSubscriptionCreated} */ (created), false)
  }

  /** @param {PushUnsubscribeRequested} _requested */
  @OnEvent
  async unsubscribe(_requested) {
    const removed = await PushSubscriptionService._unsubscribe().catch(() => ({ endpoint: '', problem: 'unsubscribe' }))
    this.applicationEventPublisher.publishEvent(/** @type {PushSubscriptionRemoved} */ (removed), false)
  }

  static _supported() {
    const missing = [typeof Notification, typeof PushManager].includes('undefined')
    return [!window.__CADUCEUS_DISABLE_SW, !missing, 'serviceWorker' in navigator].every(Boolean)
  }

  /** @param {boolean} supported */
  static _permission(supported) {
    return supported ? Notification.permission : 'unsupported'
  }

  /** @param {string} publicKey @returns {Promise<PushSubscriptionCreated>} */
  static async _subscribe(publicKey) {
    const registration = await navigator.serviceWorker.ready
    const existing = await registration.pushManager.getSubscription()
    const subscription = existing ?? await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: PushSubscriptionService._key(publicKey) })
    return { subscription: subscription.toJSON(), problem: '' }
  }

  /** @returns {Promise<PushSubscriptionRemoved>} */
  static async _unsubscribe() {
    const registration = await navigator.serviceWorker.ready
    const subscription = await registration.pushManager.getSubscription()
    const endpoint = [subscription?.endpoint].join('')
    await subscription?.unsubscribe()
    return { endpoint, problem: '' }
  }

  /** @param {string} value */
  static _key(value) {
    const quantum = 4
    const padded = (value + '='.repeat((quantum - (value.length % quantum)) % quantum)).replace(/-/g, '+').replace(/_/g, '/')
    return Uint8Array.from(atob(padded), character => character.charCodeAt(0))
  }
}
