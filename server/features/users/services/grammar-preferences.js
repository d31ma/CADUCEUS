export const GRAMMAR_LANGUAGES = ['auto', 'en-US', 'en-GB']
const LANGUAGE_SET = new Set(GRAMMAR_LANGUAGES)
const RULE_ID = /^[A-Z0-9_-]{1,64}$/
const MAX_DISABLED_RULES = 100

/** Normalize persisted grammar preferences to safe standalone defaults. */
export function normalizeGrammarPreferences(value = {}) {
  return {
    grammarCheckEnabled: value.grammarCheckEnabled === true,
    grammarCheckLanguage: LANGUAGE_SET.has(value.grammarCheckLanguage) ? value.grammarCheckLanguage : 'auto',
    grammarCheckDisabledRules: normalizeRules(value.grammarCheckDisabledRules) || [],
  }
}

/** Validate a caller patch without coercing strings or unknown rule IDs. */
export function validateGrammarPreferencePatch(value) {
  if (value.grammarCheckEnabled !== undefined && typeof value.grammarCheckEnabled !== 'boolean') {
    return { ok: false, error: 'grammarCheckEnabled must be a boolean' }
  }
  if (value.grammarCheckLanguage !== undefined && !LANGUAGE_SET.has(value.grammarCheckLanguage)) {
    return { ok: false, error: 'grammarCheckLanguage must be one of: auto, en-US, en-GB' }
  }
  if (value.grammarCheckDisabledRules !== undefined && !normalizeRules(value.grammarCheckDisabledRules)) {
    return { ok: false, error: `grammarCheckDisabledRules must contain at most ${MAX_DISABLED_RULES} valid rule identifiers` }
  }
  return { ok: true }
}

function normalizeRules(value) {
  if (!Array.isArray(value) || value.length > MAX_DISABLED_RULES) return null
  const rules = []
  for (const item of value) {
    if (typeof item !== 'string' || !RULE_ID.test(item)) return null
    if (!rules.includes(item)) rules.push(item)
  }
  return rules
}
