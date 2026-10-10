import { postOneClickUnsubscribe } from '../shared/scripts/unsubscribe.js'
import {
  openPrintConversation,
  printableConversation,
  renderPrintDocument,
} from '../shared/scripts/print-conversation.js'

/**
 * The browser side of the Go message reader. Go services cannot reach the
 * document, the location, timers, new windows, downloads or a cross-origin
 * POST, so they publish small events and this service carries them out. It
 * reads large values (a download, a compose body, the print conversation)
 * from the Go services' fields, because one event holds at most 64 KiB.
 */
@Service
export default class BrowserService {
  static _PREFILL_KEY = 'caduceus_compose_prefill'
  static _SNOOZE_UNDO_KEY = 'caduceus_pending_snooze_undo'
  static _SIDEBAR_KEY = 'caduceus-participant-sidebar'
  static _TARGET_KEY = 'caduceus-translation-target'
  static _UNDO_MILLISECONDS = 10000
  static _OPEN = 'open'
  static _NEW_TAB = '_blank'
  static _SAFE_FEATURES = 'noopener,noreferrer'
  static _DARK = 'dark'

  /** @type {HTMLElement} */
  @Needs
  view
  /** @type {TacTaskScheduler} */
  @Needs
  taskScheduler
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher
  /** @type {MessageService} */
  message
  /** @type {ConversationService} */
  conversation
  /** @type {CategoryPickerService} */
  categoryPicker
  /** @type {SnoozeService} */
  snooze
  printing = false
  _account = ''

  @OnStart
  start() {
    this.view.addEventListener('load', event => this._frameLoaded(event), true)
    /** @type {ReaderOpened} */
    const opened = {
      path: location.pathname,
      search: location.search,
      origin: location.origin,
      theme: this._theme(),
      translationTarget: this._localValue(BrowserService._TARGET_KEY),
      participantSidebar: this._localValue(BrowserService._SIDEBAR_KEY) === BrowserService._OPEN,
    }
    this.applicationEventPublisher.publishEvent(opened, true)
  }

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  session(changed) {
    this._account = changed.email
  }

  /** @param {ReaderChanged} _changed */
  @OnEvent
  changed(_changed) {
    this.taskScheduler.render()
  }

  /** @param {DownloadRequested} _requested */
  @OnEvent
  async download(_requested) {
    await this.taskScheduler.render()
    this.view.querySelector('a[data-message-download]')?.click()
  }

  /** @param {ExternalLinkRequested} requested */
  @OnEvent
  externalLink(requested) {
    window.open(requested.url, BrowserService._NEW_TAB, BrowserService._SAFE_FEATURES)
  }

  /** @param {ComposeRequested} requested */
  @OnEvent
  compose(requested) {
    const prefill = this._prefill(requested.owner)
    sessionStorage.setItem(BrowserService._PREFILL_KEY, JSON.stringify(this._withoutEmpty(prefill)))
    location.assign('/compose')
  }

  /** @param {NavigationRequested} requested */
  @OnEvent
  navigate(requested) {
    location.assign('/' + requested.target)
  }

  /** @param {EmailOpenRequested} requested */
  @OnEvent
  openEmail(requested) {
    location.assign('/email/' + encodeURIComponent(requested.emailId))
  }

  /** @param {TimerRequested} requested */
  @OnEvent
  timer(requested) {
    setTimeout(() => { void this._timerFired(requested) }, Math.max(0, requested.delayMs))
  }

  /** @param {UnsubscribeRequested} requested */
  @OnEvent
  async unsubscribe(requested) {
    const sent = await postOneClickUnsubscribe(requested.url)
    await this.message.unsubscribeFinished(sent)
    await this.taskScheduler.render()
  }

  /** @param {FindMatchFocused} focused */
  @OnEvent
  async findMatch(focused) {
    await this.taskScheduler.render()
    this.view.querySelector(`[data-find-index="${focused.index}"]`)?.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
    this._focus('[data-find-thread-input]')
  }

  /** @param {FocusRequested} requested */
  @OnEvent
  async focus(requested) {
    await this.taskScheduler.render()
    this._focus(requested.selector)
  }

  /** @param {SnoozeCompleted} completed */
  @OnEvent
  snoozed(completed) {
    const expiresAt = Date.now() + BrowserService._UNDO_MILLISECONDS
    sessionStorage.setItem(BrowserService._SNOOZE_UNDO_KEY, JSON.stringify({ id: completed.snoozeId, message: completed.message, owner: this._account, expiresAt }))
    setTimeout(() => sessionStorage.removeItem(BrowserService._SNOOZE_UNDO_KEY), BrowserService._UNDO_MILLISECONDS)
  }

  /** @param {ReaderPreferenceChanged} changed */
  @OnEvent
  preference(changed) {
    const keys = { participantSidebar: BrowserService._SIDEBAR_KEY, translationTarget: BrowserService._TARGET_KEY }
    this._storeLocal(keys[/** @type {'participantSidebar'|'translationTarget'} */ (changed.name)], changed.value)
  }

  /**
   * Ask before a destructive or third-party action. The template passes the
   * answer to the Go method.
   *
   * @param {string} question
   * @returns {boolean}
   */
  confirm(question) {
    return window.confirm(question)
  }

  /**
   * Print the complete conversation. The window is reserved inside the click
   * so the browser's popup protection allows it.
   *
   * @param {Event} _event
   */
  printConversation(_event) {
    const popup = window.open('', BrowserService._NEW_TAB, 'popup')
    this._print(popup)
  }

  /**
   * Keyboard movement inside the category picker listbox: Escape closes it,
   * the arrows, Home and End move between the options.
   *
   * @param {KeyboardEvent} event
   */
  async pickerKeys(event) {
    await this._pickerKey(event)
  }

  /**
   * Submit the custom snooze time without leaving the page.
   *
   * @param {SubmitEvent} event
   * @param {string} until
   * @param {string} untilLabel
   */
  async submitSnooze(event, until, untilLabel) {
    event.preventDefault()
    await this.snooze.submitCustom(until, untilLabel)
  }

  /** @returns {string} */
  _theme() {
    const root = document.documentElement
    return [root.getAttribute('w-theme'), root.getAttribute('data-theme')].includes(BrowserService._DARK) ? BrowserService._DARK : 'light'
  }

  /** @param {string} key @returns {string} */
  _localValue(key) {
    return String(localStorage.getItem(key) ?? '')
  }

  /** @param {string} key @param {string} value */
  _storeLocal(key, value) {
    if (key) localStorage.setItem(key, value)
  }

  /** @param {string} owner @returns {Record<string, unknown>} */
  _prefill(owner) {
    const services = { message: this.message, conversation: this.conversation }
    return /** @type {Record<string, unknown>} */ (services[/** @type {'message'|'conversation'} */ (owner)]?.composePrefill ?? {})
  }

  /** @param {Record<string, unknown>} prefill @returns {Record<string, unknown>} */
  _withoutEmpty(prefill) {
    return Object.fromEntries(Object.entries(prefill).filter(([, value]) => this._present(value)))
  }

  /** @param {unknown} value @returns {boolean} */
  _present(value) {
    if (Array.isArray(value)) return value.length > 0
    return !['', null, undefined].includes(/** @type {any} */ (value))
  }

  /** @param {TimerRequested} requested */
  async _timerFired(requested) {
    await this.message.timerFired(requested.name, requested.key)
    await this.taskScheduler.render()
  }

  /** @param {string} selector */
  _focus(selector) {
    /** @type {HTMLElement | null} */ (this.view.querySelector(selector))?.focus()
  }

  /** @param {Event} event */
  _frameLoaded(event) {
    const frame = event.target
    if (frame instanceof HTMLIFrameElement && frame.classList.contains('email-html-frame')) this._watchLinks(frame)
  }

  /** @param {HTMLIFrameElement} frame */
  _watchLinks(frame) {
    frame.contentDocument?.addEventListener('click', click => { void this._linkClicked(click) })
  }

  /** @param {MouseEvent} click */
  async _linkClicked(click) {
    const link = /** @type {any} */ (click.target)?.closest?.('a[data-caduceus-link-warning]')
    if (link) await this._warn(click, link)
  }

  /** @param {MouseEvent} click @param {Element} link */
  async _warn(click, link) {
    click.preventDefault()
    click.stopPropagation()
    const [reason, target, host, labelHost] = this._linkFields(link)
    await this.message.showLinkWarning(reason, target, host, labelHost)
    await this.taskScheduler.render()
  }

  /** @param {Element} link @returns {string[]} */
  _linkFields(link) {
    const names = ['data-caduceus-link-warning', 'data-caduceus-link-target', 'data-caduceus-link-host', 'data-caduceus-link-label-host']
    return names.map(name => this._attribute(link, name))
  }

  /** @param {Element} link @param {string} name @returns {string} */
  _attribute(link, name) {
    return String(link.getAttribute(name) ?? '')
  }

  /** @param {Window | null} popup */
  _print(popup) {
    if (!popup) {
      this._toast('Allow pop-ups to print this conversation.')
      return
    }
    this.printing = true
    this._printInto(popup)
  }

  /** @param {Window} popup */
  _printInto(popup) {
    const conversation = printableConversation(this.message.email, this.conversation.messages)
    openPrintConversation(renderPrintDocument(conversation, { account: this._account }), window, popup)
    this.printing = false
  }

  /** @param {string} message */
  _toast(message) {
    /** @type {ToastRequested} */
    const requested = { message }
    this.applicationEventPublisher.publishEvent(requested)
  }

  /** @param {KeyboardEvent} event */
  async _pickerKey(event) {
    const moves = { ArrowDown: 1, ArrowUp: -1, Home: -Infinity, End: Infinity }
    if (event.key === 'Escape') await this._closePicker(event)
    else if (Object.hasOwn(moves, event.key)) this._moveOption(event, moves[/** @type {'ArrowDown'} */ (event.key)])
  }

  /** @param {KeyboardEvent} event */
  async _closePicker(event) {
    event.preventDefault()
    await this.categoryPicker.close()
    await this.taskScheduler.render()
  }

  /** @param {KeyboardEvent} event @param {number} move */
  _moveOption(event, move) {
    const options = [...this.view.querySelectorAll('[data-detail-category-picker] .detail-category-picker-option:not([disabled])')]
    event.preventDefault()
    const start = this._optionStart(options.indexOf(/** @type {Element} */ (document.activeElement)), move, options.length)
    const index = Math.min(options.length - 1, Math.max(0, start + move))
    /** @type {HTMLElement | undefined} */ (options[index])?.focus()
  }

  /** @param {number} current @param {number} move @param {number} length @returns {number} */
  _optionStart(current, move, length) {
    if (current < 0 && move < 0) return length
    return current
  }
}
