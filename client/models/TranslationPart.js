/**
 * One piece of a translated message: html, text, or done (whose text is the
 * source language). One event holds at most 64 KiB, so a body comes in parts.
 *
 * @typedef {Object} TranslationPart
 * @property {string} messageId
 * @property {string} kind
 * @property {number} index
 * @property {number} count
 * @property {string} text
 */
export {}
