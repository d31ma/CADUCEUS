import { Guards } from '../../../middleware/guards.js'
import { Respond } from '../../../platform/http/respond.js'
import { getApp } from '../../../app.js'
import { GrammarCheckService } from '../../../features/mailbox/services/grammar-check.js'

const MAX_TEXT_LENGTH = 20_000

@RequestRoute("/grammar/check")
@Public
export class GrammarCheckController {
  /** Run one bounded, authenticated, self-hosted grammar check. */
  @Guards.auth()
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    if (!body || typeof body.text !== 'string') return Respond.r400('text must be a string')
    if (body.text.length > MAX_TEXT_LENGTH) return Respond.r400(`text may contain at most ${MAX_TEXT_LENGTH} characters`)
    const language = body.language ?? 'auto'
    if (!GrammarCheckService.isLanguage(language)) return Respond.r400('language must be one of: auto, en-US, en-GB')
    const disabledRules = body.disabledRules === undefined ? [] : GrammarCheckService.normalizeDisabledRules(body.disabledRules)
    if (!disabledRules) return Respond.r400('disabledRules must contain at most 100 valid rule identifiers')

    const app = await getApp()
    const limit = await app.rateLimit.check(['grammar-check', auth.email], 120, 60_000)
    if (!limit.allowed) return Respond.r429('Grammar check rate limit reached', limit.retryAfterSeconds)
    return app.grammarCheck.check(body.text, { language, disabledRules })
  }
}
