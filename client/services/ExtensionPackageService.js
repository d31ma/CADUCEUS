/**
 * Reads the extension package the owner chose. A WebAssembly service cannot
 * read a chosen file, so this service reads it and hands the text to
 * ExtensionsService with ExtensionPackageRead. Nothing is installed here.
 * The browser transport sends at most 64 KiB per request, so a package must
 * leave room for the request around it.
 */
@Service
export default class ExtensionPackageService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;

  static _LIMIT = 60 * 1024

  /** @param {Event} event */
  async read(event) {
    const file = /** @type {HTMLInputElement} */ (event.target).files?.[0]
    const problem = ExtensionPackageService._problem(file)
    /** @type {ExtensionPackageRead} */
    const read = { text: await ExtensionPackageService._text(file, problem), problem }
    this.applicationEventPublisher.publishEvent(read, false)
  }

  /** @param {File | undefined} file */
  static _problem(file) {
    return Number(file?.size) > ExtensionPackageService._LIMIT ? 'Extension package exceeds 60 KiB.' : ''
  }

  /** @param {File | undefined} file @param {string} problem @returns {Promise<string>} */
  static _text(file, problem) {
    return [file, problem === ''].every(Boolean) ? /** @type {File} */ (file).text() : Promise.resolve('')
  }
}
