/**
 * @typedef {object} ComposePrefill
 * @property {string} [to] - Pre-fill the To field
 * @property {string} [cc] - Pre-fill the CC field
 * @property {string} [bcc] - Pre-fill the BCC field
 * @property {string} [replyTo] - Pre-fill the Reply-To field
 * @property {string} [from] - Pre-select an owned From address
 * @property {string} [subject] - Pre-fill the Subject field
 * @property {string} [body] - Pre-fill the message body
 * @property {string} [inReplyTo] - Canonical Message-ID being answered
 * @property {string[]} [references] - RFC reference chain for conversation threading
 * @property {string} [scheduledId] - Existing scheduled message being edited
 * @property {string} [sendAt] - Existing schedule as an ISO timestamp
 * @property {Array<any>} [attachments] - Bucket-backed attachments loaded for editing
 * @property {string} [sourceEmailId] - Authorized source conversation for Send & Archive
 */
export {}
