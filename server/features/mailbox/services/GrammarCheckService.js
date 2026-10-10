
import { GrammarCheckRules } from '../../../shared/mailbox/GrammarCheckRules.js'

/**
 * Bounded standalone grammar checker with a LanguageTool-compatible result
 * shape. It deliberately leaves spelling to the browser and reports grammar,
 * punctuation, casing, and style issues only.
 */
export class GrammarCheckService {
  /**
   * @param {string} text
   * @param {{ language?: string, disabledRules?: string[] }} [options]
   */
  check(text, options = {}) {
    if (typeof text !== 'string') throw new Error('text must be a string')
    if (text.length > GrammarCheckRules.MAX_TEXT_LENGTH) throw new Error(`text may contain at most ${GrammarCheckRules.MAX_TEXT_LENGTH} characters`)
    const language = GrammarCheckRules.VALID_LANGUAGES.has(options.language || '') ? options.language : 'auto'
    const detectedLanguage = GrammarCheckRules.detectLanguage(text, language)
    const disabled = new Set((Array.isArray(options.disabledRules) ? options.disabledRules : [])
      .filter(rule => typeof rule === 'string' && GrammarCheckRules.RULE_ID.test(rule)).slice(0, 100))
    if (detectedLanguage === 'und') return { language, detectedLanguage, matches: [] }

    /** @type {GrammarMatch[]} */
    const matches = []
    const add = (offset, length, ruleId, message, replacements, category = 'GRAMMAR', issueType = 'grammar') => {
      if (disabled.has(ruleId) || length < 1 || offset < 0 || offset + length > text.length) return
      matches.push({
        offset,
        length,
        message,
        shortMessage: category === 'STYLE' ? 'Style' : category === 'PUNCTUATION' ? 'Punctuation' : 'Grammar',
        replacements: [...new Set(replacements.map(value => String(value)).filter(Boolean))].slice(0, 5),
        ruleId,
        ruleDescription: message,
        category,
        categoryName: category[0] + category.slice(1).toLowerCase(),
        issueType,
        context: GrammarCheckRules.contextFor(text, offset, length),
      })
    }

    GrammarCheckRules.scan(text, /\b([A-Za-z][A-Za-z\x27-]*)\s+\1\b/gi, match => {
      const secondOffset = match.index + match[0].lastIndexOf(match[1])
      add(secondOffset, match[1].length, 'REPEATED_WORD', 'This word is repeated.', [], 'STYLE', 'style')
    })
    GrammarCheckRules.scan(text, /(^|[^A-Za-z])i(?=$|[^A-Za-z])/g, match => {
      add(match.index + match[1].length, 1, 'LOWERCASE_I', 'The pronoun “I” should be capitalized.', ['I'], 'CASING')
    })
    GrammarCheckRules.scan(text, /[ \t]{2,}/g, match => {
      add(match.index, match[0].length, 'MULTIPLE_SPACES', 'Use a single space here.', [' '], 'STYLE', 'style')
    })
    GrammarCheckRules.scan(text, /\s+([,!?;:])/g, match => {
      add(match.index, match[0].length, 'SPACE_BEFORE_PUNCTUATION', 'Remove the space before punctuation.', [match[1]], 'PUNCTUATION')
    })
    GrammarCheckRules.scan(text, /([.!?])([A-Za-z])/g, match => {
      add(match.index, match[0].length, 'MISSING_SPACE_AFTER_PUNCTUATION', 'Add a space after punctuation.', [`${match[1]} ${match[2]}`], 'PUNCTUATION')
    })
    GrammarCheckRules.scan(text, /\b(could|should|would|must|might) of\b/gi, match => {
      add(match.index, match[0].length, 'MODAL_OF', `Use “${match[1]} have” instead.`, [`${match[1]} have`])
    })
    GrammarCheckRules.scan(text, /\b(your) welcome\b/gi, match => {
      add(match.index, match[0].length, 'YOUR_WELCOME', 'Use “you’re” for “you are”.', [GrammarCheckRules.preserveCase(match[1], "you're") + ' welcome'])
    })
    GrammarCheckRules.scan(text, /\b(he|she|it|this|that) (are|were|have)\b/gi, match => {
      const replacements = { are: 'is', were: 'was', have: 'has' }
      const verb = replacements[match[2].toLowerCase()]
      add(match.index, match[0].length, 'SINGULAR_VERB_AGREEMENT', 'Use a singular verb with this subject.', [`${match[1]} ${verb}`])
    })
    GrammarCheckRules.scan(text, /\b(they|we|you) (is|was|has)\b/gi, match => {
      const replacements = { is: 'are', was: 'were', has: 'have' }
      const verb = replacements[match[2].toLowerCase()]
      add(match.index, match[0].length, 'PLURAL_VERB_AGREEMENT', 'Use a plural verb with this subject.', [`${match[1]} ${verb}`])
    })
    GrammarCheckRules.scan(text, /\b(a)\s+([aeiou][A-Za-z\x27-]*)\b/gi, match => {
      add(match.index, match[0].length, 'A_VOWEL_SOUND', 'Use “an” before this vowel sound.', [`an ${match[2]}`])
    })
    GrammarCheckRules.scan(text, /\b(an)\s+([bcdfgjklmnpqrstvwxyz][A-Za-z\x27-]*)\b/gi, match => {
      add(match.index, match[0].length, 'AN_CONSONANT_SOUND', 'Use “a” before this consonant sound.', [`a ${match[2]}`])
    })
    GrammarCheckRules.scan(text, /(^|[.!?]\s+)([a-z])/g, match => {
      const offset = match.index + match[1].length
      add(offset, 1, 'SENTENCE_CAPITALIZATION', 'Start this sentence with a capital letter.', [match[2].toUpperCase()], 'CASING')
    })

    const nonOverlapping = matches
      .sort((a, b) => a.offset - b.offset || b.length - a.length || a.ruleId.localeCompare(b.ruleId))
      .filter((match, index, all) => index === 0 || match.offset >= all[index - 1].offset + all[index - 1].length)
      .slice(0, GrammarCheckRules.MAX_MATCHES)
    return { language, detectedLanguage, matches: nonOverlapping }
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

