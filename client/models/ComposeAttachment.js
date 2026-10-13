/**
 * One attachment of the message being written. ContentBase64 holds the bytes; the browser transport caps a request at 64 KiB, so attachments stay small.
 *
 * @typedef {Object} ComposeAttachment
 * @property {string} id
 * @property {string} filename
 * @property {string} contentType
 * @property {number} size
 * @property {string} disposition
 * @property {string} contentId
 * @property {string} contentBase64
 */
export {}
