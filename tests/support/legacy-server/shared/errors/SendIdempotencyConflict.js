export class SendIdempotencyConflict extends Error {
  /** @param {string} message @param {'mismatch'|'applying'|'ambiguous'} kind */
  constructor(message, kind) { super(message); this.kind = kind }
}
