import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

const GrammarCheck = Routes.GrammarCheckController

beforeAll(setupRoutes, 300_000)
afterAll(teardownRoutes)

describe('authenticated local grammar-check route', () => {
  it('requires authentication and validates bounded input and language', async () => {
    expect((await call(GrammarCheck, 'POST', req({ body: { text: 'i am here' } }))).unauthorized).toBe(true)
    expect((await call(GrammarCheck, 'POST', req({ token: token(), body: {} }))).error).toContain('text')
    expect((await call(GrammarCheck, 'POST', req({ token: token(), body: { text: 'hello', language: 'fr' } }))).error).toContain('language')
    expect((await call(GrammarCheck, 'POST', req({ token: token(), body: { text: 'hello', disabledRules: ['../bad'] } }))).error).toContain('disabledRules')
  })

  it('returns corrections without persisting or contacting another service', async () => {
    const result = await call(GrammarCheck, 'POST', req({
      token: token(),
      body: { text: 'i think she are ready', language: 'auto', disabledRules: [] },
    }))
    expect(result.detectedLanguage).toBe('en-US')
    expect(result.matches.map(match => match.ruleId)).toEqual(['LOWERCASE_I', 'SINGULAR_VERB_AGREEMENT'])
    expect(result.matches[1].replacements).toEqual(['she is'])
  })
})
