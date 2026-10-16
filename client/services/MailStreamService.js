/**
 * Escape hatch: the live mail stream, a WebSocket that Go cannot open.
 *
 * While someone is signed in, it keeps `/inbox/stream` open on the page's own
 * origin, with the token and the newest cursor in the address (the socket
 * carries no headers). The server closes it about every 25 seconds, so it
 * reconnects on every close, after 1 s, doubling up to 15 s until a frame
 * arrives. Each JSON frame `{event, data, cursor}`:
 *
 * - new-email becomes NewMailArrived and InboxRefreshRequested (the badges);
 * - unauthorized stops the stream and becomes SignOutRequested;
 * - two failed connections in a row (an error or close code 1006) become
 *   MailStreamHealth for the notification centre, and the next frame clears it.
 */
@Service
export default class MailStreamService {
  static _PATH = '/inbox/stream'
  static _FIRST_WAIT_MS = 1000
  static _LONGEST_WAIT_MS = 15000
  static _REPLAY_MS = 60000
  static _FAILURES_SHOWN = 2
  static _ABNORMAL_CLOSE = 1006
  static _REASON = 'mail-stream'
  static _HANDLERS = { 'new-email': '_arrived', unauthorized: '_unauthorized' }
  static _IGNORED = '_ignored'

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false
  _token = ''
  _cursor = ''
  /** @type {WebSocket | null} */
  _socket = null
  /** @type {AbortController | null} */
  _listening = null
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  _timer = undefined
  _failures = 0
  _erred = false
  _wait = MailStreamService._FIRST_WAIT_MS

  @OnStart
  start() {
    // Mail that landed between the first list load and this connection is replayed too.
    this._cursor = new Date(Date.now() - MailStreamService._REPLAY_MS).toISOString()
    this.ready = true
  }

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  session(changed) {
    this._stop()
    this._token = this._tokenOf(changed)
    this._connect()
  }

  /** @param {SessionChanged} changed */
  _tokenOf(changed) {
    return changed.signedIn ? changed.token : ''
  }

  _connect() {
    if (this._token) this._open()
  }

  _open() {
    const socket = new WebSocket(this._address())
    const listening = new AbortController()
    socket.addEventListener('message', event => this._frame(event.data), { signal: listening.signal })
    socket.addEventListener('error', () => this._markError(), { signal: listening.signal })
    socket.addEventListener('close', event => this._closed(event.code), { signal: listening.signal })
    this._socket = socket
    this._listening = listening
  }

  _address() {
    const address = new URL(MailStreamService._PATH, location.href)
    address.protocol = location.protocol.replace('http', 'ws')
    address.search = new URLSearchParams({ token: this._token, since: this._cursor }).toString()
    return address
  }

  _stop() {
    clearTimeout(this._timer)
    this._listening?.abort()
    this._socket?.close()
    this._socket = null
  }

  /** @param {unknown} text */
  _frame(text) {
    const frame = this._parsed(text)
    this._cursor = this._cursorOf(frame)
    this._healthy()
    this[this._handler(frame)](frame.data)
  }

  /** @param {unknown} text */
  _parsed(text) {
    try {
      return JSON.parse(String(text))
    } catch {
      return {}
    }
  }

  /** @param {{cursor?: string}} frame */
  _cursorOf(frame) {
    return frame.cursor || this._cursor
  }

  /** @param {{event?: string}} frame @returns {string} */
  _handler(frame) {
    return MailStreamService._HANDLERS[frame.event] || MailStreamService._IGNORED
  }

  _ignored() {}

  /** @param {Email} email */
  _arrived(email) {
    if (!email?.id) return
    /** @type {NewMailArrived} */
    const arrived = { email }
    this.applicationEventPublisher.publishEvent(arrived, false)
    /** @type {InboxRefreshRequested} */
    const refresh = { reason: MailStreamService._REASON }
    this.applicationEventPublisher.publishEvent(refresh, false)
  }

  _unauthorized() {
    this._token = ''
    this._stop()
    /** @type {SignOutRequested} */
    const request = { reason: MailStreamService._REASON }
    this.applicationEventPublisher.publishEvent(request, false)
  }

  _healthy() {
    this._wait = MailStreamService._FIRST_WAIT_MS
    if (this._failures >= MailStreamService._FAILURES_SHOWN) this._health(true)
    this._failures = 0
  }

  _markError() {
    this._erred = true
  }

  /** @param {number} code */
  _closed(code) {
    const failed = [this._erred, code === MailStreamService._ABNORMAL_CLOSE].includes(true)
    this._failures += Number(failed)
    this._failing([failed, this._failures === MailStreamService._FAILURES_SHOWN].every(Boolean))
    this._erred = false
    this._socket = null
    this._timer = setTimeout(() => this._connect(), this._backoff())
  }

  /** @param {boolean} shown */
  _failing(shown) {
    if (shown) this._health(false)
  }

  _backoff() {
    const wait = this._wait
    this._wait = Math.min(wait * 2, MailStreamService._LONGEST_WAIT_MS)
    return wait
  }

  /** @param {boolean} healthy */
  _health(healthy) {
    /** @type {MailStreamHealth} */
    const health = { healthy }
    this.applicationEventPublisher.publishEvent(health, true)
  }
}
