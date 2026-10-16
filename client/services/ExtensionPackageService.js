/**
 * The extension bridge.
 *
 * Reads the extension package the owner chose. A WebAssembly service cannot
 * read a chosen file, so this service reads it and hands the text to
 * ExtensionsService with ExtensionPackageRead. Nothing is installed here.
 * The browser transport sends at most 64 KiB per request, so a package must
 * leave room for the request around it.
 *
 * It also carries ExtensionRuntimeService's messages to the sandbox host, the
 * one opaque iframe the shell renders: the enabled plugins' sources (again
 * whenever the host says it is ready) and their commands, by postMessage.
 * What the host posts back comes to Go as ExtensionAnswered, only from the
 * host's own window. It sets the enabled theme's CSS variables on <html>.
 */
@Service
export default class ExtensionPackageService {
  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher;

  static _LIMIT = 60 * 1024

  ready = false

  /** @type {number} */
  _selection = 0

  /** @type {ExtensionSandbox[]} */
  _plugins = []

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

  /** @param {ExtensionPluginsLoaded} loaded */
  @OnEvent({"replay":true})
  plugins(loaded) {
    this._plugins = loaded.plugins
    this._loadPlugins()
  }

  /** @param {ExtensionCommandRequested} request */
  @OnEvent
  command(request) {
    this._post({
      type: 'command', extensionId: request.extensionId, sessionNonce: request.nonce, requestId: request.requestId,
      command: { id: request.commandId, slot: request.slot }, context: { slot: request.slot },
    })
  }

  /** @param {Event} event */
  async read(event) {
    const selection = ++this._selection
    const file = /** @type {HTMLInputElement} */ (event.target).files?.[0]
    const problem = ExtensionPackageService._problem(file)
    /** @type {ExtensionPackageRead} */
    const read = await ExtensionPackageService._readFile(file, problem)
    this._publishRead(selection, read)
  }

  /** @param {number} selection @param {ExtensionPackageRead} read */
  _publishRead(selection, read) {
    if (selection === this._selection) /** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(read, false)
  }

  _loadPlugins() {
    this._post({ type: 'reset' })
    this._plugins.forEach(plugin => this._post({ type: 'load', extensionId: plugin.id, sessionNonce: plugin.nonce, source: plugin.source }))
  }

  /** @param {Record<string, unknown>} message */
  _post(message) {
    this._host().forEach(host => host.postMessage({ channel: 'caduceus-extension-host', ...message }, '*'))
  }

  /** @param {Record<string, string>} variables @param {string} name */
  _value(variables, name) {
    return variables[name] || ''
  }

  /** @returns {Window[]} */
  _host() {
    return Array.from(document.querySelectorAll('iframe[data-extension-host]'), frame => /** @type {HTMLIFrameElement} */ (frame).contentWindow).filter(host => host !== null)
  }

  /** @param {MessageEvent} event */
  _received(event) {
    const message = Object(event.data)
    const steps = { 'caduceus-extension-host-ready': () => this._loadPlugins(), 'caduceus-extension': () => this._answer(message) }
    const channel = ['', String(message.channel)][Number(this._host().includes(/** @type {Window} */ (event.source)))]
    ;[() => {}, steps[channel]].filter(Boolean).pop()()
  }

  /** @param {object} message */
  _answer(message) {
    /** @type {ExtensionAnswered} */
    const answered = { message: JSON.stringify(message) }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(answered, false)
  }

  /** @param {File | undefined} file */
  static _problem(file) {
    return Number(file?.size) > ExtensionPackageService._LIMIT ? 'Extension package exceeds 60 KiB.' : ''
  }

  /** @param {File | undefined} file @param {string} problem @returns {Promise<ExtensionPackageRead>} */
  static async _readFile(file, problem) {
    try { return { text: await ExtensionPackageService._text(file, problem), problem } }
    catch { return { text: '', problem: 'Extension package could not be read.' } }
  }

  /** @param {File | undefined} file @param {string} problem @returns {Promise<string>} */
  static _text(file, problem) {
    return [file, problem === ''].every(Boolean) ? /** @type {File} */ (file).text() : Promise.resolve('')
  }
}
