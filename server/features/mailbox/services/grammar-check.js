const MAX_TEXT_LENGTH = 20_000
const MAX_MATCHES = 100
const VALID_LANGUAGES = new Set(['auto', 'en-US', 'en-GB'])
const RULE_ID = /^[A-Z0-9_-]{1,64}$/

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
    if (text.length > MAX_TEXT_LENGTH) throw new Error(`text may contain at most ${MAX_TEXT_LENGTH} characters`)
    const language = VALID_LANGUAGES.has(options.language || '') ? options.language : 'auto'
    const detectedLanguage = detectLanguage(text, language)
    const disabled = new Set((Array.isArray(options.disabledRules) ? options.disabledRules : [])
      .filter(rule => typeof rule === 'string' && RULE_ID.test(rule)).slice(0, 100))
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
        context: contextFor(text, offset, length),
      })
    }

    scan(text, /\b([A-Za-z][A-Za-z'-]*)\s+\1\b/gi, match => {
      const secondOffset = match.index + match[0].lastIndexOf(match[1])
      add(secondOffset, match[1].length, 'REPEATED_WORD', 'This word is repeated.', [], 'STYLE', 'style')
    })
    scan(text, /(^|[^A-Za-z])i(?=$|[^A-Za-z])/g, match => {
      add(match.index + match[1].length, 1, 'LOWERCASE_I', 'The pronoun “I” should be capitalized.', ['I'], 'CASING')
    })
    scan(text, /[ \t]{2,}/g, match => {
      add(match.index, match[0].length, 'MULTIPLE_SPACES', 'Use a single space here.', [' '], 'STYLE', 'style')
    })
    scan(text, /\s+([,!?;:])/g, match => {
      add(match.index, match[0].length, 'SPACE_BEFORE_PUNCTUATION', 'Remove the space before punctuation.', [match[1]], 'PUNCTUATION')
    })
    scan(text, /([.!?])([A-Za-z])/g, match => {
      add(match.index, match[0].length, 'MISSING_SPACE_AFTER_PUNCTUATION', 'Add a space after punctuation.', [`${match[1]} ${match[2]}`], 'PUNCTUATION')
    })
    scan(text, /\b(could|should|would|must|might) of\b/gi, match => {
      add(match.index, match[0].length, 'MODAL_OF', `Use “${match[1]} have” instead.`, [`${match[1]} have`])
    })
    scan(text, /\b(your) welcome\b/gi, match => {
      add(match.index, match[0].length, 'YOUR_WELCOME', 'Use “you’re” for “you are”.', [preserveCase(match[1], "you're") + ' welcome'])
    })
    scan(text, /\b(he|she|it|this|that) (are|were|have)\b/gi, match => {
      const replacements = { are: 'is', were: 'was', have: 'has' }
      const verb = replacements[match[2].toLowerCase()]
      add(match.index, match[0].length, 'SINGULAR_VERB_AGREEMENT', 'Use a singular verb with this subject.', [`${match[1]} ${verb}`])
    })
    scan(text, /\b(they|we|you) (is|was|has)\b/gi, match => {
      const replacements = { is: 'are', was: 'were', has: 'have' }
      const verb = replacements[match[2].toLowerCase()]
      add(match.index, match[0].length, 'PLURAL_VERB_AGREEMENT', 'Use a plural verb with this subject.', [`${match[1]} ${verb}`])
    })
    scan(text, /\b(a)\s+([aeiou][A-Za-z'-]*)\b/gi, match => {
      add(match.index, match[0].length, 'A_VOWEL_SOUND', 'Use “an” before this vowel sound.', [`an ${match[2]}`])
    })
    scan(text, /\b(an)\s+([bcdfgjklmnpqrstvwxyz][A-Za-z'-]*)\b/gi, match => {
      add(match.index, match[0].length, 'AN_CONSONANT_SOUND', 'Use “a” before this consonant sound.', [`a ${match[2]}`])
    })
    scan(text, /(^|[.!?]\s+)([a-z])/g, match => {
      const offset = match.index + match[1].length
      add(offset, 1, 'SENTENCE_CAPITALIZATION', 'Start this sentence with a capital letter.', [match[2].toUpperCase()], 'CASING')
    })

    const nonOverlapping = matches
      .sort((a, b) => a.offset - b.offset || b.length - a.length || a.ruleId.localeCompare(b.ruleId))
      .filter((match, index, all) => index === 0 || match.offset >= all[index - 1].offset + all[index - 1].length)
      .slice(0, MAX_MATCHES)
    return { language, detectedLanguage, matches: nonOverlapping }
  }

  static isLanguage(value) { return VALID_LANGUAGES.has(value) }
  static normalizeDisabledRules(value) {
    if (!Array.isArray(value) || value.length > 100) return null
    const rules = []
    for (const item of value) {
      if (typeof item !== 'string' || !RULE_ID.test(item)) return null
      if (!rules.includes(item)) rules.push(item)
    }
    return rules
  }
}

function scan(text, regex, callback) {
  for (const match of text.matchAll(regex)) callback(match)
}

function detectLanguage(text, requested) {
  if (requested !== 'auto') return requested
  const letters = text.match(/\p{L}/gu) || []
  if (!letters.length) return 'en-US'
  const latin = letters.filter(letter => /[A-Za-z]/.test(letter)).length
  return latin / letters.length >= 0.8 ? 'en-US' : 'und'
}

function preserveCase(source, replacement) {
  return source[0] === source[0].toUpperCase()
    ? replacement[0].toUpperCase() + replacement.slice(1)
    : replacement
}

function contextFor(text, offset, length) {
  const start = Math.max(0, offset - 30)
  const end = Math.min(text.length, offset + length + 30)
  return { text: text.slice(start, end), offset: offset - start, length }
}

