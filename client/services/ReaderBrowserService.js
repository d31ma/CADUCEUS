import { postOneClickUnsubscribe } from '../shared/scripts/unsubscribe.js'
import {
  openPrintConversation,
  printableConversation,
  renderPrintDocument,
} from '../shared/scripts/print-conversation.js'

/**
 * Escape hatch: the message reader's browser-only pieces. Go services cannot
 * listen inside the sandboxed message frame, send a cross-origin POST, open
 * a print window inside the click or scroll to a find match, so this service
 * does those and talks to Go in events. The print data comes from the
 * hidden [data-print-conversation] element that the Go services fill.
 */
@Service
export default class ReaderBrowserService {
  static _FRAME = 'email-html-frame'
  static _WARNING = 'data-caduceus-link-warning'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false

  @OnStart
  start() {
    document.addEventListener('load', event => this._frameLoaded(event.target), true)
    this.ready = true
  }

  /** @param {UnsubscribeRequested} requested */
  @OnEvent
  async unsubscribe(requested) {
    /** @type {UnsubscribeAnswered} */
    const answered = { sent: await postOneClickUnsubscribe(requested.url) }
    this.applicationEventPublisher.publishEvent(answered, false)
  }

  /** @param {ShortcutPressed} pressed */
  @OnEvent
  shortcut(pressed) {
    this._printWhen(pressed.command === 'core:print-thread')
  }

  /** @param {FindMatchFocused} focused */
  @OnEvent
  findMatch(focused) {
    setTimeout(() => document.querySelector(`[data-find-index="${focused.index}"]`)?.scrollIntoView?.({ block: 'center', behavior: 'smooth' }), 0)
  }

  /**
   * Print the whole conversation. The window opens inside the click, so the
   * browser's popup protection allows it.
   *
   * @param {Event} _event
   */
  print(_event) {
    this._printOrWarn(window.open('', '_blank', 'popup'))
  }

  /** @param {boolean} asked */
  _printWhen(asked) {
    if (asked) this.print(new Event('print'))
  }

  /** @param {Window | null} popup */
  _printOrWarn(popup) {
    if (popup) this._printInto(popup)
    else this._toast('Allow pop-ups to print this conversation.')
  }

  /** @param {Window} popup */
  _printInto(popup) {
    const element = /** @type {HTMLElement | null} */ (document.querySelector('[data-print-conversation]'))
    const data = { email: '{}', messages: '[]', account: '', ...element?.dataset }
    openPrintConversation(renderPrintDocument(printableConversation(JSON.parse(data.email), JSON.parse(data.messages)), { account: data.account }), window, popup)
  }

  /** @param {string} message */
  _toast(message) {
    /** @type {ToastRequested} */
    const requested = { message, actionLabel: '', action: '', subject: '', durationMs: 0 }
    this.applicationEventPublisher.publishEvent(requested, false)
  }

  /** @param {EventTarget | null} target */
  _frameLoaded(target) {
    [target]
      .filter(element => element instanceof HTMLIFrameElement)
      .filter(frame => /** @type {HTMLIFrameElement} */ (frame).classList.contains(ReaderBrowserService._FRAME))
      .forEach(frame => /** @type {HTMLIFrameElement} */ (frame).contentDocument?.addEventListener('click', click => this._linkClicked(click)))
  }

  /**
   * A link whose text disguises its destination asks first. The frame has
   * its own realm, so its nodes fail the parent's instanceof checks.
   *
   * @param {MouseEvent} click
   */
  _linkClicked(click) {
    const link = /** @type {any} */ (click.target)?.closest?.(`a[${ReaderBrowserService._WARNING}]`)
    ;[link].filter(Boolean).forEach(found => this._warn(click, found))
  }

  /** @param {MouseEvent} click @param {Element} link */
  _warn(click, link) {
    click.preventDefault()
    click.stopPropagation()
    /** @type {PendingLinkWarning} */
    const warning = {
      reason: this._attribute(link, ReaderBrowserService._WARNING),
      target: this._attribute(link, 'data-caduceus-link-target'),
      destinationHost: this._attribute(link, 'data-caduceus-link-host'),
      displayedHost: this._attribute(link, 'data-caduceus-link-label-host'),
    }
    this.applicationEventPublisher.publishEvent(warning, false)
  }

  /** @param {Element} link @param {string} name @returns {string} */
  _attribute(link, name) {
    return [link.getAttribute(name)].filter(value => typeof value === 'string').concat('')[0]
  }
}
