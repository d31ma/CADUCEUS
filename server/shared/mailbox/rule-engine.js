const STRING_FIELDS = new Set(['from', 'to', 'cc', 'subject', 'body'])

/**
 * Evaluate one normalized inbox rule without side effects. The same function
 * powers delivery and the authenticated preview endpoint so a preview cannot
 * drift from production matching behavior.
 *
 * @param {import('../../types').InboxRule} rule
 * @param {Record<string, unknown>} input
 */
export function evaluateInboxRule(rule, input) {
  const message = normalizeRuleMessage(input)
  const conditionResults = (rule.conditions ?? []).map(condition => ({
    ...condition,
    matched: conditionMatches(condition, message),
  }))
  const matched = conditionResults.length === 0
    || (rule.conditionMatch === 'all'
      ? conditionResults.every(result => result.matched)
      : conditionResults.some(result => result.matched))
  return {
    matched,
    conditionResults,
    plannedActions: matched ? (rule.actions ?? []).map(action => ({ ...action })) : [],
    stopProcessing: Boolean(matched && rule.stopProcessing),
  }
}

/** @param {Record<string, unknown>} input */
export function normalizeRuleMessage(input) {
  return {
    from: text(input.from ?? input.sender),
    to: text(input.to ?? input.recipient),
    cc: text(input.cc),
    subject: text(input.subject),
    body: text(input.body),
    hasAttachment: Boolean(input.hasAttachment),
  }
}

/** @param {import('../../types').RuleCondition} condition @param {ReturnType<typeof normalizeRuleMessage>} message */
function conditionMatches(condition, message) {
  if (condition.field === 'hasAttachment') {
    const expected = String(condition.value).toLowerCase() === 'true'
    return condition.op === 'notEquals'
      ? message.hasAttachment !== expected
      : message.hasAttachment === expected
  }
  if (!STRING_FIELDS.has(condition.field)) return false
  const haystack = message[condition.field]
  const needle = String(condition.value ?? '').toLocaleLowerCase()
  const value = haystack.toLocaleLowerCase()
  switch (condition.op) {
    case 'contains': return value.includes(needle)
    case 'notContains': return !value.includes(needle)
    case 'equals': return value === needle
    case 'notEquals': return value !== needle
    case 'startsWith': return value.startsWith(needle)
    case 'endsWith': return value.endsWith(needle)
    default: return false
  }
}

/** @param {unknown} value */
function text(value) {
  if (Array.isArray(value)) return value.map(item => String(item ?? '')).join(', ')
  return String(value ?? '')
}
