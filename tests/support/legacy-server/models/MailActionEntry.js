/**
 * @typedef {object} MailActionEntry
 * @property {string} id
 * @property {string} ownerEmail
 * @property {string} domain
 * @property {'archive'|'move'|'label'|'trash'|'mark-read'|'mark-unread'|'star'|'unstar'|'bulk'} kind
 * @property {'prepared'|'applied'|'undoing'|'undone'|'redoing'|'failed'} status
 * @property {string} changesJson
 * @property {number} createdAt
 * @property {number} updatedAt
 * @property {number} expiresAt
 * @property {string} [lastError]
 * @property {string} [clientRequestId]
 * @property {string} [requestFingerprint]
 */
export {}
