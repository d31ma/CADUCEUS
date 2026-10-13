import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/categories")
@SignedIn
export class CategoriesController {
  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** Return exact-owner physical categories and their assigned system roles. */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
      const app = this
      const accounts = await app.categoryMappingService.listAccounts(auth.email, auth.domains)
      if (!accounts) return Respond.r404('User not found')
      return { roles: ['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'], accounts }
    })
  }

  /** Create a nested folder or label in the exact owned mailbox catalog. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth, body }) => {
      const app = this
      const result = await app.categoryMappingService.createCategory(auth.email, body)
      if (!result.ok) {
        if (result.notFound) return Respond.r404(result.error)
        if (result.forbidden) return Respond.r403(result.error)
        return Respond.r400(result.error)
      }
      return { category: result.category }
    })
  }
}
