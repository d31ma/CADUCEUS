/** @import { RawRuleDoc, InboxRule } from '../../types' */
import { Fylo } from '../persistence/Fylo.js'

export class RulesRecords {
  /** @param {RawRuleDoc} raw @returns {InboxRule} */
  static deserialize(raw) {
    return {
      ...raw,
      ...(raw.ownerEmail ? { ownerEmail: String(raw.ownerEmail).toLowerCase() } : {}),
      ...(raw.mailbox ? { mailbox: String(raw.mailbox).toLowerCase() } : {}),
      order: Number(raw.order) || 0,
      stopProcessing: Boolean(raw.stopProcessing),
      conditions: typeof raw.conditions === 'string' ? JSON.parse(raw.conditions) : (raw.conditions ?? []),
      actions: typeof raw.actions === 'string' ? JSON.parse(raw.actions) : (raw.actions ?? []),
    }
  }

  /** @param {InboxRule} left @param {InboxRule} right */
  static compareRules(left, right) {
    return (Number(left.order) || 0) - (Number(right.order) || 0)
      || left.name.localeCompare(right.name)
      || left.id.localeCompare(right.id)
  }

  /** @param {Omit<InboxRule, 'id'> & { id?: string }} rule @returns {RawRuleDoc} */
  static serialize(rule) {
    return /** @type {RawRuleDoc} */ ({
      ...rule,
      conditions: JSON.stringify(rule.conditions ?? []),
      actions: JSON.stringify(rule.actions ?? []),
    })
  }
}
