export class GrammarCheckRules {
  static MAX_TEXT_LENGTH = 20_000

  static MAX_MATCHES = 100

  static VALID_LANGUAGES = new Set(['auto', 'en-US', 'en-GB'])

  static RULE_ID = /^[A-Z0-9_-]{1,64}$/

  static scan(text, regex, callback) {
    for (const match of text.matchAll(regex)) callback(match)
  }

  static detectLanguage(text, requested) {
    if (requested !== 'auto') return requested
    const letters = text.match(/\p{L}/gu) || []
    if (!letters.length) return 'en-US'
    const latin = letters.filter(letter => /[A-Za-z]/.test(letter)).length
    return latin / letters.length >= 0.8 ? 'en-US' : 'und'
  }

  static preserveCase(source, replacement) {
    return source[0] === source[0].toUpperCase()
      ? replacement[0].toUpperCase() + replacement.slice(1)
      : replacement
  }

  static contextFor(text, offset, length) {
    const start = Math.max(0, offset - 30)
    const end = Math.min(text.length, offset + length + 30)
    return { text: text.slice(start, end), offset: offset - start, length }
  }

  static isLanguage(value) { return GrammarCheckRules.VALID_LANGUAGES.has(value) }

  static normalizeDisabledRules(value) {
    if (!Array.isArray(value) || value.length > 100) return null
    const rules = []
    for (const item of value) {
      if (typeof item !== 'string' || !GrammarCheckRules.RULE_ID.test(item)) return null
      if (!rules.includes(item)) rules.push(item)
    }
    return rules
  }
}
