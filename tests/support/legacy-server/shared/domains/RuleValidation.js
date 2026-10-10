import { Security } from '../security/Security.js'

/** Validation for domain route rules and inbox rule actions. */
export class RuleValidation {
  static #RULE_FIELDS = new Set(['from', 'to', 'cc', 'subject', 'body', 'hasAttachment'])

  static #STRING_OPERATORS = new Set(['contains', 'notContains', 'equals', 'notEquals', 'startsWith', 'endsWith'])

  static #MAX_CONDITIONS = 20

  static #MAX_ACTIONS = 10

  /**
   * @param {RouteAction} action
   * @returns {ValidationResult}
   */
  static validateRouteAction(action) {
    if (!action || typeof action !== 'object') return { valid: false, error: 'Action is required' }
    switch (action.type) {
      case 'webhook': {
        if (!action.url || typeof action.url !== 'string') return { valid: false, error: 'Webhook URL is required' }
        const result = Security.assertSafeWebhookUrl(action.url)
        if (!result.ok) return { valid: false, error: result.error }
        return { valid: true }
      }
      case 'forward': {
        if (!action.to || typeof action.to !== 'string') return { valid: false, error: 'Forward target is required' }
        return { valid: true }
      }
      case 'store':
      case 'drop':
        return { valid: true }
      default:
        return { valid: false, error: 'Unknown action type' }
    }
  }

  /**
   * @param {Array<{ id: string, match: string, action: RouteAction, enabled: boolean }>} rules
   * @returns {ValidationResult}
   */
  static validateRouteRules(rules) {
    if (!Array.isArray(rules)) return { valid: false, error: 'Routes must be an array' }
    const ids = new Set()
    for (const rule of rules) {
      if (!rule.id || typeof rule.id !== 'string') return { valid: false, error: 'Each route must have an id' }
      if (ids.has(rule.id)) return { valid: false, error: `Duplicate route id: ${rule.id}` }
      ids.add(rule.id)
      if (typeof rule.match !== 'string' || !rule.match.trim()) return { valid: false, error: `Route ${rule.id}: match is required` }
      const actionResult = RuleValidation.validateRouteAction(rule.action)
      if (!actionResult.valid) return { valid: false, error: `Route ${rule.id}: ${actionResult.error}` }
    }
    return { valid: true }
  }

  /**
   * @param {Array<{ type: string, folder?: string, to?: string, value?: boolean }>} actions
   * @returns {ValidationResult}
   */
  static validateInboxRuleActions(actions) {
    if (!Array.isArray(actions) || actions.length === 0) return { valid: false, error: 'At least one action is required' }
    if (actions.length > RuleValidation.#MAX_ACTIONS) return { valid: false, error: `At most ${RuleValidation.#MAX_ACTIONS} actions are allowed` }
    for (const action of actions) {
      if (!action || typeof action !== 'object' || !action.type) return { valid: false, error: 'Action type is required' }
      if (action.type === 'folder') {
        if (!action.folder || typeof action.folder !== 'string') return { valid: false, error: 'Folder action requires a folder name' }
        const folder = action.folder.trim().toLowerCase()
        if (!folder) return { valid: false, error: 'Folder action requires a folder name' }
        if (folder.length > 60) return { valid: false, error: 'Folder name is too long' }
        if (!/^[a-z0-9][a-z0-9/_-]*$/.test(folder)) return { valid: false, error: 'Folder may contain lowercase letters, numbers, hyphens, underscores, and slashes' }
      }
      if (action.type === 'forward') {
        if (!action.to || typeof action.to !== 'string') return { valid: false, error: 'Forward action requires a to address' }
        if (!Security.normalizeEmailAddress(action.to)) return { valid: false, error: 'Forward action requires a valid email address' }
      }
      if (['markRead', 'star'].includes(action.type) && typeof action.value !== 'boolean') {
        return { valid: false, error: `${action.type} action requires a boolean value` }
      }
      if (!['folder', 'forward', 'delete', 'markRead', 'star'].includes(action.type)) {
        return { valid: false, error: `Unknown action type: ${action.type}` }
      }
    }
    return { valid: true }
  }

  /** @param {Array<{field?:string,op?:string,value?:unknown}>} conditions @returns {ValidationResult} */
  static validateInboxRuleConditions(conditions) {
    if (!Array.isArray(conditions)) return { valid: false, error: 'Conditions must be an array' }
    if (conditions.length > RuleValidation.#MAX_CONDITIONS) return { valid: false, error: `At most ${RuleValidation.#MAX_CONDITIONS} conditions are allowed` }
    for (const condition of conditions) {
      if (!condition || typeof condition !== 'object' || !RuleValidation.#RULE_FIELDS.has(String(condition.field))) {
        return { valid: false, error: `Unknown condition field: ${String(condition?.field ?? '')}` }
      }
      if (!RuleValidation.#STRING_OPERATORS.has(String(condition.op))) {
        return { valid: false, error: `Unknown condition operator: ${String(condition.op ?? '')}` }
      }
      if (condition.field === 'hasAttachment') {
        if (!['equals', 'notEquals'].includes(String(condition.op)) || !['true', 'false'].includes(String(condition.value).toLowerCase())) {
          return { valid: false, error: 'hasAttachment requires equals/notEquals and true/false' }
        }
      } else if (typeof condition.value !== 'string' || !condition.value.trim()) {
        return { valid: false, error: 'Every condition requires a non-empty value' }
      }
      if (String(condition.value).length > 500) return { valid: false, error: 'Condition value is too long' }
    }
    return { valid: true }
  }

  /** @param {Record<string, any>} rule @returns {ValidationResult} */
  static validateInboxRule(rule) {
    if (!rule || typeof rule !== 'object') return { valid: false, error: 'Rule is required' }
    if (typeof rule.name !== 'string' || !rule.name.trim()) return { valid: false, error: 'name required' }
    if (rule.name.trim().length > 200 || /[\0-\x1f\x7f]/.test(rule.name)) return { valid: false, error: 'Rule name is invalid' }
    if (!['all', 'any'].includes(rule.conditionMatch ?? 'all')) return { valid: false, error: 'conditionMatch must be all or any' }
    if (rule.enabled !== undefined && typeof rule.enabled !== 'boolean') return { valid: false, error: 'enabled must be a boolean' }
    if (rule.stopProcessing !== undefined && typeof rule.stopProcessing !== 'boolean') return { valid: false, error: 'stopProcessing must be a boolean' }
    if (rule.order !== undefined && (!Number.isSafeInteger(rule.order) || rule.order < 0 || rule.order > 10_000)) {
      return { valid: false, error: 'order must be an integer from 0 through 10000' }
    }
    const conditions = RuleValidation.validateInboxRuleConditions(rule.conditions ?? [])
    if (!conditions.valid) return conditions
    return RuleValidation.validateInboxRuleActions(rule.actions ?? [])
  }
}
