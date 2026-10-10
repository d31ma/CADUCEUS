import { Security } from '../security/Security.js'

const EMAIL_IN_HEADER = /[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+/gi

/**
 * Derive safe, displayable sender metadata and Mailspring-compatible
 * Reply-To mismatch warnings from untrusted RFC headers.
 */
export class PhishingDetection {
  /**
   * @param {{ envelopeSender: string, headers?: Record<string, string> }} input
   * @returns {{ headerFrom: string, replyTo: string[], securityWarnings: Array<'reply_to_mismatch'> }}
   */
  static analyze({ envelopeSender, headers = {} }) {
    const normalizedEnvelope = Security.normalizeEmailAddress(envelopeSender) ?? ''
    const headerFrom = PhishingDetection.extractAddresses(headers.from)[0] ?? normalizedEnvelope
    const replyTo = PhishingDetection.extractAddresses(headers['reply-to'])
    const securityWarnings = /** @type {Array<'reply_to_mismatch'>} */ (replyTo.some(address => address !== headerFrom)
      ? ['reply_to_mismatch']
      : [])
    return { headerFrom, replyTo, securityWarnings }
  }

  /**
   * Extract normalized mailbox addresses without trusting display names or
   * attempting to interpret comments as addresses.
   *
   * @param {unknown} header
   * @returns {string[]}
   */
  static extractAddresses(header) {
    if (typeof header !== 'string' || !header) return []
    const addresses = []
    for (const match of header.match(EMAIL_IN_HEADER) ?? []) {
      const normalized = Security.normalizeEmailAddress(match)
      if (normalized && !addresses.includes(normalized)) addresses.push(normalized)
      if (addresses.length === 10) break
    }
    return addresses
  }
}
