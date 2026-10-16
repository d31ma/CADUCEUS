/** @import { QuoteProof } from './QuoteProof.js' */
/** @import { ComposeAttachment } from './ComposeAttachment.js' */
/**
 * Honest companion to the Go-owned ComposeEditorSnapshot JSON contract.
 * @typedef {Object} ComposeEditorSnapshot
 * @property {string} document
 * @property {number} revision
 * @property {string} body
 * @property {string} quote
 * @property {QuoteProof|null} [proof]
 * @property {ComposeAttachment[]} attachments
 */
export {}
