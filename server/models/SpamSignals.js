/**
 * @typedef {object} SpamSignals
 * @property {string} [sender]   Envelope/from address.
 * @property {string} [subject]  Message subject.
 * @property {string} [text]     Plain-text body.
 * @property {Record<string, string>} [headers]  Lower-cased MIME headers.
 * @property {string} [verdict]  Structured local verdict ('PASS' | 'FAIL', or
 *   'spam' | 'ham').
 * @property {number|string} [score]  Structured upstream numeric score.
 */
export {}
