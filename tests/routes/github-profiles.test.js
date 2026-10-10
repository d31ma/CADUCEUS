import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { Handler as GitHubProfile } from '@/routes/github/profile/yon.js'
import { Handler as Preferences } from '@/routes/users/me/preferences/yon.js'

let app
const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'bob@example.com', domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('mailbox-scoped GitHub contact profile route', () => {
  it('is disabled by default and makes no public lookup', async () => {
    let calls = 0
    app.githubProfiles = { findByEmail: async () => { calls++; return { status: 'not-found' } } }
    const result = await GitHubProfile.GET(req({ token: alice(), query: { email: 'grace@example.net' } }))
    expect(result).toEqual({ enabled: false, status: 'disabled' })
    expect(calls).toBe(0)
  })

  it('returns sanitized service data only for an exact owned contact after opt-in', async () => {
    await Preferences.PUT(req({ token: alice(), body: { githubProfileEnrichment: true } }))
    await app.contacts.create('alice@example.com', { name: 'Grace', emails: [{ value: 'grace@example.net' }] })
    app.githubProfiles = { findByEmail: async value => ({
      status: 'found',
      profile: { login: 'grace', profileUrl: 'https://github.com/grace', repositories: [{
        name: 'compiler', fullName: 'grace/compiler', url: 'https://github.com/grace/compiler',
        description: 'A compiler', language: 'C', stars: value === 'grace@example.net' ? 42 : 0,
      }] },
    }) }

    const result = await GitHubProfile.GET(req({ token: alice(), query: { email: 'GRACE@EXAMPLE.NET' } }))
    expect(result).toMatchObject({ enabled: true, status: 'found', profile: { login: 'grace' } })
    expect(result.profile.repositories[0].stars).toBe(42)

    const unrelated = await GitHubProfile.GET(req({ token: alice(), query: { email: 'stranger@example.net' } }))
    expect(unrelated.notFound).toBe(true)
  })

  it('recognizes exact owned message participants but not another same-domain mailbox copy', async () => {
    await Preferences.PUT(req({ token: bob(), body: { githubProfileEnrichment: true } }))
    await app.emails.put(email({
      id: 'github-owner-boundary',
      ownerEmail: 'alice@example.com',
      recipient: 'alice@example.com',
      sender: 'participant@example.net',
      body: 'stranger@example.net is merely text',
    }))
    app.githubProfiles = { findByEmail: async () => ({ status: 'not-found' }) }

    const aliceResult = await GitHubProfile.GET(req({ token: alice(), query: { email: 'participant@example.net' } }))
    expect(aliceResult).toEqual({ enabled: true, status: 'not-found' })
    expect((await GitHubProfile.GET(req({ token: alice(), query: { email: 'stranger@example.net' } }))).notFound).toBe(true)
    expect((await GitHubProfile.GET(req({ token: bob(), query: { email: 'participant@example.net' } }))).notFound).toBe(true)
  })

  it('rejects malformed email queries before lookup', async () => {
    expect((await GitHubProfile.GET(req({ token: alice(), query: { email: 'not-an-email' } }))).error).toContain('valid email')
  })
})
