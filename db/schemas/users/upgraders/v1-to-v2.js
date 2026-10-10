/**
 * Materialize legacy user documents with display preference defaults.
 *
 * @param {Record<string, unknown>} doc
 * @returns {Record<string, unknown>}
 */
export default function upgrade(doc) {
  return {
    ...doc,
    calendarVisibility: typeof doc.calendarVisibility === 'string' ? doc.calendarVisibility : 'private',
    mailboxNames: doc.mailboxNames && typeof doc.mailboxNames === 'object' ? doc.mailboxNames : {},
  }
}
