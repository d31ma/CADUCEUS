import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { GrammarCheckRules } from '../shared/mailbox/GrammarCheckRules.js'

@RequestRoute("/grammar/check")
@SignedIn
export class GrammarCheckController {
  /** @type {GrammarCheckService} */
  @Needs
  grammarCheckService

  /** @type {RateLimitService} */
  @Needs
  rateLimitService

  static MAX_TEXT_LENGTH = 20_000

  /** Run one bounded, authenticated, self-hosted grammar check. */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (!body || typeof body.text !== 'string') return Respond.r400('text must be a string')
      if (body.text.length > GrammarCheckController.MAX_TEXT_LENGTH) return Respond.r400(`text may contain at most ${GrammarCheckController.MAX_TEXT_LENGTH} characters`)
      const language = body.language ?? 'auto'
      if (!GrammarCheckRules.isLanguage(language)) return Respond.r400('language must be one of: auto, en-US, en-GB')
      const disabledRules = body.disabledRules === undefined ? [] : GrammarCheckRules.normalizeDisabledRules(body.disabledRules)
      if (!disabledRules) return Respond.r400('disabledRules must contain at most 100 valid rule identifiers')

      const app = this
      const limit = await app.rateLimitService.check(['grammar-check', auth.email], 120, 60_000)
      if (!limit.allowed) return Respond.r429('Grammar check rate limit reached', limit.retryAfterSeconds)
      return app.grammarCheckService.check(body.text, { language, disabledRules })
    })
  }
}
