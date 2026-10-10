/** @typedef {import('../../../types').StoredEmail} StoredEmail */
/** @typedef {import('../repositories/push.js').PushSubscriptionsRepository} PushSubscriptionsRepository */
/** @typedef {import('../../users/repositories/users.js').UsersRepository} UsersRepository */
/** @typedef {import('../../mailbox/repositories/emails.js').EmailsRepository} EmailsRepository */

import { unreadBadgeDomains } from '../../users/services/unread-badge-preferences.js'
import {
  newMailNotificationsEnabledForDomain,
  newMailNotificationsEnabledForExternalAccount,
  normalizeNewMailNotificationPreferences,
  normalizeNewMailNotificationPreferencesForAccounts,
} from '../../users/services/new-mail-notification-preferences.js'
import { externalMessageId } from '../../mailbox/services/external-message.js'

const NOTIFICATION_ACTIONS = Object.freeze([
  { action: 'mark-read', title: 'Mark read' },
  { action: 'archive', title: 'Archive' },
])

/** @param {StoredEmail} email */
function notificationDomain(email) {
  return String(email.domain || email.recipient.split('@').pop() || '').toLowerCase()
}

/**
 * Apply the user's lock-screen privacy level without weakening the deep-link
 * authorization boundary. Message identifiers remain opaque app navigation
 * data and are re-authorized before any action is applied.
 *
 * @param {StoredEmail} email
 * @param {'full'|'sender'|'private'} privacy
 */
function newMailCopy(email, privacy) {
  if (privacy === 'private') return { title: 'New mail', body: 'Open CADUCEUS to read it.' }
  if (privacy === 'sender') {
    return { title: `New mail from ${email.sender}`, body: 'Open CADUCEUS to read this message.' }
  }
  return { title: `New mail from ${email.sender}`, body: email.subject }
}

/**
 * Build the public Web Push payload. Keeping this deterministic makes the
 * notification and unread-badge contract independently testable from the
 * network transport.
 *
 * @param {StoredEmail} email
 * @param {number | undefined} unreadCount
 * @param {{ newMailNotificationPrivacy?: 'full'|'sender'|'private', newMailNotificationActions?: boolean, soundId?: string }} [preferences]
 */
export function buildNewMailPushPayload(email, unreadCount, preferences = {}) {
  const badgeCount = Number.isFinite(unreadCount)
    ? Math.max(0, Math.trunc(/** @type {number} */ (unreadCount)))
    : undefined
  const privacy = preferences.newMailNotificationPrivacy || 'full'
  const copy = newMailCopy(email, privacy)
  const soundId = typeof preferences.soundId === 'string' ? preferences.soundId : 'chime'
  return {
    ...copy,
    data: { url: `/email/${email.id}`, emailId: email.id, soundId },
    tag: `mail:${email.id}`,
    silent: soundId === 'none',
    ...(preferences.newMailNotificationActions === false ? {} : { actions: NOTIFICATION_ACTIONS }),
    ...(badgeCount == null ? {} : { badgeCount }),
  }
}

/** @param {StoredEmail} email @param {number | undefined} unreadCount @param {{ newMailNotificationPrivacy?: 'full'|'sender'|'private', newMailNotificationActions?: boolean, soundId?: string }} [preferences] */
export function buildUnsnoozedPushPayload(email, unreadCount, preferences = {}) {
  const badgeCount = Number.isFinite(unreadCount)
    ? Math.max(0, Math.trunc(/** @type {number} */ (unreadCount)))
    : undefined
  const privacy = preferences.newMailNotificationPrivacy || 'full'
  const soundId = typeof preferences.soundId === 'string' ? preferences.soundId : 'chime'
  return {
    title: 'Snoozed conversation returned',
    body: privacy === 'full' ? email.subject : 'Open CADUCEUS to read it.',
    data: { url: `/email/${email.id}`, emailId: email.id, reason: 'snooze-expired', soundId },
    tag: `snooze:${email.threadId || email.id}`,
    silent: soundId === 'none',
    ...(preferences.newMailNotificationActions === false ? {} : { actions: NOTIFICATION_ACTIONS }),
    ...(badgeCount == null ? {} : { badgeCount }),
  }
}

/**
 * Resolve the exact unread inbox count visible to the notification owner.
 * This intentionally uses the repository's normal domain and mailbox scope so
 * background state cannot bypass the same boundary as the foreground inbox.
 *
 * @param {EmailsRepository} emails
 * @param {import('../../../types').User | null} user
 * @param {StoredEmail} email
 * @param {string} userEmail
 */
export async function countUnreadInbox(emails, user, email, userEmail) {
  const fallbackDomain = notificationDomain(email)
  const domains = unreadBadgeDomains(user, user?.domains, fallbackDomain)
  const unread = await emails.list(domains, {
    folder: 'inbox',
    read: false,
    ownerEmail: userEmail,
  })
  return unread.length
}

export class PushService {
  /**
   * @param {PushSubscriptionsRepository} pushSubscriptions
   * @param {UsersRepository} users
   * @param {EmailsRepository} [emails]
   */
  constructor(pushSubscriptions, users, emails) {
    this.pushSubscriptions = pushSubscriptions
    this.users = users
    this.emails = emails
  }

  /** @returns {Promise<string>} */
  async getVapidPublicKey() {
    if (process.env.VAPID_PUBLIC_KEY) return process.env.VAPID_PUBLIC_KEY
    try {
      const wp = await import('web-push')
      const keys = wp.default.generateVAPIDKeys?.() || wp.generateVAPIDKeys?.()
      if (keys?.publicKey) return keys.publicKey
    } catch (e) {
      console.error('[push] web-push import/generation failed, using fallback:', e)
    }
    // Ephemeral fallback for local dev.
    const { generateKeyPairSync } = await import('node:crypto')
    const { publicKey } = generateKeyPairSync('ec', { namedCurve: 'prime256v1' })
    return publicKey.export({ type: 'spki', format: 'der' }).toString('base64url')
  }

  /**
   * Fan out a push notification to every subscribed device for the recipient.
   * No-ops when push is disabled or VAPID keys are missing — push is opt-in.
   *
   * @param {StoredEmail} email
   * @returns {Promise<boolean>}
   */
  async notifyNewEmail(email) {
    return this.notify(email, buildNewMailPushPayload, 'notifyNewEmail')
  }

  /**
   * Deliver one scheduler-claimed provider notification through the same Web
   * Push privacy, sound, action, and subscription policy as hosted mail.
   *
   * @param {{ accountId:string, ownerAddress:string, stableId:string, sender:string, subject:string, receivedAt:string }} notification
   * @returns {Promise<boolean>} true when the durable outbox item is terminal
   */
  async notifyProviderNewEmail(notification) {
    const id = externalMessageId(notification.accountId, notification.stableId)
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
    return this.notify(email, buildNewMailPushPayload, 'notifyProviderNewEmail', {
      ownerEmail: ownerAddress,
      externalAccountId: notification.accountId,
    })
  }

  /** @param {StoredEmail} email @returns {Promise<boolean>} */
  async notifyUnsnoozed(email) {
    return this.notify(email, buildUnsnoozedPushPayload, 'notifyUnsnoozed')
  }

  /**
   * @param {StoredEmail} email
   * @param {(email: StoredEmail, unreadCount: number | undefined, preferences: Record<string, unknown>) => Record<string, unknown>} payloadFor
   * @param {string} operation
   * @param {{ ownerEmail?: string, externalAccountId?: string }} [options]
   * @returns {Promise<boolean>}
   */
  async notify(email, payloadFor, operation, options = {}) {
    if (process.env.WEB_PUSH_DISABLED === 'true') return true
    if (!process.env.VAPID_PUBLIC_KEY || !process.env.VAPID_PRIVATE_KEY) return true
    try {
      const webPush = await import('web-push')
      const vapidPublicKey = process.env.VAPID_PUBLIC_KEY
      const vapidPrivateKey = process.env.VAPID_PRIVATE_KEY
      const vapidSubject = process.env.VAPID_SUBJECT || 'mailto:admin@caduceus.local'

      const lookupAddress = options.ownerEmail || email.recipient
      const [, user] = await this.users.findByEmail(lookupAddress)
      if (!user) return true
      const userEmail = user.email || lookupAddress
      const domain = notificationDomain(email)
      const policyDomains = user?.domains?.length ? user.domains : [domain]
      if (options.externalAccountId) {
        if (!newMailNotificationsEnabledForExternalAccount(user, policyDomains, options.externalAccountId)) return true
      } else if (!newMailNotificationsEnabledForDomain(user, policyDomains, domain)) return true
      const notificationPreferences = options.externalAccountId
        ? normalizeNewMailNotificationPreferencesForAccounts(user, policyDomains, [options.externalAccountId])
        : normalizeNewMailNotificationPreferences(user, policyDomains)
      const subscriptions = await this.pushSubscriptions.listForAddress(userEmail)
      if (subscriptions.length === 0) return true

      let unreadCount
      if (this.emails && !options.externalAccountId) {
        try {
          unreadCount = await countUnreadInbox(this.emails, user, email, userEmail)
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
