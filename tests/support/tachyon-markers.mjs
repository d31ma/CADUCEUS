/**
 * Bun preload for the test suites. Tachyon's annotations are globals that only
 * `ty` understands; under `bun test` the same sources load directly, so each
 * marker is a decorator that leaves its target unchanged. `RequestRoute` also
 * keeps its path on the class so a test can build the request URL.
 */
class TachyonMarkers {
  static NAMES = [
    'Allow', 'DeleteRoute', 'ErrorHandler', 'Fake', 'FromBody', 'FromCookie', 'FromHeader', 'FromPath',
    'FromQuery', 'GetRequest', 'GetRoute', 'HandlesError', 'Helper', 'HTTPClient', 'IdentityProvider',
    'Lifetime', 'LocalState', 'Needs', 'NeedsAll', 'OnEvent', 'OnStart', 'OnStop', 'PatchRoute',
    'PostRequest', 'PostRoute', 'Public', 'PutRequest', 'PutRoute', 'DeleteRequest', 'PatchRequest',
    'RequestRoute', 'ResponseStatus', 'Service', 'SessionState', 'Setting', 'Settings', 'SignedIn',
    'Store', 'StreamRoute', 'Test', 'TestSetup', 'RequestBody', 'PathVariable', 'RequestParam', 'RequestHeader',
  ]

  static install() {
    for (const name of TachyonMarkers.NAMES) {
      if (!(name in globalThis)) Object.defineProperty(globalThis, name, { value: TachyonMarkers.#marker(name), configurable: true })
    }
  }

  /** @param {string} name */
  static #marker(name) {
    /** @param {unknown[]} args */
    const decorate = (args) => (/** @type {any} */ target, /** @type {any} */ context) => {
      if (name === 'RequestRoute' && typeof target === 'function') target.tachyonPath = String(args[0] ?? '/')
      return undefined
    }
    return (/** @type {unknown[]} */ ...args) => {
      const bare = args.length >= 2 && args[1] && typeof args[1] === 'object' && 'kind' in /** @type {object} */ (args[1])
      return bare ? decorate([])(args[0], args[1]) : decorate(args)
    }
  }
}

TachyonMarkers.install()
