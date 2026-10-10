import { AuthGuard } from '../shared/http/AuthGuard.js'
import { RouteRequest } from '../shared/http/RouteRequest.js'

/**
 * Tells Tachyon who is signed in before a `@SignedIn` route runs. The Go
 * authorization boundary verifies the bearer token; Tachyon answers 401 when
 * it does not. Per-action checks stay with the route (see {@link RouteRequest}).
 */
@IdentityProvider
export class CaduceusIdentityProvider {
  /**
   * @param {Request} request
   * @returns {Promise<{ user: string, roles: string[] } | null>}
   */
  async identify(request) {
    const { context } = await RouteRequest.read(new Request(request.url, { headers: request.headers }))
    const result = await AuthGuard.requireAuth(context)
    return result.ok ? { user: result.principal.address, roles: [] } : null
  }
}
