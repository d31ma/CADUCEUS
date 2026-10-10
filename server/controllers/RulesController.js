import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Auth } from '../shared/auth/Auth.js'
import { randomBytes } from 'node:crypto'
import { Respond } from '../shared/http/Respond.js'
import { RuleValidation } from '../shared/domains/RuleValidation.js'
import { SenderAddresses } from '../shared/users/SenderAddresses.js'
import { RuleInput } from '../shared/mailbox/RuleInput.js'

@RequestRoute("/rules")
@SignedIn
export class RulesController {
  /** @type {RulesHelper} */
  @Needs
  rulesHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /**
 * POST /rules
 * @param {object} params
 * @param {{ name: string, domain: string, enabled?: boolean, conditionMatch?: string, conditions?: object[], actions?: object[] }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: { action: Auth.Actions.MAILBOX_WRITE } }, async ({ body, auth }) => {
    const app = this
    const owned = await SenderAddresses.ownedSenderAddresses(app.usersHelper, auth.email)
    const mailbox = typeof body?.mailbox === 'string' ? body.mailbox.trim().toLowerCase() : auth.email
    if (!owned.includes(mailbox)) return Respond.r403('Mailbox access denied')
    const ruleId = randomBytes(16).toString('hex')
    const rule = RuleInput.normalizeInboxRuleInput(body ?? {}, {
      id: ruleId,
      ownerEmail: auth.email,
      mailbox,
      order: await app.rulesHelper.nextOrder(auth.email),
    })
    const result = RuleValidation.validateInboxRule(rule)
    if (result.valid === false) return Respond.r400(result.error)
    if (rule.actions.some(action => action.type === 'forward' && action.to === mailbox)) {
      return Respond.r400('A rule cannot forward a message back to its own mailbox')
    }
    await app.rulesHelper.put(/** @type {any} */ (rule))
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
    const app = this
    return await app.rulesHelper.listForOwner(auth.email)
    })
  }
}
