import { generateKeyPairSync } from 'node:crypto'
import { UnreadBadgePreferences } from '../users/UnreadBadgePreferences.js'
import { NewMailNotificationPreferences } from '../users/NewMailNotificationPreferences.js'
import { ExternalMessage } from '../mailbox/ExternalMessage.js'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class PushRules {
  /** @import { StoredEmail } from '../../../types' */
  /** @import { PushSubscriptionsHelper } from '../repositories/PushSubscriptionsHelper.js' */
  /** @import { UsersHelper } from '../../users/repositories/UsersHelper.js' */
  /** @import { EmailsHelper } from '../../mailbox/repositories/EmailsHelper.js' */

  static NOTIFICATION_ACTIONS = Object.freeze([
    { action: 'mark-read', title: 'Mark read' },
    { action: 'archive', title: 'Archive' },
  ])

  /** @param {StoredEmail} email */
  static notificationDomain(email) {
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
  static newMailCopy(email, privacy) {
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
  static buildNewMailPushPayload(email, unreadCount, preferences = {}) {
    const badgeCount = Number.isFinite(unreadCount)
      ? Math.max(0, Math.trunc(/** @type {number} */ (unreadCount)))
      : undefined
    const privacy = preferences.newMailNotificationPrivacy || 'full'
    const copy = PushRules.newMailCopy(email, privacy)
    const soundId = typeof preferences.soundId === 'string' ? preferences.soundId : 'chime'
    return {
      ...copy,
      data: { url: `/email/${email.id}`, emailId: email.id, soundId },
      tag: `mail:${email.id}`,
      silent: soundId === 'none',
      ...(preferences.newMailNotificationActions === false ? {} : { actions: PushRules.NOTIFICATION_ACTIONS }),
      ...(badgeCount == null ? {} : { badgeCount }),
    }
  }

  /** @param {StoredEmail} email @param {number | undefined} unreadCount @param {{ newMailNotificationPrivacy?: 'full'|'sender'|'private', newMailNotificationActions?: boolean, soundId?: string }} [preferences] */
  static buildUnsnoozedPushPayload(email, unreadCount, preferences = {}) {
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
      ...(preferences.newMailNotificationActions === false ? {} : { actions: PushRules.NOTIFICATION_ACTIONS }),
      ...(badgeCount == null ? {} : { badgeCount }),
    }
  }

  /**
   * Resolve the exact unread inbox count visible to the notification owner.
   * This intentionally uses the repository's normal domain and mailbox scope so
   * background state cannot bypass the same boundary as the foreground inbox.
   *
   * @param {EmailsHelper} emails
   * @param {import('../../../types').User | null} user
   * @param {StoredEmail} email
   * @param {string} userEmail
   */
  static async countUnreadInbox(emails, user, email, userEmail) {
    const fallbackDomain = PushRules.notificationDomain(email)
    const domains = UnreadBadgePreferences.unreadBadgeDomains(user, user?.domains, fallbackDomain)
    const unread = await emails.list(domains, {
      folder: 'inbox',
      read: false,
      ownerEmail: userEmail,
    })
    return unread.length
  }

  /** The bundled web-push module, loaded only when a push is sent. */
  static async webPush() {
    return await import('../../deps/npm/web-push.mjs')
  }

  /** @returns {string} A throwaway VAPID public key for local development. */
  static ephemeralVapidPublicKey() {
    const { publicKey } = generateKeyPairSync('ec', { namedCurve: 'prime256v1' })
    return publicKey.export({ type: 'spki', format: 'der' }).toString('base64url')
  }
}
