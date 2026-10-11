/**
 * Escape hatch: reading the vCard file picked for import, which a Go service
 * cannot do. The file's text goes to ContactTransferService in a
 * ContactsFileChosen event; the import decides what to do with it.
 */
@Service
export default class ContactsBrowserService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  /** @param {Event} event */
  importFile(event) {
    const input = /** @type {HTMLInputElement} */ (event.target)
    const files = Array.from(/** @type {FileList} */ (input.files))
    input.value = ''
    files.slice(0, 1).forEach(file => this._read(file))
  }

  /** @param {File} file */
  _read(file) {
    void file.text().then(content => this._publish(file, content))
  }

  /** @param {File} file @param {string} content */
  _publish(file, content) {
    /** @type {ContactsFileChosen} */
    const chosen = { name: file.name, size: file.size, content }
    this.applicationEventPublisher.publishEvent(chosen, false)
  }
}
