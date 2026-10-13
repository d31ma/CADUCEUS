import { describe, expect, it } from 'bun:test'
import GrammarCheckService from '@/services/GrammarCheckService.js'
import { GrammarCheckRules } from '@/shared/mailbox/GrammarCheckRules.js'
import { Providers } from '../../scripts/support/Providers.mjs'

describe('standalone LanguageTool-compatible grammar checker', () => {
  it('reports bounded grammar, style, punctuation, and casing issues with exact offsets', () => {
    const text = 'i  think he are ready.We could of left.'
    const result = Providers.create(GrammarCheckService).check(text, { language: 'en-GB' })
    expect(result.language).toBe('en-GB')
    expect(result.detectedLanguage).toBe('en-GB')
    expect(result.matches.map(match => match.ruleId)).toEqual([
      'LOWERCASE_I',
      'MULTIPLE_SPACES',
      'SINGULAR_VERB_AGREEMENT',
      'MISSING_SPACE_AFTER_PUNCTUATION',
      'MODAL_OF',
    ])
    for (const match of result.matches) {
      expect(text.slice(match.offset, match.offset + match.length).length).toBe(match.length)
      expect(match.context.text.slice(match.context.offset, match.context.offset + match.context.length))
        .toBe(text.slice(match.offset, match.offset + match.length))
    }
    expect(result.matches.find(match => match.ruleId === 'SINGULAR_VERB_AGREEMENT').replacements).toEqual(['he is'])
  })

  it('ignores browser spelling, supports disabled rules, and detects unsupported scripts', () => {
    const service = Providers.create(GrammarCheckService)
    expect(service.check('Flibbertigibbet').matches).toEqual([])
    expect(service.check('When i am ready', { disabledRules: ['LOWERCASE_I'] }).matches).toEqual([])
    expect(service.check('Привет мир', { language: 'auto' })).toEqual({
      language: 'auto', detectedLanguage: 'und', matches: [],
    })
  })

  it('validates language, ignored rules, and the text bound', () => {
    expect(GrammarCheckRules.isLanguage('auto')).toBe(true)
    expect(GrammarCheckRules.isLanguage('fr')).toBe(false)
    expect(GrammarCheckRules.normalizeDisabledRules(['LOWERCASE_I', 'LOWERCASE_I'])).toEqual(['LOWERCASE_I'])
    expect(GrammarCheckRules.normalizeDisabledRules(['../../unsafe'])).toBeNull()
    expect(() => Providers.create(GrammarCheckService).check('x'.repeat(20_001))).toThrow('at most 20000')
  })
})
