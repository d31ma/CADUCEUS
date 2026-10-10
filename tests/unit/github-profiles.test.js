import { describe, expect, it } from 'bun:test'
import { GitHubProfilesService } from '@/features/contacts/services/github-profiles.js'

function json(value, status = 200, headers = {}) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'content-type': 'application/json', ...headers },
  })
}

describe('GitHub public profile enrichment', () => {
  it('uses only fixed GitHub endpoints, sanitizes output, sorts stars, and caches by normalized email', async () => {
    const requests = []
    const fetchImpl = async (url, options) => {
      requests.push({ url: String(url), options })
      if (String(url).includes('/search/users')) return json({ items: [{ login: 'grace-hopper' }] })
      return json({ items: [
        { name: 'compiler', owner: { login: 'grace-hopper' }, description: ' First\ncompiler ', language: 'C++', stargazers_count: 100 },
        { name: 'navy', owner: { login: 'grace-hopper' }, description: '', language: null, stargazers_count: 500 },
        { name: 'foreign', owner: { login: 'someone-else' }, stargazers_count: 999999 },
        { name: '../unsafe', owner: { login: 'grace-hopper' }, stargazers_count: 900 },
      ] })
    }
    const service = new GitHubProfilesService({ fetchImpl })

    const first = await service.findByEmail(' Grace@Example.net ')
    const second = await service.findByEmail('grace@example.net')

    expect(first).toEqual({
      status: 'found',
      profile: {
        login: 'grace-hopper',
        profileUrl: 'https://github.com/grace-hopper',
        repositories: [
          { name: 'navy', fullName: 'grace-hopper/navy', url: 'https://github.com/grace-hopper/navy', description: '', language: '', stars: 500 },
          { name: 'compiler', fullName: 'grace-hopper/compiler', url: 'https://github.com/grace-hopper/compiler', description: 'First compiler', language: 'C++', stars: 100 },
        ],
      },
    })
    expect(second).toBe(first)
    expect(requests).toHaveLength(2)
    expect(requests[0].url).toBe('https://api.github.com/search/users?q=grace%40example.net&per_page=1')
    expect(requests[1].url).toContain('https://api.github.com/search/repositories?')
    expect(requests[0].options.redirect).toBe('error')
    expect(requests[0].options.headers['User-Agent']).toBe('CADUCEUS-Mail-Server')
  })

  it('negative-caches no match and fails optional lookup closed on malformed or oversized responses', async () => {
    let calls = 0
    const noMatch = new GitHubProfilesService({ fetchImpl: async () => { calls++; return json({ items: [] }) } })
    expect(await noMatch.findByEmail('none@example.net')).toEqual({ status: 'not-found' })
    expect(await noMatch.findByEmail('none@example.net')).toEqual({ status: 'not-found' })
    expect(calls).toBe(1)

    const malformed = new GitHubProfilesService({ fetchImpl: async () => new Response('not-json') })
    expect(await malformed.findByEmail('grace@example.net')).toEqual({ status: 'unavailable' })

    const oversized = new GitHubProfilesService({
      fetchImpl: async () => json({ items: [] }, 200, { 'content-length': '1000001' }),
    })
    expect(await oversized.findByEmail('grace@example.net')).toEqual({ status: 'unavailable' })
    expect(await oversized.findByEmail('not an email')).toEqual({ status: 'not-found' })
  })
})
