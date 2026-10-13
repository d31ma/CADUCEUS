import { Buffer } from 'node:buffer'
import { Threading } from '../mailbox/Threading.js'
import { TrackingTokens } from './TrackingTokens.js'

export class ExternalSubmission {
  static #ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/

  static #SUBMISSION_ID = /^sub_[A-Za-z0-9_-]{20,80}$/

  static #ACTION_ID = /^[A-Za-z0-9_-]{1,512}$/

  static #PREFIX = 'extundo.'

  /** Opaque browser token; every decoded identity is re-authorized server-side. */
  static externalUndoToken(accountId, submissionId, messageId, archiveActionId = '') {
    if (!ExternalSubmission.#ACCOUNT_ID.test(accountId) || !ExternalSubmission.#SUBMISSION_ID.test(submissionId) || !Threading.normalizeMessageId(messageId) || archiveActionId && !ExternalSubmission.#ACTION_ID.test(archiveActionId)) return ''
    return ExternalSubmission.#PREFIX + Buffer.from(JSON.stringify({ accountId, submissionId, messageId, ...(archiveActionId ? { archiveActionId } : {}) }), 'utf8').toString('base64url')
  }

  static parseExternalUndoToken(value) {
    if (typeof value !== 'string' || !value.startsWith(ExternalSubmission.#PREFIX) || value.length > 2048) return null
    try {
      const decoded = JSON.parse(Buffer.from(value.slice(ExternalSubmission.#PREFIX.length), 'base64url').toString('utf8'))
      if (!decoded || typeof decoded !== 'object' || !['accountId,messageId,submissionId', 'accountId,archiveActionId,messageId,submissionId'].includes(Object.keys(decoded).sort().join(','))) return null
      const messageId = Threading.normalizeMessageId(decoded.messageId)
      if (!ExternalSubmission.#ACCOUNT_ID.test(decoded.accountId) || !ExternalSubmission.#SUBMISSION_ID.test(decoded.submissionId) || !messageId || decoded.archiveActionId && !ExternalSubmission.#ACTION_ID.test(decoded.archiveActionId)) return null
      return { accountId: decoded.accountId, submissionId: decoded.submissionId, messageId, ...(decoded.archiveActionId ? { archiveActionId: decoded.archiveActionId } : {}) }
    } catch {
      return null
    }
  }

  /** Build the provider-owned MIME input while leaving the canonical Sent copy
   * clean. Connected tracking is exact only when one envelope recipient owns the
   * embedded capabilities; callers enforce that cardinality before using this. */
  static externalDeliveryRequest(request, trackingId, recipient) {
    if (!trackingId) return request
    const tracked = TrackingTokens.prepareTrackedHtml(request.html || '', {
      trackingId,
      recipient,
      openTracking: request.openTracking,
      linkTracking: request.linkTracking,
    })
    return { ...request, html: tracked.html }
  }
}
