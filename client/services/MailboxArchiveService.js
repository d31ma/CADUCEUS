import { exportFolder, importFiles, rememberSummaries, searchOffline } from '/shared/scripts/mailbox-archive.js'

/**
 * Moves the bytes of the mailbox archive tools and the encrypted offline
 * index, which the Go services cannot reach: file pickers and downloads, an
 * unbounded binary export stream, 50 MiB uploads, and IndexedDB with
 * WebCrypto. MailboxTransferService (Go) asks with events and words the
 * progress this service reports; InboxService (Go) hands it the summaries to
 * keep and asks it to search them when the server cannot answer.
 */
@Service
export default class MailboxArchiveService {
  static _ABORT = 'AbortError'

  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher
  /** @type {AbortSignal | undefined} Injected before lifecycle methods run. */
  @Needs
  lifecycle
  /** @type {AbortController} */
  _exporting = new AbortController()
  /** @type {AbortController} */
  _importing = new AbortController()
  /** @type {ImportPickRequested} */
  _target = { mailbox: '', folder: '', authorization: '', open: false }
  /** @type {File[]} */
  _pending = []

  /**
   * The files chosen in the import picker. They wait for the import target
   * when they are chosen before the mailbox list has said where it is.
   * @param {Event} event
   */
  importChosenFiles(event) {
    this._pending = this._takeFiles(/** @type {HTMLInputElement} */ (event.target))
    this._resume()
  }

  /** @param {ExportRequested} request */
  @OnEvent
  exportRequested(request) {
    void this._export(request)
  }

  /** @param {ExportCancelRequested} _request */
  @OnEvent
  exportCancelRequested(_request) {
    this._exporting.abort(new DOMException('Mailbox export canceled', MailboxArchiveService._ABORT))
  }

  /** @param {ImportPickRequested} request */
  @OnEvent({"replay":true})
  importPickRequested(request) {
    this._target = request
    this._pickWhen(request.open)
    this._resume()
  }

  _resume() {
    if ([this._pending.length > 0, Boolean(this._target.mailbox), Boolean(this._target.folder)].every(Boolean)) void this._import(this._takePending())
  }

  /** @returns {File[]} */
  _takePending() {
    const files = this._pending
    this._pending = []
    return files
  }

  /** @param {boolean} open */
  _pickWhen(open) {
    if (open) /** @type {HTMLInputElement | null} */ (document.querySelector('[data-eml-import]'))?.click()
  }

  /** @param {ImportCancelRequested} _request */
  @OnEvent
  importCancelRequested(_request) {
    this._importing.abort(new DOMException('EML import canceled', MailboxArchiveService._ABORT))
  }

  /** @param {MailboxSummaries} summaries */
  @OnEvent
  mailboxSummaries(summaries) {
    void rememberSummaries(summaries.owner, summaries.emails)
  }

  /** @param {OfflineSearchRequested} request */
  @OnEvent
  offlineSearchRequested(request) {
    void this._searchOffline(request)
  }

  /** @param {ExportRequested} request */
  async _export(request) {
    this._exporting = new AbortController()
    const link = /** @type {HTMLAnchorElement | null} */ (document.querySelector('[data-mailbox-download]'))
    /** @type {ExportFinished} */
    const finished = await exportFolder(request, this._exporting.signal, (messages, bytesLabel) => this._exportProgress(messages, bytesLabel), link)
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(finished, false)
  }

  /** @param {number} messages @param {string} bytesLabel */
  _exportProgress(messages, bytesLabel) {
    /** @type {ExportProgressed} */
    const progressed = { messages, bytesLabel }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(progressed, false)
  }

  /** @param {HTMLInputElement} input @returns {File[]} */
  _takeFiles(input) {
    const files = [...(input.files ?? [])]
    input.value = ''
    return files
  }

  /** @param {File[]} files */
  async _import(files) {
    this._importing = new AbortController()
    /** @type {ImportFinished} */
    const finished = await importFiles(files, this._target, this._importing.signal, (imported, total, current) => this._importProgress(imported, total, current))
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(finished, false)
  }

  /** @param {number} imported @param {number} total @param {string} current */
  _importProgress(imported, total, current) {
    /** @type {ImportProgressed} */
    const progressed = { imported, total, current }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(progressed, false)
  }

  /** @param {OfflineSearchRequested} request */
  async _searchOffline(request) {
    const slices = await searchOffline(request)
    slices.forEach((emails, index) => this._offlineSlice(request.request, emails, index === slices.length - 1))
  }

  /** @param {number} request @param {Array<Record<string, any>>} emails @param {boolean} done */
  _offlineSlice(request, emails, done) {
    /** @type {OfflineSearchFinished} */
    const finished = { request, emails: /** @type {Email[]} */ (/** @type {unknown} */ (emails)), done }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(finished, false)
  }
}
