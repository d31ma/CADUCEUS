import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { RuleValidation } from '../shared/domains/RuleValidation.js'
import { evaluateInboxRule } from '../shared/mailbox/rule-engine.js'
import { normalizeInboxRuleInput } from '../shared/mailbox/rule-input.js'
import { ownedSenderAddresses } from '../shared/users/sender-addresses.js'

@RequestRoute("/rules/preview")
@SignedIn
export class RulesPreviewController {
  /** @param {Record<string, unknown>} message */
  static validateMessage(message) {
    for (const field of ['from', 'to', 'cc', 'subject']) {
      if (message[field] !== undefined && typeof message[field] !== 'string' && !Array.isArray(message[field])) return `${field} must be text or a list`
      if (String(message[field] ?? '').length > 5_000) return `${field} is too long`
    }
    if (message.body !== undefined && (typeof message.body !== 'string' || message.body.length > 100_000)) return 'body is too long'
    if (message.hasAttachment !== undefined && typeof message.hasAttachment !== 'boolean') return 'hasAttachment must be a boolean'
    return ''
  }

  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
      if (!body?.message || typeof body.message !== 'object' || Array.isArray(body.message)) return Respond.r400('message required')
      const messageError = RulesPreviewController.validateMessage(body.message)
      if (messageError) return Respond.r400(messageError)
      const app = await getApp()
      let rule
      if (body.ruleId) {
        const [, existing] = await app.rules.findById(String(body.ruleId))
        if (!existing || existing.ownerEmail !== auth.email.toLowerCase()) return Respond.r404('Rule not found')
        rule = existing
      } else {
        const owned = await ownedSenderAddresses(app.users, auth.email)
        const mailbox = typeof body.rule?.mailbox === 'string' ? body.rule.mailbox.trim().toLowerCase() : auth.email
        if (!owned.includes(mailbox)) return Respond.r403('Mailbox access denied')
        rule = normalizeInboxRuleInput(body.rule ?? {}, { ownerEmail: auth.email, mailbox, order: 0 })
        const result = RuleValidation.validateInboxRule(rule)
        if (result.valid === false) return Respond.r400(result.error)
      }
      return evaluateInboxRule(/** @type {any} */ (rule), body.message)
    })
  }
}
