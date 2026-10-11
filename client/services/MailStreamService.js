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
 * - a frame after a failure, or two failures in a row (an error or close
 *   code 1006), become MailStreamHealth for the notification centre.
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

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  ready = false
  token = ''
  cursor = ''
  /** @type {WebSocket | null} */
  socket = null
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  timer = undefined
  failures = 0
  erred = false
  wait = MailStreamService._FIRST_WAIT_MS

  @OnStart
  start() {
    // Mail that landed between the first list load and this connection is replayed too.
    this.cursor = new Date(Date.now() - MailStreamService._REPLAY_MS).toISOString()
    this.ready = true
  }

  /** @param {SessionChanged} changed */
  @OnEvent({"replay":true})
  session(changed) {
    this._stop()
    this.token = changed.signedIn ? changed.token : ''
    this._connect()
  }

  _connect() {
    if (!this.token) return
    const address = new URL(MailStreamService._PATH, location.href)
    address.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    address.search = new URLSearchParams({ token: this.token, since: this.cursor }).toString()
    const socket = new WebSocket(address)
    socket.addEventListener('message', event => this._frame(socket, event.data))
    socket.addEventListener('error', () => this._erred(socket))
    socket.addEventListener('close', event => this._closed(socket, event.code))
    this.socket = socket
  }

  _stop() {
    clearTimeout(this.timer)
    const socket = this.socket
    this.socket = null
    socket?.close()
  }

  /** @param {WebSocket} socket @param {unknown} text */
  _frame(socket, text) {
    const frame = this._parsed(text)
    if (!frame || socket !== this.socket) return
    this.cursor = frame.cursor ?? this.cursor
    this._healthy()
    const handler = MailStreamService._HANDLERS[frame.event]
    if (handler) this[handler](frame.data)
  }

  /** @param {unknown} text */
  _parsed(text) {
    try {
      return JSON.parse(String(text))
    } catch {
      return null
    }
  }

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
    this.token = ''
    this._stop()
    /** @type {SignOutRequested} */
    const request = { reason: MailStreamService._REASON }
    this.applicationEventPublisher.publishEvent(request, false)
  }

  _healthy() {
    this.wait = MailStreamService._FIRST_WAIT_MS
    if (this.failures >= MailStreamService._FAILURES_SHOWN) this._health(true)
    this.failures = 0
  }

  /** @param {WebSocket} socket */
  _erred(socket) {
    this.erred = this.erred || socket === this.socket
  }

  /** @param {WebSocket} socket @param {number} code */
  _closed(socket, code) {
    if (socket !== this.socket) return
    this._counted(this.erred || code === MailStreamService._ABNORMAL_CLOSE)
    this.erred = false
    this.socket = null
    this.timer = setTimeout(() => this._connect(), this._backoff())
  }

  /** @param {boolean} failed */
  _counted(failed) {
    this.failures = failed ? this.failures + 1 : this.failures
    if (failed && this.failures === MailStreamService._FAILURES_SHOWN) this._health(false)
  }

  _backoff() {
    const wait = this.wait
    this.wait = Math.min(wait * 2, MailStreamService._LONGEST_WAIT_MS)
    return wait
  }

  /** @param {boolean} healthy */
  _health(healthy) {
    /** @type {MailStreamHealth} */
    const health = { healthy }
    this.applicationEventPublisher.publishEvent(health, true)
  }
}
