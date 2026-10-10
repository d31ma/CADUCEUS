/** An error carrying the boundary's stable code. */
export class AuthBridgeError extends Error {
  /** @param {string} code @param {string} message */
  constructor(code, message) {
    super(message)
    this.name = 'AuthBridgeError'
    this.code = code
  }

  /** HTTP status for this failure. Unknown codes are never a success. */
  get status() {
    switch (this.code) {
      case 'EBADREQUEST':    return 400
      case 'EUNSUPPORTED':   return 400
      case 'EUNAUTHORIZED':  return 401
      case 'EFORBIDDEN':     return 403
      case 'EPOLICY':        return 403
      case 'ENOTFOUND':      return 404
      case 'ECONFLICT':      return 409
      case 'EAUTHREQUIRED':  return 422
      default:               return 503
    }
  }
}
