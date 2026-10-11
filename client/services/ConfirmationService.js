/**
 * Escape hatch: window.confirm(), which no Go service can reach. A view asks
 * the question on a click; an accepted question becomes a ConfirmationGiven
 * event for the Go service that owns the action.
 */
@Service
export default class ConfirmationService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  /** @param {Event} _event @param {string} question @param {string} action @param {string} id */
  confirmThen(_event, question, action, id) {
    this._publishWhen(confirm(question), { action, id })
  }

  /** @param {boolean} confirmed @param {ConfirmationGiven} confirmation */
  _publishWhen(confirmed, confirmation) {
    if (confirmed) this.applicationEventPublisher.publishEvent(confirmation, false)
  }
}
