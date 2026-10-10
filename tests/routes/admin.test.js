import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req, email } from './helpers.js'
import { UsersController as Users } from '@/controllers/UsersController.js'
import { UsersIdController as User } from '@/controllers/UsersIdController.js'
import { UsersAliasesController as Aliases } from '@/controllers/UsersAliasesController.js'
import { UsersMePreferencesController as Preferences } from '@/controllers/UsersMePreferencesController.js'
import { DomainsController as Domains } from '@/controllers/DomainsController.js'
import { DomainsDomainRoutesController as DomainRoutes } from '@/controllers/DomainsDomainRoutesController.js'
import { DomainsDomainRoutesIdController as DomainRoute } from '@/controllers/DomainsDomainRoutesIdController.js'
import { RulesController as Rules } from '@/controllers/RulesController.js'
import { RulesIdController as Rule } from '@/controllers/RulesIdController.js'
import { RulesPreviewController as RulePreview } from '@/controllers/RulesPreviewController.js'
import { RulesOrderController as RuleOrder } from '@/controllers/RulesOrderController.js'
import { TemplatesController as Templates } from '@/controllers/TemplatesController.js'
import { TemplatesIdController as Template } from '@/controllers/TemplatesIdController.js'
import { SignaturesController as Signatures } from '@/controllers/SignaturesController.js'
import { SignaturesIdController as Signature } from '@/controllers/SignaturesIdController.js'
import { SnoozeController as Snooze } from '@/controllers/SnoozeController.js'
import { PresenceTeamController as Team } from '@/controllers/PresenceTeamController.js'
import { PresenceHeartbeatController as Heartbeat } from '@/controllers/PresenceHeartbeatController.js'
import { NotificationsSubscriptionsController as Subscriptions } from '@/controllers/NotificationsSubscriptionsController.js'
import { TestResetController as Reset } from '@/controllers/TestResetController.js'
import { MailPreferences } from '@/shared/users/MailPreferences.js'
import { call } from '../support/controller-call.mjs'

let app
const auth = () => token()
const viewer = () => token({ email: 'viewer@example.com' })

beforeAll(async () => {
  app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'viewer@example.com', aliases: [], domains: ['example.com'], role: 'viewer' })
  await app.users.put({ email: 'bob@example.com', aliases: [], domains: ['example.com'], role: 'viewer' })
}, 300_000)
afterAll(teardownRoutes)

describe('POST /users', () => {
  const valid = { email: 'new@example.com', domains: ['example.com'], role: 'viewer' }

  it('403s for a non-admin', async () => {
    expect((await call(Users, 'POST', req({ token: viewer(), body: valid }))).forbidden).toBe(true)
  })

  it('400s on missing required fields', async () => {
    expect((await call(Users, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/email required/)
    expect((await call(Users, 'POST', req({ token: auth(), body: { email: 'a@b.com' } }))).error).toMatch(/domains required/)
    expect((await call(Users, 'POST', req({ token: auth(), body: { email: 'a@example.com', domains: ['example.com'] } }))).error).toMatch(/role required/)
  })

  it('400s on an invalid role', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, role: 'root' } }))
    expect(res.error).toMatch(/role invalid/)
  })

  it('400s on an unparseable email', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, email: 'not-an-email' } }))
    expect(res.error).toMatch(/email invalid/)
  })

  it('400s on an unparseable alias or domain', async () => {
    expect((await call(Users, 'POST', req({ token: auth(), body: { ...valid, aliases: ['bad'] } }))).error).toMatch(/aliases invalid/)
    expect((await call(Users, 'POST', req({ token: auth(), body: { ...valid, domains: ['not a domain'] } }))).error).toMatch(/domains invalid/)
  })

  it('rejects an alias equal to the primary email', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, aliases: ['new@example.com'] } }))
    expect(res.error).toMatch(/must not include primary/)
  })

  it('rejects an alias outside the user domains', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, aliases: ['x@elsewhere.com'] } }))
    expect(res.error).toMatch(/must belong to user domains/)
  })

  it('403s for a domain the admin does not hold', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, email: 'x@far.com', domains: ['far.com'] } }))
    expect(res.forbidden).toBe(true)
  })

  it('creates the user', async () => {
    expect(await call(Users, 'POST', req({ token: auth(), body: valid }))).toEqual({ email: 'new@example.com' })
  })

  it('rejects an address already held by another user', async () => {
    const res = await call(Users, 'POST', req({ token: auth(), body: { ...valid, email: 'other@example.com', aliases: ['new@example.com'] } }))
    expect(res.error).toMatch(/already belongs to another user/)
  })

  it('lists users for an admin', async () => {
    const res = await call(Users, 'GET', req({ token: auth() }))
    expect(res.some(u => u.email === 'new@example.com')).toBe(true)
  })
})

describe('DELETE /users/:id', () => {
  it('400s without an id', async () => {
    expect((await call(User, 'DELETE', req({ token: auth() }))).error).toMatch(/email required/)
  })

  it('404s for an unknown user', async () => {
    expect((await call(User, 'DELETE', req({ token: auth(), paths: { id: 'ghost@example.com' } }))).notFound).toBe(true)
  })

  it('deletes the user', async () => {
    const res = await call(User, 'DELETE', req({ token: auth(), paths: { id: 'new@example.com' } }))
    expect(res.deleted).toBe('new@example.com')
  })
})

describe('/users/aliases', () => {
  it('400s on an unparseable alias', async () => {
    expect((await call(Aliases, 'POST', req({ token: auth(), body: { alias: 'nope' } }))).error).toMatch(/alias invalid/)
  })

  it('403s for a domain outside the caller claims', async () => {
    const res = await call(Aliases, 'POST', req({ token: auth(), body: { alias: 'a@far.com' } }))
    expect(res.forbidden).toBe(true)
  })

  it('adds and lists an alias', async () => {
    const res = await call(Aliases, 'POST', req({ token: auth(), body: { alias: 'ALIAS@example.com' } }))
    expect(res.aliases).toContain('alias@example.com')
    expect((await call(Aliases, 'GET', req({ token: auth() }))).aliases).toContain('alias@example.com')
  })

  it('rejects an alias matching the primary email', async () => {
    const res = await call(Aliases, 'POST', req({ token: auth(), body: { alias: 'alice@example.com' } }))
    expect(res.error).toMatch(/must not match primary/)
  })

  it('404s when the caller has no user record', async () => {
    const res = await call(Aliases, 'GET', req({ token: token({ email: 'ghost@example.com' }) }))
    expect(res.notFound).toBe(true)
  })

  it('removes an alias', async () => {
    const res = await call(Aliases, 'DELETE', req({ token: auth(), body: { alias: 'alias@example.com' } }))
    expect(res.aliases).not.toContain('alias@example.com')
  })

  it('404s removing an alias that is not set', async () => {
    const res = await call(Aliases, 'DELETE', req({ token: auth(), body: { alias: 'alias@example.com' } }))
    expect(res.notFound).toBe(true)
  })

  it('400s removing an unparseable alias', async () => {
    expect((await call(Aliases, 'DELETE', req({ token: auth(), body: {} }))).error).toMatch(/alias invalid/)
  })
})

describe('Preferences._normalizeMailboxNames', () => {
  const domains = ['example.com']

  it('rejects a non-object', () => {
    expect(Preferences._normalizeMailboxNames(null, domains).error).toMatch(/must be an object/)
    expect(Preferences._normalizeMailboxNames([], domains).error).toMatch(/must be an object/)
  })

  it('rejects an unassigned domain', () => {
    const res = Preferences._normalizeMailboxNames({ 'far.com': 'Work' }, domains)
    expect(res.forbidden).toBe(true)
  })

  it('rejects a non-string name', () => {
    expect(Preferences._normalizeMailboxNames({ 'example.com': 5 }, domains).error).toMatch(/must be text/)
  })

  it('skips blank names', () => {
    expect(Preferences._normalizeMailboxNames({ 'example.com': '  ' }, domains)).toEqual({ ok: true, mailboxNames: {} })
  })

  it('rejects control characters and over-long names', () => {
    expect(Preferences._normalizeMailboxNames({ 'example.com': 'a b' }, domains).error).toMatch(/printable/)
    expect(Preferences._normalizeMailboxNames({ 'example.com': 'x'.repeat(65) }, domains).error).toMatch(/printable/)
  })

  it('rejects duplicate names case-insensitively', () => {
    const res = Preferences._normalizeMailboxNames({ 'example.com': 'Work', 'EXAMPLE.com ': 'work' }, domains)
    expect(res.error).toMatch(/unique/)
  })

  it('normalises and returns names', () => {
    expect(Preferences._normalizeMailboxNames({ 'EXAMPLE.com': ' Work ' }, domains))
      .toEqual({ ok: true, mailboxNames: { 'example.com': 'Work' } })
  })
})

describe('/users/me/preferences', () => {
  it('404s when the caller has no user record', async () => {
    expect((await call(Preferences, 'GET', req({ token: token({ email: 'ghost@example.com' }) }))).notFound).toBe(true)
    expect((await call(Preferences, 'PUT', req({ token: token({ email: 'ghost@example.com' }), body: {} }))).notFound).toBe(true)
  })

  it('defaults to a private calendar and no mailbox names', async () => {
    expect(await call(Preferences, 'GET', req({ token: auth() }))).toEqual({
      calendarVisibility: 'private',
      categoryMappings: {
        'example.com': {
          archive: 'archive',
          drafts: 'drafts',
          inbox: 'inbox',
          sent: 'sent',
          spam: 'spam',
          trash: 'trash',
        },
      },
      composerFont: 'interface',
      githubProfileEnrichment: false,
      grammarCheckEnabled: false,
      grammarCheckLanguage: 'auto',
      grammarCheckDisabledRules: [],
      interfaceFont: 'plex-sans',
      interfaceLocale: 'auto',
      keyboardShortcutOverrides: {},
      mailPreferences: MailPreferences.DEFAULT_MAIL_PREFERENCES,
      mailboxNames: {},
      newMailNotificationActions: true,
      newMailNotificationDomains: ['example.com'],
      newMailNotificationExternalAccounts: [],
      newMailNotificationPrivacy: 'full',
      notificationSounds: {
        confirm: 'bell',
        'hit-send': 'pop',
        'new-mail': 'chime',
        send: 'pulse',
      },
      remoteImagePolicy: 'block',
      themePreferences: {
        activeTheme: 'automatic',
        darkTheme: 'dark',
        lightTheme: 'light',
      },
      trustedImageSenders: [],
      unreadBadgeScope: 'all',
      unreadBadgeDomain: 'example.com',
    })
  })

  it('400s on an unknown calendar visibility', async () => {
    const res = await call(Preferences, 'PUT', req({ token: auth(), body: { calendarVisibility: 'sometimes' } }))
    expect(res.error).toMatch(/private, busy, full/)
  })

  it('403s on an unassigned mailbox domain', async () => {
    const res = await call(Preferences, 'PUT', req({ token: auth(), body: { mailboxNames: { 'far.com': 'Work' } } }))
    expect(res.forbidden).toBe(true)
  })

  it('400s on an invalid mailbox name', async () => {
    const res = await call(Preferences, 'PUT', req({ token: auth(), body: { mailboxNames: { 'example.com': 5 } } }))
    expect(res.error).toMatch(/must be text/)
  })

  it('saves both preferences', async () => {
    const res = await call(Preferences, 'PUT', req({ token: auth(), body: { calendarVisibility: 'busy', mailboxNames: { 'example.com': 'Work' } } }))
    expect(res).toEqual({
      updated: true,
      calendarVisibility: 'busy',
      categoryMappings: {
        'example.com': {
          archive: 'archive',
          drafts: 'drafts',
          inbox: 'inbox',
          sent: 'sent',
          spam: 'spam',
          trash: 'trash',
        },
      },
      composerFont: 'interface',
      githubProfileEnrichment: false,
      grammarCheckEnabled: false,
      grammarCheckLanguage: 'auto',
      grammarCheckDisabledRules: [],
      interfaceFont: 'plex-sans',
      interfaceLocale: 'auto',
      keyboardShortcutOverrides: {},
      mailPreferences: MailPreferences.DEFAULT_MAIL_PREFERENCES,
      mailboxNames: { 'example.com': 'Work' },
      newMailNotificationActions: true,
      newMailNotificationDomains: ['example.com'],
      newMailNotificationExternalAccounts: [],
      newMailNotificationPrivacy: 'full',
      notificationSounds: {
        confirm: 'bell',
        'hit-send': 'pop',
        'new-mail': 'chime',
        send: 'pulse',
      },
      remoteImagePolicy: 'block',
      themePreferences: {
        activeTheme: 'automatic',
        darkTheme: 'dark',
        lightTheme: 'light',
      },
      trustedImageSenders: [],
      unreadBadgeScope: 'all',
      unreadBadgeDomain: 'example.com',
    })
  })
})

describe('/domains', () => {
  it('403s for a non-admin', async () => {
    expect((await call(Domains, 'POST', req({ token: viewer(), body: { domain: 'example.com' } }))).forbidden).toBe(true)
  })

  it('400s without a domain and on an unparseable one', async () => {
    expect((await call(Domains, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/domain required/)
    expect((await call(Domains, 'POST', req({ token: auth(), body: { domain: 'not a domain' } }))).error).toMatch(/domain invalid/)
  })

  it('403s for a domain outside the caller claims', async () => {
    expect((await call(Domains, 'POST', req({ token: auth(), body: { domain: 'far.com' } }))).forbidden).toBe(true)
  })

  it('400s on invalid routes', async () => {
    const res = await call(Domains, 'POST', req({ token: auth(), body: { domain: 'example.com', routes: [{ match: '*' }] } }))
    expect(res.error).toMatch(/must have an id/)
  })

  it('creates the domain and lists it', async () => {
    const body = { domain: 'EXAMPLE.com', inboundEnabled: true, routes: [{ id: 'r1', match: '*@example.com', action: { type: 'store' }, enabled: true }] }
    expect(await call(Domains, 'POST', req({ token: auth(), body }))).toEqual({ domain: 'example.com' })
    const list = await call(Domains, 'GET', req({ token: auth() }))
    expect(list.some(d => d.domain === 'example.com')).toBe(true)
  })
})

describe('/domains/:domain/routes', () => {
  it('404s without a domain', async () => {
    expect((await call(DomainRoutes, 'GET', req({ token: auth() }))).notFound).toBe(true)
  })

  it('403s for a domain outside the caller claims', async () => {
    expect((await call(DomainRoutes, 'GET', req({ token: auth(), paths: { domain: 'far.com' } }))).forbidden).toBe(true)
  })

  it('lists the configured routes', async () => {
    const res = await call(DomainRoutes, 'GET', req({ token: auth(), paths: { domain: 'example.com' } }))
    expect(res.map(r => r.id)).toContain('r1')
  })

  it('400s without both path params', async () => {
    expect((await call(DomainRoute, 'PUT', req({ token: auth(), paths: { domain: 'example.com' }, body: {} }))).error).toMatch(/route id required/)
    expect((await call(DomainRoute, 'DELETE', req({ token: auth(), paths: {} }))).error).toMatch(/route id required/)
  })

  it('403s for a domain outside the caller claims', async () => {
    expect((await call(DomainRoute, 'PUT', req({ token: auth(), paths: { domain: 'far.com', id: 'r1' }, body: {} }))).forbidden).toBe(true)
    expect((await call(DomainRoute, 'DELETE', req({ token: auth(), paths: { domain: 'far.com', id: 'r1' } }))).forbidden).toBe(true)
  })

  it('400s on a missing body or invalid action', async () => {
    expect((await call(DomainRoute, 'PUT', req({ token: auth(), paths: { domain: 'example.com', id: 'r1' } }))).error).toMatch(/Missing body/)
    const res = await call(DomainRoute, 'PUT', req({ token: auth(), paths: { domain: 'example.com', id: 'r1' }, body: { action: { type: 'nope' } } }))
    expect(res.error).toMatch(/Unknown action type/)
  })

  it('replaces and deletes a route', async () => {
    const put = await call(DomainRoute, 'PUT', req({ token: auth(), paths: { domain: 'example.com', id: 'r1' }, body: { match: '*', action: { type: 'drop' }, enabled: false } }))
    expect(put.updated).toBe('r1')
    const del = await call(DomainRoute, 'DELETE', req({ token: auth(), paths: { domain: 'example.com', id: 'r1' } }))
    expect(del.deleted).toBe('r1')
    expect(await call(DomainRoutes, 'GET', req({ token: auth(), paths: { domain: 'example.com' } }))).toEqual([])
  })

  it('404s for an unconfigured domain', async () => {
    const t = token({ email: 'dave@unset.com' })
    expect((await call(DomainRoutes, 'GET', req({ token: t, paths: { domain: 'unset.com' } }))).notFound).toBe(true)
    expect((await call(DomainRoute, 'PUT', req({ token: t, paths: { domain: 'unset.com', id: 'x' }, body: { action: { type: 'store' } } }))).notFound).toBe(true)
    expect((await call(DomainRoute, 'DELETE', req({ token: t, paths: { domain: 'unset.com', id: 'x' } }))).notFound).toBe(true)
  })
})

describe('/rules', () => {
  let ruleId, secondRuleId
  const validRule = {
    name: 'Archive',
    mailbox: 'alice@example.com',
    conditions: [{ field: 'subject', op: 'contains', value: 'invoice' }],
    actions: [{ type: 'folder', folder: 'finance' }, { type: 'markRead', value: true }],
    stopProcessing: true,
  }

  it('400s without a name', async () => {
    expect((await call(Rules, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/name required/)
  })

  it('403s for a mailbox the caller does not own', async () => {
    const res = await call(Rules, 'POST', req({ token: auth(), body: { ...validRule, mailbox: 'bob@example.com' } }))
    expect(res.forbidden).toBe(true)
  })

  it('400s on invalid actions', async () => {
    const res = await call(Rules, 'POST', req({ token: auth(), body: { ...validRule, actions: [] } }))
    expect(res.error).toMatch(/At least one action/)
  })

  it('lets a mailbox user create a normalized exact-owner rule', async () => {
    const viewerRes = await call(Rules, 'POST', req({ token: viewer(), body: { ...validRule, mailbox: 'viewer@example.com', name: 'Viewer rule' } }))
    expect(viewerRes.id).toBeTruthy()
    expect((await call(Rules, 'GET', req({ token: viewer() }))).map(rule => rule.id)).toContain(viewerRes.id)
    expect((await call(Rules, 'GET', req({ token: auth() }))).map(rule => rule.id)).not.toContain(viewerRes.id)

    const res = await call(Rules, 'POST', req({ token: auth(), body: validRule }))
    ruleId = res.id
    expect(ruleId).toBeTruthy()
    const list = await call(Rules, 'GET', req({ token: auth() }))
    const stored = list.find(r => r.id === ruleId)
    expect(stored.ownerEmail).toBe('alice@example.com')
    expect(stored.mailbox).toBe('alice@example.com')
    expect(stored.domain).toBe('example.com')
    expect(stored.stopProcessing).toBe(true)
  })

  it('rejects a simple forwarding loop', async () => {
    const res = await call(Rules, 'POST', req({ token: auth(), body: { ...validRule, actions: [{ type: 'forward', to: 'ALICE@example.com' }] } }))
    expect(res.error).toMatch(/own mailbox/)
  })

  it('400s without an id', async () => {
    expect((await call(Rule, 'PUT', req({ token: auth(), body: {} }))).error).toMatch(/rule id/)
    expect((await call(Rule, 'DELETE', req({ token: auth() }))).error).toMatch(/rule id/)
  })

  it('404s for an unknown rule', async () => {
    expect((await call(Rule, 'PUT', req({ token: auth(), paths: { id: 'ghost' }, body: {} }))).notFound).toBe(true)
    expect((await call(Rule, 'DELETE', req({ token: auth(), paths: { id: 'ghost' } }))).notFound).toBe(true)
  })

  it('404s for another user on the same domain', async () => {
    const t = token({ email: 'bob@example.com' })
    expect((await call(Rule, 'PUT', req({ token: t, paths: { id: ruleId }, body: {} }))).notFound).toBe(true)
    expect((await call(Rule, 'DELETE', req({ token: t, paths: { id: ruleId } }))).notFound).toBe(true)
  })

  it('400s updating with invalid actions', async () => {
    const res = await call(Rule, 'PUT', req({ token: auth(), paths: { id: ruleId }, body: { actions: [{ type: 'nope' }] } }))
    expect(res.error).toMatch(/Unknown action type/)
  })

  it('previews with the production evaluator without mutating the rule', async () => {
    const preview = await call(RulePreview, 'POST', req({ token: auth(), body: {
      ruleId,
      message: { from: 'billing@vendor.example', to: 'alice@example.com', subject: 'August invoice', body: 'Attached', hasAttachment: true },
    } }))
    expect(preview.matched).toBe(true)
    expect(preview.plannedActions).toEqual(validRule.actions)
    expect(preview.stopProcessing).toBe(true)
    expect((await call(Rules, 'GET', req({ token: auth() }))).find(rule => rule.id === ruleId).enabled).toBe(true)
    expect((await call(RulePreview, 'POST', req({ token: token({ email: 'bob@example.com' }), body: { ruleId, message: {} } }))).notFound).toBe(true)
  })

  it('previews a validated unsaved draft and validates message bounds', async () => {
    const draft = await call(RulePreview, 'POST', req({ token: auth(), body: { rule: validRule, message: { subject: 'no match' } } }))
    expect(draft.matched).toBe(false)
    expect(draft.plannedActions).toEqual([])
    expect((await call(RulePreview, 'POST', req({ token: auth(), body: { rule: validRule, message: { body: 'x'.repeat(100_001) } } }))).error).toMatch(/body is too long/)
  })

  it('reorders the complete owner set and rejects duplicate IDs', async () => {
    secondRuleId = (await call(Rules, 'POST', req({ token: auth(), body: { ...validRule, name: 'Second' } }))).id
    const before = await call(Rules, 'GET', req({ token: auth() }))
    const ids = before.map(rule => rule.id).reverse()
    expect(await call(RuleOrder, 'PUT', req({ token: auth(), body: { ids } }))).toEqual({ ordered: ids.length })
    expect((await call(Rules, 'GET', req({ token: auth() }))).map(rule => rule.id)).toEqual(ids)
    expect((await call(RuleOrder, 'PUT', req({ token: auth(), body: { ids: [ruleId, ruleId] } }))).error).toMatch(/unique/)
  })

  it('updates and deletes the rules', async () => {
    expect((await call(Rule, 'PUT', req({ token: auth(), paths: { id: ruleId }, body: { enabled: false, name: 'Archived invoices' } }))).updated).toBe(ruleId)
    expect((await call(Rule, 'DELETE', req({ token: auth(), paths: { id: ruleId } }))).deleted).toBe(ruleId)
    expect((await call(Rule, 'DELETE', req({ token: auth(), paths: { id: secondRuleId } }))).deleted).toBe(secondRuleId)
  })
})

describe('/templates', () => {
  let id

  it('validates names and requires reusable content', async () => {
    expect((await call(Templates, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/name is required/)
    expect((await call(Templates, 'POST', req({ token: auth(), body: { name: 'n' } }))).error).toMatch(/content is required/)
    expect((await call(Templates, 'POST', req({ token: auth(), body: { name: 'bad/name', text: 'body' } }))).error).toMatch(/may contain/)
  })

  it('creates, sanitizes, de-duplicates, and lists templates', async () => {
    const res = await call(Templates, 'POST', req({ token: auth(), body: { name: ' Weekly ', subject: ' Hi ', text: '<p onclick="bad()">body</p><script>bad()</script>', to: 'a@b.com' } }))
    id = res.id
    expect(res.name).toBe('Weekly')
    expect(res.text).toBe('<p>body</p>')
    expect((await call(Templates, 'POST', req({ token: auth(), body: { name: 'weekly', text: 'another' } }))).name).toBe('weekly 1')
    const list = await call(Templates, 'GET', req({ token: auth() }))
    expect(list.find(t => t.id === id).name).toBe('Weekly')
    expect(list.map(t => t.name)).toEqual([...list.map(t => t.name)].sort((a, b) => a.localeCompare(b)))
  })

  it('updates only the exact owner template', async () => {
    expect((await call(Template, 'PUT', req({ token: auth(), paths: { id }, body: {} }))).error).toMatch(/At least one/)
    const updated = await call(Template, 'PUT', req({ token: auth(), paths: { id }, body: { name: 'Renewal', subject: '', text: '<p><code class="var">Name</code></p>' } }))
    expect(updated.name).toBe('Renewal')
    expect(updated.text).toContain('class="var"')
    expect((await call(Template, 'PUT', req({ token: token({ email: 'bob@example.com' }), paths: { id }, body: { name: 'Stolen' } }))).notFound).toBe(true)
  })

  it('400s deleting without an id', async () => {
    expect((await call(Templates, 'DELETE', req({ token: auth(), body: {} }))).error).toMatch(/id is required/)
  })

  it('404s deleting an unknown or foreign template', async () => {
    expect((await call(Templates, 'DELETE', req({ token: auth(), body: { id: 'ghost' } }))).notFound).toBe(true)
    const res = await call(Templates, 'DELETE', req({ token: token({ email: 'bob@example.com' }), body: { id } }))
    expect(res.notFound).toBe(true)
  })

  it('deletes the template', async () => {
    expect((await call(Template, 'DELETE', req({ token: auth(), paths: { id } }))).deleted).toBe(id)
  })
})

describe('/signatures', () => {
  let id, alternateId

  it('validates bounded rich content and exact owned defaults', async () => {
    expect((await call(Signatures, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/name/)
    const res = await call(Signatures, 'POST', req({
      token: auth(), body: { name: 'Foreign', text: 'x', defaultFor: ['bob@example.com'] },
    }))
    expect(res.error).toMatch(/not owned/)
  })

  it('creates, sanitizes, and lists only the exact owner signatures', async () => {
    const [, user] = await app.users.findByEmail('alice@example.com')
    const [docId] = await app.users.findByEmail('alice@example.com')
    await app.users.updateAliases(docId, ['sales@example.com'], user.email)
    const res = await call(Signatures, 'POST', req({
      token: auth(),
      body: {
        name: 'Work',
        text: '<script>bad()</script><p onclick="bad()">-- Alice</p>',
        defaultFor: ['alice@example.com'],
      },
    }))
    id = res.id
    expect(res.text).toBe('<p>-- Alice</p>')
    expect(res.defaultFor).toEqual(['alice@example.com'])
    expect((await call(Signatures, 'GET', req({ token: auth() }))).some(signature => signature.id === id)).toBe(true)
    expect(await call(Signatures, 'GET', req({ token: token({ email: 'bob@example.com' }) }))).toEqual([])
  })

  it('moves an address default atomically and de-duplicates names', async () => {
    const second = await call(Signatures, 'POST', req({
      token: auth(), body: { name: 'work', text: '<p>Sales</p>', defaultFor: ['alice@example.com', 'sales@example.com'] },
    }))
    alternateId = second.id
    expect(second.name).toBe('work 1')
    const list = await call(Signatures, 'GET', req({ token: auth() }))
    expect(list.find(signature => signature.id === id).defaultFor).toEqual([])
    expect(list.find(signature => signature.id === alternateId).defaultFor).toEqual(['alice@example.com', 'sales@example.com'])
  })

  it('updates only the exact owner record', async () => {
    expect((await call(Signature, 'PUT', req({
      token: token({ email: 'bob@example.com' }), paths: { id: alternateId }, body: { name: 'Stolen' },
    }))).notFound).toBe(true)
    const updated = await call(Signature, 'PUT', req({
      token: auth(), paths: { id: alternateId }, body: { name: 'Sales', defaultFor: ['sales@example.com'] },
    }))
    expect(updated).toMatchObject({ id: alternateId, name: 'Sales', defaultFor: ['sales@example.com'] })
  })

  it('hides unknown and foreign deletes, then deletes the owner record', async () => {
    expect((await call(Signature, 'DELETE', req({ token: auth() }))).notFound).toBe(true)
    expect((await call(Signature, 'DELETE', req({ token: auth(), paths: { id: 'ghost' } }))).notFound).toBe(true)
    expect((await call(Signature, 'DELETE', req({ token: token({ email: 'bob@example.com' }), paths: { id } }))).notFound).toBe(true)
    expect((await call(Signature, 'DELETE', req({ token: auth(), paths: { id } }))).deleted).toBe(id)
  })
})

describe('/snooze', () => {
  it('400s on missing or invalid input', async () => {
    expect((await call(Snooze, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/emailId is required/)
    expect((await call(Snooze, 'POST', req({ token: auth(), body: { emailId: 'e' } }))).error).toMatch(/until/)
    expect((await call(Snooze, 'POST', req({ token: auth(), body: { emailId: 'e', until: 'nope' } }))).error).toMatch(/valid ISO/)
    expect((await call(Snooze, 'POST', req({ token: auth(), body: { emailId: 'e', until: '2000-01-01T00:00:00.000Z' } }))).error).toMatch(/in the future/)
  })

  it('404s for an unknown or foreign email', async () => {
    const until = new Date(Date.now() + 60_000).toISOString()
    expect((await call(Snooze, 'POST', req({ token: auth(), body: { emailId: 'ghost', until } }))).notFound).toBe(true)
    await app.emails.put(email({ id: 'sn1' }))
    const res = await call(Snooze, 'POST', req({ token: token({ email: 'erin@far.com' }), body: { emailId: 'sn1', until } }))
    expect(res.notFound).toBe(true)
  })

  it('snoozes the email and lists it', async () => {
    const until = new Date(Date.now() + 60_000).toISOString()
    expect((await call(Snooze, 'POST', req({ token: auth(), body: { emailId: 'sn1', until } }))).snoozed).toBe(true)
    const list = await call(Snooze, 'GET', req({ token: auth() }))
    expect(list.some(e => e.emailId === 'sn1')).toBe(true)
  })

  it('hides snoozed mail from other domains', async () => {
    expect(await call(Snooze, 'GET', req({ token: token({ email: 'erin@far.com' }) }))).toEqual([])
  })
})

describe('/presence', () => {
  it('records a heartbeat and reports the caller online', async () => {
    await call(Heartbeat, 'POST', req({ token: auth() }))
    const res = await call(Team, 'GET', req({ token: auth() }))
    const me = res.find(u => u.email === 'alice@example.com')
    expect(me.online).toBe(true)
    expect(me.isSelf).toBe(true)
  })

  it('lists offline teammates after online ones', async () => {
    await app.users.put({ email: 'zed@example.com', aliases: [], domains: ['example.com'], role: 'viewer' })
    const res = await call(Team, 'GET', req({ token: auth() }))
    const onlineFlags = res.map(u => u.online)
    expect(onlineFlags).toEqual([...onlineFlags].sort((a, b) => Number(b) - Number(a)))
  })

  it('excludes users from other domains', async () => {
    await app.users.put({ email: 'far@far.com', aliases: [], domains: ['far.com'], role: 'viewer' })
    const res = await call(Team, 'GET', req({ token: auth() }))
    expect(res.every(u => u.email !== 'far@far.com')).toBe(true)
  })

  it('de-duplicates repeated user records', async () => {
    await app.users.put({ email: 'zed@example.com', aliases: [], domains: ['example.com'], role: 'viewer' })
    const res = await call(Team, 'GET', req({ token: auth() }))
    expect(res.filter(u => u.email === 'zed@example.com')).toHaveLength(1)
  })
})

describe('/notifications/subscriptions', () => {
  const sub = { endpoint: 'https://push.example/1', keys: { p256dh: 'key', auth: 'secret' } }

  it('400s on an incomplete subscription', async () => {
    expect((await call(Subscriptions, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/are required/)
    expect((await call(Subscriptions, 'POST', req({ token: auth(), body: { endpoint: 'x' } }))).error).toMatch(/are required/)
  })

  it('stores a subscription without echoing the keys', async () => {
    const res = await call(Subscriptions, 'POST', req({ token: auth(), body: sub, headers: { 'user-agent': 'test' } }))
    expect(res.keys).toBeUndefined()
    expect(res.endpoint).toBe(sub.endpoint)
  })

  it('lists and removes a subscription', async () => {
    expect((await call(Subscriptions, 'GET', req({ token: auth() }))).length).toBeGreaterThan(0)
    await call(Subscriptions, 'DELETE', req({ token: auth(), body: { endpoint: sub.endpoint } }))
    expect(await call(Subscriptions, 'GET', req({ token: auth() }))).toEqual([])
  })
})

describe('DELETE /test/reset', () => {
  it('403s when test routes are disabled', async () => {
    delete process.env.CADUCEUS_ENABLE_TEST_ROUTES
    expect((await call(Reset, 'DELETE', req({ token: auth() }))).forbidden).toBe(true)
  })

  it('wipes every collection when enabled', async () => {
    process.env.CADUCEUS_ENABLE_TEST_ROUTES = 'true'
    try {
      expect((await call(Reset, 'DELETE', req({ token: auth() }))).reset).toBe(true)
      expect(await app.users.list()).toEqual([])
    } finally {
      delete process.env.CADUCEUS_ENABLE_TEST_ROUTES
    }
  })
})
