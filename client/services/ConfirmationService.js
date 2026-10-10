/**
 * Asks the browser's own confirmation dialog before a destructive action.
 * The dialog is a browser feature with no Tachyon API, so this small
 * JavaScript service shows it and announces an accepted answer with
 * ConfirmationAccepted; the Go service that owns the topic acts on it.
 */
@Service
export default class ConfirmationService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;

  /** @param {Event} _event @param {string} message @param {string} topic @param {string} subject */
  ask(_event, message, topic, subject) {
    /** @type {ConfirmationAccepted} */
    const accepted = { topic, subject }
    this._confirm(message, accepted)
  }

  /** @param {string} message @param {ConfirmationAccepted} accepted */
  _confirm(message, accepted) {
    if (confirm(message)) this.applicationEventPublisher.publishEvent(accepted, false)
  }
}
