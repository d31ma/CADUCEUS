/**
 * Escape hatch: the page facts and page actions that Go services cannot reach.
 *
 * Tachyon gives Go services no location, no navigation and no feature
 * detection, so this service publishes the current address and the browser's
 * features once, and performs the navigations that Go asks for.
 */
@Service
export default class BrowserPageService {
  static _FUNCTION = 'function'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  @OnStart
  start() {
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
    this.applicationEventPublisher.publishEvent(place, true)
    this.applicationEventPublisher.publishEvent(features, true)
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

  /** @param {NavigationRequested} request */
  _go(request) {
    if (request.replace) location.replace(request.target)
    else location.assign(request.target)
  }
}
