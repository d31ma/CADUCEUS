import { buildSafeHtmlFrame, SAFE_HTML_FRAME_TAGS, safeHtmlFrameKeepsTag } from '../shared/scripts/safe-html-frame.js'
/** @import { SafeHtmlFramePackage } from '../models/SafeHtmlFramePackage.js' */

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
  /** @type {string} */
  static _QUOTED = 'data-caduceus-quoted'
  /** @type {string} */
  static _READER_ROOT = 'data-caduceus-reader-root'
  /** @type {string} */
  static _READER_MODE = 'data-caduceus-reader-mode'
  /** @type {string} */
  static _READER_QUOTED = 'data-caduceus-reader-quoted'
  /** @type {string} */
  static _LINK_TARGET = 'data-caduceus-link-target'
  /** @type {string} */
  static _LINK_HOST = 'data-caduceus-link-host'
  /** @type {string} */
  static _LINK_LABEL_HOST = 'data-caduceus-link-label-host'
  /** @type {string} */
  static _DARK = 'dark'
  /** @type {string} */
  static _DIV = 'div'
  /** @type {string} */
  static _TRUE = 'true'
  /** @type {string} */
  static _LINK = 'a'

  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher

  ready = false

  /** @type {SafeHtmlFramePackage | null} */
  _protectedFrame = null
  /** @type {string} */
  _protectedDocument = ''
  /** @type {number} */
  _protectedRevision = -1

  /** @param {string} documentHtml @param {number} revision @returns {SafeHtmlFramePackage} */
  protectedFrame(documentHtml, revision) {
    const current = this._frameIsCurrent(documentHtml, revision)
    return this._frameForState(current, documentHtml, revision)
  }

  /** @param {string} documentHtml @param {number} revision @returns {boolean} */
  _frameIsCurrent(documentHtml, revision) {
    return [this._protectedFrame !== null, this._protectedDocument === documentHtml, this._protectedRevision === revision].every(Boolean)
  }

  /** @param {boolean} current @param {string} documentHtml @param {number} revision @returns {SafeHtmlFramePackage} */
  _frameForState(current, documentHtml, revision) {
    if (current) return /** @type {SafeHtmlFramePackage} */ (this._protectedFrame)
    return this._createProtectedFrame(documentHtml, revision)
  }

  /** @param {string} documentHtml @param {number} revision @returns {SafeHtmlFramePackage} */
  _createProtectedFrame(documentHtml, revision) {
    const frame = buildSafeHtmlFrame(documentHtml, revision, {
      metadataAttributes: this._readerMetadataNames(),
      prepare: parsed => this._prepareReaderBody(parsed),
      authorize: element => this._readerMetadata(element),
    })
    this._protectedDocument = documentHtml
    this._protectedRevision = revision
    this._protectedFrame = frame
    return frame
  }

  /** @returns {Record<string,string[]>} */
  _readerMetadataNames() {
    const names = Object.fromEntries(SAFE_HTML_FRAME_TAGS.map(tag => [tag, [ReaderBrowserService._QUOTED]]))
    names[ReaderBrowserService._DIV] = ['class', ReaderBrowserService._QUOTED]
    names[ReaderBrowserService._LINK] = ['class', ReaderBrowserService._QUOTED, ...this._readerWarningNames()]
    return names
  }

  /** @returns {string[]} */
  _readerWarningNames() {
    return [ReaderBrowserService._WARNING, ReaderBrowserService._LINK_TARGET, ReaderBrowserService._LINK_HOST, ReaderBrowserService._LINK_LABEL_HOST]
  }

  /** @param {Document} parsed */
  _prepareReaderBody(parsed) {
    const quoted = parsed.body.classList.contains('show-quoted')
    const style = this._readerStyle(parsed)
    const dark = style.startsWith('html{color-scheme:dark;')
    const mode = this._readerPalette(dark)
    const elements = this._readerQuotedElements(parsed)
    this._wrapReaderQuotes(parsed, elements)
    this._appendReaderRoot(parsed, mode, quoted)
  }

  /** @param {Document} parsed @returns {string} */
  _readerStyle(parsed) {
    return parsed.head.querySelector('style')?.textContent || ''
  }

  /** @param {boolean} dark @returns {string} */
  _readerPalette(dark) {
    return dark ? ReaderBrowserService._DARK : 'light'
  }

  /** @param {Document} parsed @returns {Element[]} */
  _readerQuotedElements(parsed) {
    return [...parsed.body.querySelectorAll(`[${ReaderBrowserService._QUOTED}]`)].filter(this._readerQuoteNeedsWrapper)
  }

  /** @param {Element} element @returns {boolean} */
  _readerQuoteNeedsWrapper(element) {
    return [safeHtmlFrameKeepsTag(element.localName), ![ReaderBrowserService._LINK, 'img'].includes(element.localName)].includes(false)
  }

  /** @param {Document} parsed @param {Element[]} elements */
  _wrapReaderQuotes(parsed, elements) {
    for (const element of elements) this._wrapReaderQuote(parsed, element)
  }

  /** @param {Document} parsed @param {Element} element */
  _wrapReaderQuote(parsed, element) {
    const wrapper = parsed.createElement(ReaderBrowserService._DIV)
    wrapper.setAttribute(ReaderBrowserService._QUOTED, '')
    element.removeAttribute(ReaderBrowserService._QUOTED)
    element.parentNode?.insertBefore(wrapper, element)
    wrapper.append(element)
  }

  /** @param {Document} parsed @param {string} mode @param {boolean} quoted */
  _appendReaderRoot(parsed, mode, quoted) {
    const root = parsed.createElement(ReaderBrowserService._DIV)
    root.setAttribute(ReaderBrowserService._READER_ROOT, ReaderBrowserService._TRUE)
    root.setAttribute(ReaderBrowserService._READER_MODE, mode)
    root.setAttribute(ReaderBrowserService._READER_QUOTED, String(quoted))
    root.append(...parsed.body.childNodes)
    parsed.body.append(root)
  }

  /** @param {Element} element @returns {Record<string,string>} */
  _readerMetadata(element) {
    const root = this._readerIsRoot(element)
    const classes = this._readerRootClasses(element)
    const rootAttributes = this._readerRootMetadata(root, classes)
    const quotedAttributes = this._readerQuotedMetadata(element.hasAttribute(ReaderBrowserService._QUOTED))
    const warning = this._readerIsWarning(element)
    const warningValues = this._readerWarningValues(element)
    const warningAttributes = this._readerWarningMetadata(warning, warningValues)
    return { ...rootAttributes, ...quotedAttributes, ...warningAttributes }
  }

  /** @param {Element} element @returns {boolean} */
  _readerIsRoot(element) {
    return [element.localName === ReaderBrowserService._DIV, element.getAttribute(ReaderBrowserService._READER_ROOT) === ReaderBrowserService._TRUE, element.parentElement?.localName === 'body'].every(Boolean)
  }

  /** @param {Element} element @returns {string} */
  _readerRootClasses(element) {
    const mode = this._readerPalette(element.getAttribute(ReaderBrowserService._READER_MODE) === ReaderBrowserService._DARK)
    const quoted = this._readerQuotedClass(element.getAttribute(ReaderBrowserService._READER_QUOTED) === ReaderBrowserService._TRUE)
    return 'caduceus-reader-root caduceus-reader-' + mode + quoted
  }

  /** @param {boolean} quoted @returns {string} */
  _readerQuotedClass(quoted) {
    return quoted ? ' show-quoted' : ''
  }

  /** @param {boolean} root @param {string} classes @returns {Record<string,string>} */
  _readerRootMetadata(root, classes) {
    if (root) return { class: classes }
    return {}
  }

  /** @param {boolean} quoted @returns {Record<string,string>} */
  _readerQuotedMetadata(quoted) {
    if (quoted) return { [ReaderBrowserService._QUOTED]: '' }
    return {}
  }

  /** @param {Element} element @returns {boolean} */
  _readerIsWarning(element) {
    return [element.localName === ReaderBrowserService._LINK, element.hasAttribute(ReaderBrowserService._WARNING)].every(Boolean)
  }

  /** @param {Element} element @returns {Record<string,string>} */
  _readerWarningValues(element) {
    return Object.fromEntries(this._readerWarningNames().map(name => [name, this._attribute(element, name)]))
  }

  /** @param {boolean} warning @param {Record<string,string>} values @returns {Record<string,string>} */
  _readerWarningMetadata(warning, values) {
    if (warning) return { class: 'caduceus-suspicious-link', ...values }
    return {}
  }

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
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(answered, false)
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
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(requested, false)
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
      target: this._attribute(link, ReaderBrowserService._LINK_TARGET),
      destinationHost: this._attribute(link, ReaderBrowserService._LINK_HOST),
      displayedHost: this._attribute(link, ReaderBrowserService._LINK_LABEL_HOST),
    }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(warning, false)
  }

  /** @param {Element} link @param {string} name @returns {string} */
  _attribute(link, name) {
    return [link.getAttribute(name)].filter(value => typeof value === 'string').concat('')[0]
  }
}
