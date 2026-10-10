import { PersonalLevel } from '../shared/mailbox/PersonalLevel.js'
import { ThreadsRecords } from '../shared/mailbox/ThreadsRecords.js'

/** @import { StoredEmail } from '../types' */
/** @import { EmailsHelper } from './EmailsHelper.js' */

@Helper
export default class ThreadsHelper {
  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /**
   * Group messages by RFC Message-ID relationships, newest thread first.
   * Historical rows that predate retained headers use an isolated subject
   * fallback; a modern message with the same subject is never merged by name.
   * @param {string[]} allowedDomains
   * @param {string} [folder]
   * @param {string} [ownerEmail]
   * @param {Record<string, string>} [folderByDomain]
   * @returns {Promise<Thread[]>}
   */
  async list(allowedDomains, folder, ownerEmail, folderByDomain) {
    const emails = await this.emailsHelper.list(allowedDomains, { folder: 'all', ownerEmail })
    const threads = ThreadsRecords.groupMessagesIntoThreads(emails, ownerEmail)
    if (!folder || folder === 'all') return threads
    return threads.filter(thread => thread.emails.some(email => (
      email.folder || 'inbox'
    ) === (folderByDomain?.[email.domain] || folder)))
  }

  /**
   * Resolve the authorized conversation containing one selected message.
   * Unlike the ordinary folder list, printing and other conversation-level
   * actions must include messages that have moved to Sent, Archive, or a
   * custom folder. Domain filtering remains inside the email repository.
   *
   * @param {string[]} allowedDomains
   * @param {string} emailId
   * @param {string} [ownerEmail]
   * @returns {Promise<Thread | null>}
   */
  async findForEmail(allowedDomains, emailId, ownerEmail) {
    if (!emailId) return null
    const threads = await this.list(allowedDomains, 'all', ownerEmail)
    return threads.find(thread => thread.emails.some(email => email.id === emailId)) ?? null
  }
}

