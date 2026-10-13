/**
 * @typedef {object} OfflineWriteInput
 * @property {'mail-action'|'draft'|'send'|'snooze'|'calendar'} kind
 * @property {string} owner
 * @property {string} path
 * @property {'POST'|'PUT'|'DELETE'} method
 * @property {Record<string,unknown>|null} [body]
 * @property {string} [message]
 * @property {string} [id]
 * @property {string} [coalesceKey]
 * @property {Record<string,unknown>} [metadata]
 */
export {}
