/**
 * Escape hatch: the page facts and page actions that Go services cannot reach.
 *
 * Tachyon gives Go services no location, navigation, document element,
 * timers, focus, drag data, platform or time zone facts, or global key and
 * click listeners. This service
 * publishes those facts as events and performs the actions Go asks for. It
 * makes no decision of its own beyond reading the DOM.
 */
@Service
export default class BrowserPageService {
  static _FUNCTION = 'function'
  static _ATTRIBUTE_CACHE = 'caduceus-document-attributes'
  static _DRAG_TYPE = 'application/x-caduceus-mail-ids'
  static _DRAGGING = 'drag-over'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false

  /** @type {Map<string, number>} */
  _timers = new Map()

  /** @type {HTMLElement | null} */
  _returnFocus = null

  @OnStart
  start() {
    this._replayAttributes()
    /** @type {PageLocation} */
    const place = { path: location.pathname, search: location.search, hash: location.hash }
    /** @type {BrowserFeatures} */
    const features = {
      passkeys: typeof PublicKeyCredential !== 'undefined',
      serviceWorker: 'serviceWorker' in navigator,
      launchQueue: typeof window.launchQueue?.setConsumer === BrowserPageService._FUNCTION,
      appBadge: typeof navigator.setAppBadge === BrowserPageService._FUNCTION,
      notifications: 'Notification' in window,
      pushDisabled: window.__CADUCEUS_DISABLE_SW === true,
    }
    /** @type {DevicePlatform} */
    const platform = {
      apple: /Mac|iPhone|iPad/i.test(navigator.platform),
      timeZoneOffset: -new Date().getTimezoneOffset(),
      origin: location.origin,
    }
    this.applicationEventPublisher.publishEvent(place, true)
    this.applicationEventPublisher.publishEvent(features, true)
    this.applicationEventPublisher.publishEvent(platform, true)
    document.addEventListener('click', event => this._clicked(this._element(event.target), event))
    document.addEventListener('keydown', event => this._escape(event))
    this.ready = true
  }

  /**
   * Tachyon serves every route as its own document, so a navigation is a
   * document load. It waits one task so that storage writes published just
   * before it land first.
   * @param {NavigationRequested} request
   */
  @OnEvent
  navigate(request) {
    setTimeout(() => this._go(request), 0)
  }

  /** @param {DocumentAttributesChanged} changed */
  @OnEvent
  attributes(changed) {
    this._apply(changed.attributes)
    this._cache(changed.owner, changed.attributes)
  }

  /** @param {TimerRequested} request */
  @OnEvent
  timer(request) {
    clearTimeout(this._timers.get(request.name))
    clearInterval(this._timers.get(request.name))
    /** @type {TimerElapsed} */
    const elapsed = { name: request.name }
    this._timers.set(request.name, this._schedule(request.repeat)(() => this.applicationEventPublisher.publishEvent(elapsed, false), request.delayMs))
  }

  /** @param {FocusRequested} request */
  @OnEvent
  focus(request) {
    this._returnFocus = [this._returnFocus, document.activeElement][Number(request.remember)]
    setTimeout(() => this._focus(request), 0)
  }

  /** @param {AddressReplaced} replaced */
  @OnEvent
  replace(replaced) {
    history.replaceState(history.state, '', replaced.target)
  }

  /** @param {ClickRequested} request */
  @OnEvent
  click(request) {
    setTimeout(() => this._click(request.selector), 0)
  }

  /** @param {DragEvent} event */
  dragOver(event) {
    event.preventDefault()
    /** @type {HTMLElement} */ (event.currentTarget).classList.add(BrowserPageService._DRAGGING)
  }

  /** @param {DragEvent} event */
  dragLeave(event) {
    /** @type {HTMLElement} */ (event.currentTarget).classList.remove(BrowserPageService._DRAGGING)
  }

  /** @param {string} folder @param {DragEvent} event @param {string} [domain] */
  drop(folder, event, domain = '') {
    event.preventDefault()
    /** @type {HTMLElement} */ (event.currentTarget).classList.remove(BrowserPageService._DRAGGING)
    /** @type {FolderDropped} */
    const dropped = { folder, domain, ids: this._ids(this._listed(event.dataTransfer)) }
    this.applicationEventPublisher.publishEvent(dropped, false)
  }

  /** @param {MouseEvent} event @param {string} dialog */
  backdrop(event, dialog) {
    /** @type {BackdropClicked} */
    const clicked = { dialog: [ '', dialog ][Number(event.target === event.currentTarget)] }
    this.applicationEventPublisher.publishEvent(clicked, false)
  }

  /** @param {SubmitEvent} event @param {string} form */
  submit(event, form) {
    event.preventDefault()
    /** @type {FormSubmitted} */
    const submitted = { form }
    this.applicationEventPublisher.publishEvent(submitted, false)
  }

  /** @param {NavigationRequested} request */
  _go(request) {
    if (request.reload) location.reload()
    else if (request.replace) location.replace(request.target)
    else location.assign(request.target)
  }

  /** @param {string} selector */
  _click(selector) {
    const target = document.querySelector(selector)
    if (target instanceof HTMLElement) target.click()
  }

  /** @param {Record<string, string>} attributes */
  _apply(attributes) {
    Object.entries(attributes).forEach(([name, value]) => this._set(name, value))
  }

  /** @param {string} name @param {string} value */
  _set(name, value) {
    if (value) document.documentElement.setAttribute(name, value)
    else document.documentElement.removeAttribute(name)
  }

  /** @param {boolean} repeat */
  _schedule(repeat) {
    return repeat ? setInterval : setTimeout
  }

  /** @param {string} owner @param {Record<string, string>} attributes */
  _cache(owner, attributes) {
    const cached = this._cached()
    cached[owner] = attributes
    localStorage.setItem(BrowserPageService._ATTRIBUTE_CACHE, JSON.stringify(cached))
  }

  /** Apply the attributes Go chose on the previous page before Go has loaded. */
  _replayAttributes() {
    for (const attributes of Object.values(this._cached())) this._apply(attributes)
  }

  /** @returns {Record<string, Record<string, string>>} */
  _cached() {
    return JSON.parse(localStorage.getItem(BrowserPageService._ATTRIBUTE_CACHE) || '{}')
  }

  /** @param {FocusRequested} request */
  _focus(request) {
    const pick = [() => document.querySelector(request.selector), () => this._returnFocus][Number(request.restore)]
    const target = pick()
    if (target instanceof HTMLElement && target.isConnected) target.focus()
  }

  /** @param {DataTransfer | null} transfer @returns {unknown[]} */
  _listed(transfer) {
    const encoded = transfer?.getData(BrowserPageService._DRAG_TYPE)
    return encoded ? JSON.parse(encoded) : [transfer?.getData('text/plain')]
  }

  /** @param {unknown[]} listed @returns {string[]} */
  _ids(listed) {
    return [...new Set(listed.filter(id => typeof id === 'string').filter(Boolean))]
  }

  /** @param {EventTarget | null} target */
  _element(target) {
    return target instanceof Element ? target : null
  }

  /** @param {Element | null} target @param {MouseEvent} event */
  _clicked(target, event) {
    /** @type {PageClicked} */
    const clicked = { insideUserMenu: Boolean(target?.closest('.app-user-menu')) }
    this.applicationEventPublisher.publishEvent(clicked, false)
    if (target?.closest('[data-sign-out]')) this._signOut(event)
  }

  /** @param {MouseEvent} event */
  _signOut(event) {
    event.preventDefault()
    /** @type {SignOutRequested} */
    const request = { reason: 'user' }
    this.applicationEventPublisher.publishEvent(request, false)
  }

  /** @param {KeyboardEvent} event */
  _escape(event) {
    /** @type {EscapePressed} */
    const pressed = { inside: '' }
    if (event.key === 'Escape') this.applicationEventPublisher.publishEvent(pressed, false)
  }
}
