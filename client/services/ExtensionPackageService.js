/**
 * The extension bridge.
 *
 * Reads the extension package the owner chose. A WebAssembly service cannot
 * read a chosen file, so this service reads it and hands the text to
 * ExtensionsService with ExtensionPackageRead. Nothing is installed here.
 * The browser transport sends at most 64 KiB per request, so a package must
 * leave room for the request around it.
 *
 * It also carries ExtensionRuntimeService's messages: a command goes to the
 * plugin's sandbox iframe (rendered by the shell) with postMessage, and what
 * a sandbox posts back comes to Go as ExtensionAnswered, only from that
 * sandbox's own window. It sets the enabled theme's CSS variables on <html>.
 */
@Service
export default class ExtensionPackageService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;

  static _LIMIT = 60 * 1024

  ready = false

  static _THEME = {
    '--ms-error': value => document.documentElement.style.setProperty('--ms-error', value),
    '--ms-on-primary': value => document.documentElement.style.setProperty('--ms-on-primary', value),
    '--ms-primary': value => document.documentElement.style.setProperty('--ms-primary', value),
    '--ms-primary-container': value => document.documentElement.style.setProperty('--ms-primary-container', value),
    '--ms-surface': value => document.documentElement.style.setProperty('--ms-surface', value),
    '--ms-surface-container': value => document.documentElement.style.setProperty('--ms-surface-container', value),
    '--ms-surface-container-high': value => document.documentElement.style.setProperty('--ms-surface-container-high', value),
    '--ms-on-surface': value => document.documentElement.style.setProperty('--ms-on-surface', value),
    '--ms-on-surface-variant': value => document.documentElement.style.setProperty('--ms-on-surface-variant', value),
  }

  @OnStart
  start() {
    addEventListener('message', event => this._received(event))
    this.ready = true
  }

  /** @param {ExtensionThemeApplied} applied */
  @OnEvent({"replay":true})
  theme(applied) {
    Object.entries(ExtensionPackageService._THEME).forEach(([name, paint]) => paint(this._value(applied.variables, name)))
  }

  /** @param {ExtensionCommandRequested} request */
  @OnEvent
  command(request) {
    this._sandbox(request.extensionId)?.postMessage({
      channel: 'caduceus-extension-host', type: 'command', extensionId: request.extensionId, sessionNonce: request.nonce,
      requestId: request.requestId, command: { id: request.commandId, slot: request.slot }, context: { slot: request.slot },
    }, '*')
  }

  /** @param {Event} event */
  async read(event) {
    const file = /** @type {HTMLInputElement} */ (event.target).files?.[0]
    const problem = ExtensionPackageService._problem(file)
    /** @type {ExtensionPackageRead} */
    const read = { text: await ExtensionPackageService._text(file, problem), problem }
    this.applicationEventPublisher.publishEvent(read, false)
  }

  /** @param {Record<string, string> | undefined} variables @param {string} name */
  _value(variables, name) {
    return variables?.[name] || ''
  }

  /** @param {string} extensionId @returns {Window | null | undefined} */
  _sandbox(extensionId) {
    return /** @type {HTMLIFrameElement | null} */ (document.querySelector(`iframe[data-extension-id="${CSS.escape(extensionId)}"]`))?.contentWindow
  }

  /** @param {MessageEvent} event */
  _received(event) {
    const message = event.data || {}
    if ([message.channel === 'caduceus-extension', typeof message.extensionId === 'string', event.source === this._sandbox(message.extensionId)].every(Boolean)) {
      /** @type {ExtensionAnswered} */
      const answered = {
        extensionId: message.extensionId, nonce: String(message.sessionNonce || ''), kind: String(message.type || ''),
        requestId: String(message.requestId || ''), result: JSON.stringify(message.result ?? null), error: String(message.error || ''),
      }
      this.applicationEventPublisher.publishEvent(answered, false)
    }
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
