import { RouteRequest } from '../middleware/RouteRequest.js'
import { Actions } from '../platform/auth/Auth.js'
import { randomBytes } from 'node:crypto'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { RuleValidation } from '../features/domains/services/RuleValidation.js'
import { ownedSenderAddresses } from '../features/users/services/sender-addresses.js'
import { normalizeInboxRuleInput } from '../features/mailbox/services/rule-input.js'

@RequestRoute("/rules")
@SignedIn
export class RulesController {
  /**
 * POST /rules
 * @param {object} params
 * @param {{ name: string, domain: string, enabled?: boolean, conditionMatch?: string, conditions?: object[], actions?: object[] }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
    const app = await getApp()
    const owned = await ownedSenderAddresses(app.users, auth.email)
    const mailbox = typeof body?.mailbox === 'string' ? body.mailbox.trim().toLowerCase() : auth.email
    if (!owned.includes(mailbox)) return Respond.r403('Mailbox access denied')
    const ruleId = randomBytes(16).toString('hex')
    const rule = normalizeInboxRuleInput(body ?? {}, {
      id: ruleId,
      ownerEmail: auth.email,
      mailbox,
      order: await app.rules.nextOrder(auth.email),
    })
    const result = RuleValidation.validateInboxRule(rule)
    if (result.valid === false) return Respond.r400(result.error)
    if (rule.actions.some(action => action.type === 'forward' && action.to === mailbox)) {
      return Respond.r400('A rule cannot forward a message back to its own mailbox')
    }
    await app.rules.put(/** @type {any} */ (rule))
    return { id: ruleId }
    })
  }

  /**
 * GET /rules
 * @param {object} params
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ auth }) => {
    const app = await getApp()
    return await app.rules.listForOwner(auth.email)
    })
  }
}
