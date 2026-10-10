import { UnreadBadgePreferences } from '../shared/users/UnreadBadgePreferences.js'
import { NewMailNotificationPreferences } from '../shared/users/NewMailNotificationPreferences.js'
import { ExternalMessage } from '../shared/mailbox/ExternalMessage.js'
import { ServerEnvironment } from '../shared/runtime/ServerEnvironment.js'
import { PushRules } from '../shared/notifications/PushRules.js'

@Service
export default class PushService {
  /** @type {PushSubscriptionsHelper} */
  @Needs
  pushSubscriptionsHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @returns {Promise<string>} */
  async getVapidPublicKey() {
    if (ServerEnvironment.value('VAPID_PUBLIC_KEY')) return ServerEnvironment.value('VAPID_PUBLIC_KEY')
    try {
      const wp = await PushRules.webPush()
      const keys = wp.default.generateVAPIDKeys?.() || wp.generateVAPIDKeys?.()
      if (keys?.publicKey) return keys.publicKey
    } catch (e) {
      console.error('[push] web-push import/generation failed, using fallback:', e)
    }
    // Ephemeral fallback for local dev.
    return PushRules.ephemeralVapidPublicKey()
  }

  /**
   * Fan out a push notification to every subscribed device for the recipient.
   * No-ops when push is disabled or VAPID keys are missing — push is opt-in.
   *
   * @param {StoredEmail} email
   * @returns {Promise<boolean>}
   */
  async notifyNewEmail(email) {
    return this.notify(email, PushRules.buildNewMailPushPayload, 'notifyNewEmail')
  }

  /**
   * Deliver one scheduler-claimed provider notification through the same Web
   * Push privacy, sound, action, and subscription policy as hosted mail.
   *
   * @param {{ accountId:string, ownerAddress:string, stableId:string, sender:string, subject:string, receivedAt:string }} notification
   * @returns {Promise<boolean>} true when the durable outbox item is terminal
   */
  async notifyProviderNewEmail(notification) {
    const id = ExternalMessage.externalMessageId(notification.accountId, notification.stableId)
    if (!id) return true
    const ownerAddress = String(notification.ownerAddress || '').toLowerCase()
    const email = /** @type {StoredEmail} */ (/** @type {unknown} */ ({
      id,
      external: true,
      providerAccountId: notification.accountId,
      recipient: ownerAddress,
      domain: ownerAddress.split('@')[1] || '',
      sender: notification.sender || '',
      subject: notification.subject || '',
      receivedAt: notification.receivedAt,
      folder: 'inbox',
      systemFolder: 'inbox',
      read: false,
      starred: false,
    }))
    return this.notify(email, PushRules.buildNewMailPushPayload, 'notifyProviderNewEmail', {
      ownerEmail: ownerAddress,
      externalAccountId: notification.accountId,
    })
  }

  /** @param {StoredEmail} email @returns {Promise<boolean>} */
  async notifyUnsnoozed(email) {
    return this.notify(email, PushRules.buildUnsnoozedPushPayload, 'notifyUnsnoozed')
  }

  /**
   * @param {StoredEmail} email
   * @param {(email: StoredEmail, unreadCount: number | undefined, preferences: Record<string, unknown>) => Record<string, unknown>} payloadFor
   * @param {string} operation
   * @param {{ ownerEmail?: string, externalAccountId?: string }} [options]
   * @returns {Promise<boolean>}
   */
  async notify(email, payloadFor, operation, options = {}) {
    if (ServerEnvironment.value('WEB_PUSH_DISABLED') === 'true') return true
    if (!ServerEnvironment.value('VAPID_PUBLIC_KEY') || !ServerEnvironment.value('VAPID_PRIVATE_KEY')) return true
    try {
      const webPush = await PushRules.webPush()
      const vapidPublicKey = ServerEnvironment.value('VAPID_PUBLIC_KEY')
      const vapidPrivateKey = ServerEnvironment.value('VAPID_PRIVATE_KEY')
      const vapidSubject = ServerEnvironment.value('VAPID_SUBJECT') || 'mailto:admin@caduceus.local'

      const lookupAddress = options.ownerEmail || email.recipient
      const [, user] = await this.usersHelper.findByEmail(lookupAddress)
      if (!user) return true
      const userEmail = user.email || lookupAddress
      const domain = PushRules.notificationDomain(email)
      const policyDomains = user?.domains?.length ? user.domains : [domain]
      if (options.externalAccountId) {
        if (!NewMailNotificationPreferences.newMailNotificationsEnabledForExternalAccount(user, policyDomains, options.externalAccountId)) return true
      } else if (!NewMailNotificationPreferences.newMailNotificationsEnabledForDomain(user, policyDomains, domain)) return true
      const notificationPreferences = options.externalAccountId
        ? NewMailNotificationPreferences.normalizeNewMailNotificationPreferencesForAccounts(user, policyDomains, [options.externalAccountId])
        : NewMailNotificationPreferences.normalizeNewMailNotificationPreferences(user, policyDomains)
      const subscriptions = await this.pushSubscriptionsHelper.listForAddress(userEmail)
      if (subscriptions.length === 0) return true

      let unreadCount
      if (this.emailsHelper && !options.externalAccountId) {
        try {
          unreadCount = await PushRules.countUnreadInbox(this.emailsHelper, user, email, userEmail)
        } catch (error) {
          // Delivery notifications still matter when an optional count query
          // fails. The worker will show a generic badge and the next mailbox
          // snapshot will reconcile the exact number.
          console.error('[push] unread count failed, using generic badge:', error)
        }
      }

      const payload = JSON.stringify(payloadFor(email, unreadCount, {
        ...notificationPreferences,
        soundId: user?.notificationSounds?.['new-mail'] || 'chime',
      }))

      const results = await Promise.allSettled(subscriptions.map(sub =>
        // @ts-ignore - subscription record has the right shape for web-push at runtime
        webPush.default.sendNotification(sub, payload, {
          vapidDetails: { subject: vapidSubject, publicKey: vapidPublicKey, privateKey: vapidPrivateKey },
        }),
      ))
      return results.some(result => result.status === 'fulfilled')
    } catch (e) {
      console.error(`[push] ${operation} failed:`, e)
      return false
    }
  }
}
