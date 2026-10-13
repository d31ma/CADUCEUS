/**
 * @typedef {object} OutboxEntry
 * @property {string} id
 * @property {import('./SendRequest.js').SendRequest} email
 * @property {string} sender
 * @property {string} [owner]
 * @property {number} expiresAt          When the entry first becomes eligible (undo window end).
 * @property {number} [nextAttempt]      Earliest time the next delivery attempt may run.
 * @property {number} [attempts]         Delivery attempts made so far.
 * @property {'queued' | 'sending'} [status]
 * @property {string} [lastError]
 * @property {number} [createdAt]
 * @property {string} [trackingId]
 * @property {string} [scheduledId]      Logical scheduled-send record linked to this delivery.
 * @property {number} [claimedAt]        Worker lease start; stale leases are reclaimable.
 * @property {string} [attachmentEmailId] Bucket-backed attachment set to hydrate before SMTP.
 * @property {string} [archiveStateJson] Undo-safe source conversation folder snapshot.
 */
export {}
