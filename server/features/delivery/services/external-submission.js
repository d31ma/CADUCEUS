import { Buffer } from 'node:buffer'
import { normalizeMessageId } from '../../mailbox/services/threading.js'
import { prepareTrackedHtml } from './tracking-tokens.js'

const ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/
const SUBMISSION_ID = /^sub_[A-Za-z0-9_-]{20,80}$/
const ACTION_ID = /^[A-Za-z0-9_-]{1,512}$/
const PREFIX = 'extundo.'

/** Opaque browser token; every decoded identity is re-authorized server-side. */
export function externalUndoToken(accountId, submissionId, messageId, archiveActionId = '') {
  if (!ACCOUNT_ID.test(accountId) || !SUBMISSION_ID.test(submissionId) || !normalizeMessageId(messageId) || archiveActionId && !ACTION_ID.test(archiveActionId)) return ''
  return PREFIX + Buffer.from(JSON.stringify({ accountId, submissionId, messageId, ...(archiveActionId ? { archiveActionId } : {}) }), 'utf8').toString('base64url')
}

export function parseExternalUndoToken(value) {
  if (typeof value !== 'string' || !value.startsWith(PREFIX) || value.length > 2048) return null
  try {
    const decoded = JSON.parse(Buffer.from(value.slice(PREFIX.length), 'base64url').toString('utf8'))
    if (!decoded || typeof decoded !== 'object' || !['accountId,messageId,submissionId', 'accountId,archiveActionId,messageId,submissionId'].includes(Object.keys(decoded).sort().join(','))) return null
    const messageId = normalizeMessageId(decoded.messageId)
    if (!ACCOUNT_ID.test(decoded.accountId) || !SUBMISSION_ID.test(decoded.submissionId) || !messageId || decoded.archiveActionId && !ACTION_ID.test(decoded.archiveActionId)) return null
    return { accountId: decoded.accountId, submissionId: decoded.submissionId, messageId, ...(decoded.archiveActionId ? { archiveActionId: decoded.archiveActionId } : {}) }
  } catch {
    return null
  }
}

/** Build the provider-owned MIME input while leaving the canonical Sent copy
 * clean. Connected tracking is exact only when one envelope recipient owns the
 * embedded capabilities; callers enforce that cardinality before using this. */
export function externalDeliveryRequest(request, trackingId, recipient) {
  if (!trackingId) return request
  const tracked = prepareTrackedHtml(request.html || '', {
    trackingId,
    recipient,
    openTracking: request.openTracking,
    linkTracking: request.linkTracking,
  })
  return { ...request, html: tracked.html }
}
