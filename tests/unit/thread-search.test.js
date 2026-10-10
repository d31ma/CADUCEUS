import { describe, test, expect } from 'bun:test'
import {
  applySearchSuggestion, getCurrentTokenAndTerm, searchTokenSegments,
  staticSearchSuggestions, wrapSearchTerm,
} from '../../client/shared/scripts/thread-search.js'

describe('thread search interaction model', () => {
  test('identifies complete and partial tokens at the caret', () => {
    expect(getCurrentTokenAndTerm('from:alice to:bob', 16)).toMatchObject({
      token: 'to', term: 'bob', index: 11, length: 6,
    })
    expect(getCurrentTokenAndTerm('sub:Quarterly', 5)).toMatchObject({
      token: 'sub', term: 'quarterly', index: 0, length: 13,
    })
    expect(getCurrentTokenAndTerm('plain words', 6)).toEqual({
      token: null, term: null, index: -1, length: 0,
    })
  })

  test('returns semantic token segments without changing the query text', () => {
    const segments = searchTokenSegments('from:alice AND subject:"Hello World"')
    expect(segments.map(segment => segment.text).join('')).toBe('from:alice AND subject:"Hello World"')
    expect(segments.filter(segment => segment.token).map(segment => segment.text)).toEqual([
      'from:', 'subject:',
    ])
  })

  test('replaces the token under the caret and safely quotes suggested values', () => {
    expect(wrapSearchTerm('Alice "A" Example')).toBe('"Alice A Example"')
    expect(applySearchSuggestion('from:ali is:unread', 7, {
      token: 'from', term: '"alice@example.com"', submit: true,
    })).toEqual({ query: 'from:"alice@example.com" is:unread ', submit: true })
    expect(applySearchSuggestion('hello ', 6, {
      token: 'in', term: '', submit: false,
    })).toEqual({ query: 'hello in: ', submit: false })
  })

  test('exposes the upstream token vocabulary and context-specific static values', () => {
    expect(staticSearchSuggestions('', 0).map(item => item.token)).toEqual([
      'from', 'to', 'in', 'before', 'since', 'is', 'has', 'subject',
    ])
    expect(staticSearchSuggestions('is:', 3).map(item => item.term)).toEqual([
      'unread', 'read', 'starred', 'unstarred',
    ])
    expect(staticSearchSuggestions('has:a', 5).map(item => item.term)).toEqual(['attachment'])
  })
})
