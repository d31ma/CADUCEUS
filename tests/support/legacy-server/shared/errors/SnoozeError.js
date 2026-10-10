export class SnoozeError extends Error {
  /** @param {string} message @param {number} status */
  constructor(message, status = 400) {
    super(message)
    this.status = status
  }
}
