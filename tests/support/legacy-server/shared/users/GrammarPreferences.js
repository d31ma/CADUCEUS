export class GrammarPreferences {
  static GRAMMAR_LANGUAGES = ['auto', 'en-US', 'en-GB']

  static #LANGUAGE_SET = new Set(GrammarPreferences.GRAMMAR_LANGUAGES)

  static #RULE_ID = /^[A-Z0-9_-]{1,64}$/

  static #MAX_DISABLED_RULES = 100

  /** Normalize persisted grammar preferences to safe standalone defaults. @param {{grammarCheckEnabled?: unknown, grammarCheckLanguage?: unknown, grammarCheckDisabledRules?: unknown}} [value] */
  static normalizeGrammarPreferences(value = {}) {
    return {
      grammarCheckEnabled: value.grammarCheckEnabled === true,
      grammarCheckLanguage: GrammarPreferences.#LANGUAGE_SET.has(/** @type {string} */ (value.grammarCheckLanguage)) ? /** @type {'auto' | 'en-US' | 'en-GB'} */ (value.grammarCheckLanguage) : 'auto',
      grammarCheckDisabledRules: GrammarPreferences.#normalizeRules(value.grammarCheckDisabledRules) || [],
    }
  }

  /** Validate a caller patch without coercing strings or unknown rule IDs. @param {{grammarCheckEnabled?: unknown, grammarCheckLanguage?: unknown, grammarCheckDisabledRules?: unknown}} value */
  static validateGrammarPreferencePatch(value) {
    if (value.grammarCheckEnabled !== undefined && typeof value.grammarCheckEnabled !== 'boolean') {
      return { ok: false, error: 'grammarCheckEnabled must be a boolean' }
    }
    if (value.grammarCheckLanguage !== undefined && !GrammarPreferences.#LANGUAGE_SET.has(/** @type {string} */ (value.grammarCheckLanguage))) {
      return { ok: false, error: 'grammarCheckLanguage must be one of: auto, en-US, en-GB' }
    }
    if (value.grammarCheckDisabledRules !== undefined && !GrammarPreferences.#normalizeRules(value.grammarCheckDisabledRules)) {
      return { ok: false, error: `grammarCheckDisabledRules must contain at most ${GrammarPreferences.#MAX_DISABLED_RULES} valid rule identifiers` }
    }
    return { ok: true }
  }

  /** @param {unknown} value */
  static #normalizeRules(value) {
    if (!Array.isArray(value) || value.length > GrammarPreferences.#MAX_DISABLED_RULES) return null
    /** @type {string[]} */
    const rules = []
    for (const item of value) {
      if (typeof item !== 'string' || !GrammarPreferences.#RULE_ID.test(item)) return null
      if (!rules.includes(item)) rules.push(item)
    }
    return rules
  }
}
