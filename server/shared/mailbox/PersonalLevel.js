export class PersonalLevel {
  /** @import { StoredEmail } from '../../types' */
  static #ADDRESS_PATTERN = /[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+/gi

  /**
   * Extract normalized mailbox addresses from an RFC address-list value.
   * Display names, angle brackets, and comments are intentionally discarded;
   * the persisted relationship is an address, not presentation text.
   *
   * @param {unknown} value
   * @returns {string[]}
   */
  static parseAddressList(value) {
    const values = Array.isArray(value) ? value : [value]
    const addresses = []
    for (const item of values) {
      const matches = String(item ?? '').match(PersonalLevel.#ADDRESS_PATTERN) ?? []
      addresses.push(...matches.map(address => address.toLowerCase()))
    }
    return [...new Set(addresses)]
  }

  /**
   * Preserve the visible To/Cc/Bcc relationship for an accepted message. An
   * SMTP envelope recipient is used only when the message contains no recipient
   * headers at all. This distinction keeps list-delivered mail from appearing
   * to be addressed directly to the local mailbox.
   *
   * @param {Record<string, string>} headers
   * @param {string} envelopeRecipient
   * @returns {{to: string[], cc: string[], bcc: string[]}}
   */
  static recipientsFromHeaders(headers, envelopeRecipient) {
    const to = PersonalLevel.parseAddressList(headers?.to)
    const cc = PersonalLevel.parseAddressList(headers?.cc)
    const bcc = PersonalLevel.parseAddressList(headers?.bcc)
    if (to.length + cc.length + bcc.length === 0) {
      return { to: PersonalLevel.parseAddressList(envelopeRecipient), cc: [], bcc: [] }
    }
    return { to, cc, bcc }
  }

  /**
   * Return every unique participant explicitly visible in one stored message.
   * Historical records predate recipient arrays, so their envelope recipient is
   * the best available compatibility fallback.
   *
   * @param {StoredEmail} email
   * @returns {string[]}
   */
  static messageParticipants(email) {
    const explicitRecipients = [
      ...PersonalLevel.parseAddressList(email.to),
      ...PersonalLevel.parseAddressList(email.cc),
      ...PersonalLevel.parseAddressList(email.bcc),
    ]
    const recipients = explicitRecipients.length > 0
      ? explicitRecipients
      : PersonalLevel.parseAddressList(email.recipient)
    return [...new Set([...PersonalLevel.parseAddressList(email.sender), ...recipients])]
  }

  /**
   * Match Mailspring's personal-level semantics for a complete conversation:
   * level 2 means the mailbox and exactly one other participant, level 1 means
   * the mailbox is one of three or more participants, and 0 means no indicator.
   *
   * @param {StoredEmail[]} emails
   * @param {string | undefined} mailbox
   * @returns {0 | 1 | 2}
   */
  static personalLevelForThread(emails, mailbox) {
    const me = PersonalLevel.parseAddressList(mailbox)[0]
    if (!me) return 0
    const participants = new Set(emails.flatMap(PersonalLevel.messageParticipants))
    if (!participants.has(me)) return 0
    if (participants.size === 2) return 2
    if (participants.size > 2) return 1
    return 0
  }
}
