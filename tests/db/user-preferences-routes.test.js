import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'bun:test'
import { setupRoutes, teardownRoutes, token as issue } from '../routes/helpers.js'
import { UsersMePreferencesController as PreferencesHandler } from '@/controllers/UsersMePreferencesController.js'
import { DEFAULT_MAIL_PREFERENCES } from '@/features/users/services/mail-preferences.js'
import { call } from '../support/controller-call.mjs'

const getPreferences = (req) => call(PreferencesHandler, 'GET', req)
const putPreferences = (req) => call(PreferencesHandler, 'PUT', req)
const defaultSounds = { 'new-mail': 'chime', 'hit-send': 'pop', send: 'pulse', confirm: 'bell' }
const defaultThemes = { activeTheme: 'automatic', lightTheme: 'light', darkTheme: 'dark' }
const nativeRoles = { inbox: 'inbox', sent: 'sent', drafts: 'drafts', spam: 'spam', archive: 'archive', trash: 'trash' }
const defaultCategoryMappings = {
  'example.com': nativeRoles,
  'work.example.com': nativeRoles,
}

let app
let token

beforeAll(async () => {
  app = await setupRoutes()
  token = issue()
}, 300_000)
afterAll(teardownRoutes)

beforeEach(async () => {
  // Each test starts from one mailbox record carrying no preferences.
  await app.users.put({
    email: 'alice@example.com',
    aliases: [],
    // The mail-side record lists every domain this mailbox may be named in.
    // That is CADUCEUS's own data, separate from the authority the boundary
    // decides, and the handler validates against it.
    domains: ['example.com', 'work.example.com'],
    role: 'admin',
  })
}, 300_000)

const context = () => ({ bearer: { token } })

describe('mailbox name preferences', () => {
  it('defaults to no mailbox nicknames', async () => {
    const result = await getPreferences({ context: context() })
    expect(result).toEqual({
      calendarVisibility: 'private',
      mailboxNames: {},
      categoryMappings: defaultCategoryMappings,
      remoteImagePolicy: 'block',
      trustedImageSenders: [],
      interfaceFont: 'plex-sans',
      composerFont: 'interface',
      interfaceLocale: 'auto',
      notificationSounds: defaultSounds,
      themePreferences: defaultThemes,
      unreadBadgeScope: 'all',
      unreadBadgeDomain: 'example.com',
      newMailNotificationDomains: ['example.com', 'work.example.com'],
      newMailNotificationExternalAccounts: [],
      newMailNotificationPrivacy: 'full',
      newMailNotificationActions: true,
      githubProfileEnrichment: false,
      keyboardShortcutOverrides: {},
      mailPreferences: DEFAULT_MAIL_PREFERENCES,
      grammarCheckEnabled: false,
      grammarCheckLanguage: 'auto',
      grammarCheckDisabledRules: [],
    })
  })

  it('persists trimmed names keyed by normalized assigned domain', async () => {
    const updated = await putPreferences({
      body: { calendarVisibility: 'busy', mailboxNames: { 'EXAMPLE.COM': '  cadu  ', 'work.example.com': 'Work' } },
      context: context(),
    })
    expect(updated).toEqual({
      updated: true,
      calendarVisibility: 'busy',
      mailboxNames: { 'example.com': 'cadu', 'work.example.com': 'Work' },
      categoryMappings: defaultCategoryMappings,
      remoteImagePolicy: 'block',
      trustedImageSenders: [],
      interfaceFont: 'plex-sans',
      composerFont: 'interface',
      interfaceLocale: 'auto',
      notificationSounds: defaultSounds,
      themePreferences: defaultThemes,
      unreadBadgeScope: 'all',
      unreadBadgeDomain: 'example.com',
      newMailNotificationDomains: ['example.com', 'work.example.com'],
      newMailNotificationExternalAccounts: [],
      newMailNotificationPrivacy: 'full',
      newMailNotificationActions: true,
      githubProfileEnrichment: false,
      keyboardShortcutOverrides: {},
      mailPreferences: DEFAULT_MAIL_PREFERENCES,
      grammarCheckEnabled: false,
      grammarCheckLanguage: 'auto',
      grammarCheckDisabledRules: [],
    })

    const result = await getPreferences({ context: context() })
    expect(result.calendarVisibility).toBe('busy')
    expect(result.mailboxNames).toEqual({ 'example.com': 'cadu', 'work.example.com': 'Work' })
  })

  it('persists a remote-image policy and normalized exact-sender trust list', async () => {
    const updated = await putPreferences({
      body: {
        remoteImagePolicy: 'allow',
        trustedImageSenders: [' News@Other.test ', 'news@other.test', 'alerts@other.test'],
      },
      context: context(),
    })

    expect(updated.remoteImagePolicy).toBe('allow')
    expect(updated.trustedImageSenders).toEqual(['news@other.test', 'alerts@other.test'])
    const persisted = await getPreferences({ context: context() })
    expect(persisted.remoteImagePolicy).toBe('allow')
    expect(persisted.trustedImageSenders).toEqual(['news@other.test', 'alerts@other.test'])
  })

  it('persists one focused hosted account for unread badges and rejects foreign scope', async () => {
    const updated = await putPreferences({
      body: { unreadBadgeScope: 'domain', unreadBadgeDomain: 'WORK.EXAMPLE.COM' },
      context: context(),
    })
    expect(updated).toMatchObject({
      unreadBadgeScope: 'domain',
      unreadBadgeDomain: 'work.example.com',
    })
    expect(await getPreferences({ context: context() })).toMatchObject({
      unreadBadgeScope: 'domain',
      unreadBadgeDomain: 'work.example.com',
    })

    expect((await putPreferences({
      body: { unreadBadgeScope: 'some' },
      context: context(),
    })).error).toContain('unreadBadgeScope')
    expect((await putPreferences({
      body: { unreadBadgeScope: 'domain', unreadBadgeDomain: 'foreign.test' },
      context: context(),
    })).forbidden).toBe(true)
    expect((await putPreferences({
      body: { unreadBadgeDomain: 42 },
      context: context(),
    })).error).toContain('unreadBadgeDomain')
  })

  it('persists an exact per-account new-mail policy and rejects foreign or malformed policy', async () => {
    const updated = await putPreferences({
      body: {
        newMailNotificationDomains: ['WORK.EXAMPLE.COM'],
        newMailNotificationPrivacy: 'private',
        newMailNotificationActions: false,
      },
      context: context(),
    })
    expect(updated).toMatchObject({
      newMailNotificationDomains: ['work.example.com'],
      newMailNotificationPrivacy: 'private',
      newMailNotificationActions: false,
    })
    expect(await getPreferences({ context: context() })).toMatchObject({
      newMailNotificationDomains: ['work.example.com'],
      newMailNotificationPrivacy: 'private',
      newMailNotificationActions: false,
    })

    expect((await putPreferences({
      body: { newMailNotificationDomains: ['foreign.test'] },
      context: context(),
    })).forbidden).toBe(true)
    expect((await putPreferences({
      body: { newMailNotificationPrivacy: 'subject-only' },
      context: context(),
    })).error).toContain('newMailNotificationPrivacy')
    expect((await putPreferences({
      body: { newMailNotificationActions: 'true' },
      context: context(),
    })).error).toContain('boolean')
  })

  it('persists the explicit GitHub profile enrichment opt-in and rejects coercion', async () => {
    const enabled = await putPreferences({
      body: { githubProfileEnrichment: true },
      context: context(),
    })
    expect(enabled.githubProfileEnrichment).toBe(true)
    expect((await getPreferences({ context: context() })).githubProfileEnrichment).toBe(true)

    const invalid = await putPreferences({
      body: { githubProfileEnrichment: 'true' },
      context: context(),
    })
    expect(invalid.error).toContain('boolean')
  })

  it('persists standalone grammar toggle, language, and ignored rules', async () => {
    const updated = await putPreferences({
      body: {
        grammarCheckEnabled: true,
        grammarCheckLanguage: 'en-GB',
        grammarCheckDisabledRules: ['LOWERCASE_I', 'LOWERCASE_I'],
      },
      context: context(),
    })
    expect(updated).toMatchObject({
      grammarCheckEnabled: true,
      grammarCheckLanguage: 'en-GB',
      grammarCheckDisabledRules: ['LOWERCASE_I'],
    })
    expect(await getPreferences({ context: context() })).toMatchObject({
      grammarCheckEnabled: true,
      grammarCheckLanguage: 'en-GB',
      grammarCheckDisabledRules: ['LOWERCASE_I'],
    })
    for (const body of [
      { grammarCheckEnabled: 'true' },
      { grammarCheckLanguage: 'fr' },
      { grammarCheckDisabledRules: ['../unsafe'] },
    ]) expect((await putPreferences({ body, context: context() })).error).toBeTruthy()
  })

  it('persists independent interface and composer font catalog choices', async () => {
    const updated = await putPreferences({
      body: { interfaceFont: 'serif', composerFont: 'monospace' },
      context: context(),
    })

    expect(updated.interfaceFont).toBe('serif')
    expect(updated.composerFont).toBe('monospace')

    const composerOnly = await putPreferences({
      body: { composerFont: 'interface' },
      context: context(),
    })
    expect(composerOnly.interfaceFont).toBe('serif')
    expect(composerOnly.composerFont).toBe('interface')

    const persisted = await getPreferences({ context: context() })
    expect(persisted.interfaceFont).toBe('serif')
    expect(persisted.composerFont).toBe('interface')
  })

  it('persists a catalog-backed interface locale and rejects arbitrary tags', async () => {
    const updated = await putPreferences({
      body: { interfaceLocale: 'he' },
      context: context(),
    })
    expect(updated.interfaceLocale).toBe('he')
    expect((await getPreferences({ context: context() })).interfaceLocale).toBe('he')

    for (const interfaceLocale of ['fr-CA', '../fr', 'en<script>', 42]) {
      const rejected = await putPreferences({ body: { interfaceLocale }, context: context() })
      expect(rejected.error).toContain('interfaceLocale')
    }
    expect((await getPreferences({ context: context() })).interfaceLocale).toBe('he')
  })

  it('persists normalized mail workspace, reading, composing, and sending preferences', async () => {
    const updated = await putPreferences({
      body: {
        mailPreferences: {
          workspaceLayout: 'split-vertical',
          use24HourClock: true,
          interfaceScale: 1.1,
          showUnreadForAllFolders: true,
          markAsReadDelay: 2000,
          detailedHeaders: true,
          detailedNames: true,
          restrictMessageWidth: true,
          backspaceAction: 'trash',
          swipeEnabled: false,
          conversationOrder: 'descending',
          emailRenderMode: 'light',
          richText: false,
          spellcheck: false,
          spellcheckLanguage: 'en-CA',
          defaultSendAction: 'send-and-archive',
          defaultReplyAction: 'reply',
          undoSendDelay: 30000,
          defaultFrom: ' Alice@Example.com ',
          autoCcBySender: { 'ALICE@EXAMPLE.COM': [' Manager@Other.test '] },
          autoBccBySender: { 'alice@example.com': ['archive@other.test'] },
          displayAttachmentPreviews: false,
          unsnoozeToTop: false,
          findInMailDisabledDomains: ['WORK.EXAMPLE.COM'],
        },
      },
      context: context(),
    })

    expect(updated.mailPreferences).toEqual({
      workspaceLayout: 'split-vertical',
      use24HourClock: true,
      interfaceScale: 1.1,
      showUnreadForAllFolders: true,
      markAsReadDelay: 2000,
      detailedHeaders: true,
      detailedNames: true,
      restrictMessageWidth: true,
      backspaceAction: 'trash',
      swipeEnabled: false,
      conversationOrder: 'descending',
      emailRenderMode: 'light',
      richText: false,
      spellcheck: false,
      spellcheckLanguage: 'en-CA',
      defaultSendAction: 'send-and-archive',
      defaultReplyAction: 'reply',
      undoSendDelay: 30000,
      defaultFrom: 'alice@example.com',
      autoCcBySender: { 'alice@example.com': ['manager@other.test'] },
      autoBccBySender: { 'alice@example.com': ['archive@other.test'] },
      displayAttachmentPreviews: false,
      unsnoozeToTop: false,
      findInMailDisabledDomains: ['work.example.com'],
    })
    expect((await getPreferences({ context: context() })).mailPreferences).toEqual(updated.mailPreferences)

    const partial = await putPreferences({
      body: { mailPreferences: { use24HourClock: false } },
      context: context(),
    })
    expect(partial.mailPreferences.use24HourClock).toBe(false)
    expect(partial.mailPreferences.workspaceLayout).toBe('split-vertical')
    expect(partial.mailPreferences.defaultFrom).toBe('alice@example.com')
  })

  it('rejects unknown, malformed, foreign, and oversized mail preferences atomically', async () => {
    const before = (await getPreferences({ context: context() })).mailPreferences
    for (const mailPreferences of [
      { script: 'alert(1)' },
      { workspaceLayout: 'panorama' },
      { use24HourClock: 'true' },
      { defaultFrom: 'other@example.com' },
      { autoCcBySender: { 'other@example.com': ['alice@example.com'] } },
      { autoBccBySender: { 'alice@example.com': ['not-an-address'] } },
      { findInMailDisabledDomains: ['foreign.test'] },
      { autoCcBySender: { 'alice@example.com': Array.from({ length: 51 }, (_, index) => `person${index}@other.test`) } },
    ]) {
      const result = await putPreferences({ body: { mailPreferences }, context: context() })
      expect(result.error).toContain('mail preference')
    }
    expect((await getPreferences({ context: context() })).mailPreferences).toEqual(before)
  })

  it('rejects arbitrary font names, CSS, and remote font values', async () => {
    for (const [field, value] of [
      ['interfaceFont', 'Comic Sans MS'],
      ['interfaceFont', 'url(https://fonts.example.test/leak.woff2)'],
      ['composerFont', 'serif; color: red'],
    ]) {
      const result = await putPreferences({ body: { [field]: value }, context: context() })
      expect(result.error).toContain(field)
    }
  })

  it('persists a complete per-event notification sound catalog', async () => {
    const sounds = { 'new-mail': 'bell', 'hit-send': 'none', send: 'chime', confirm: 'pulse' }
    const updated = await putPreferences({ body: { notificationSounds: sounds }, context: context() })
    expect(updated.notificationSounds).toEqual(sounds)
    expect((await getPreferences({ context: context() })).notificationSounds).toEqual(sounds)
  })

  it('rejects arbitrary sound identifiers and unknown events', async () => {
    const remote = await putPreferences({
      body: { notificationSounds: { ...defaultSounds, 'new-mail': 'https://sounds.example.test/mail.ogg' } },
      context: context(),
    })
    expect(remote.error).toContain('new-mail sound')

    const unknown = await putPreferences({
      body: { notificationSounds: { ...defaultSounds, logout: 'bell' } },
      context: context(),
    })
    expect(unknown.error).toContain('Unknown notification sound event')
  })

  it('persists automatic and direct catalog-backed theme choices', async () => {
    const themes = { activeTheme: 'automatic', lightTheme: 'taiga', darkTheme: 'darkside' }
    const updated = await putPreferences({ body: { themePreferences: themes }, context: context() })
    expect(updated.themePreferences).toEqual(themes)

    const direct = await putPreferences({
      body: { themePreferences: { activeTheme: 'ubuntu', lightTheme: 'less-is-more', darkTheme: 'dark' } },
      context: context(),
    })
    expect(direct.themePreferences.activeTheme).toBe('ubuntu')
    expect((await getPreferences({ context: context() })).themePreferences).toEqual(direct.themePreferences)
  })

  it('persists normalized keyboard customization and explicit disabled commands', async () => {
    const updated = await putPreferences({
      body: {
        keyboardShortcutOverrides: {
          'core:compose': ' MOD+K ',
          'composer:discard': '',
          'email:reply-all': 'shift+r',
        },
      },
      context: context(),
    })
    expect(updated.keyboardShortcutOverrides).toEqual({
      'core:compose': 'mod+k',
      'composer:discard': 'off',
    })
    expect((await getPreferences({ context: context() })).keyboardShortcutOverrides).toEqual(updated.keyboardShortcutOverrides)
  })

  it('rejects unknown, malformed, and context-conflicting keyboard commands atomically', async () => {
    const before = (await getPreferences({ context: context() })).keyboardShortcutOverrides
    for (const keyboardShortcutOverrides of [
      { 'unknown:command': 'x' },
      { 'core:compose': 'g i x' },
      { 'core:compose': 'mod+1' },
      { 'core:compose': 'ctrl+alt' },
    ]) {
      const result = await putPreferences({ body: { keyboardShortcutOverrides }, context: context() })
      expect(result.error).toBeTruthy()
    }
    expect((await getPreferences({ context: context() })).keyboardShortcutOverrides).toEqual(before)
  })

  it('allows one binding in mutually exclusive mailbox and reader contexts', async () => {
    const result = await putPreferences({
      body: { keyboardShortcutOverrides: { 'core:refresh': 'x', 'email:reply-all': 'x' } },
      context: context(),
    })
    expect(result.keyboardShortcutOverrides).toEqual({ 'core:refresh': 'x', 'email:reply-all': 'x' })
  })

  it('rejects arbitrary theme packages, CSS, URLs, and unknown fields', async () => {
    for (const themePreferences of [
      { ...defaultThemes, activeTheme: 'https://themes.example.test/dark.css' },
      { ...defaultThemes, lightTheme: 'light;--w-text:red' },
      { ...defaultThemes, darkTheme: '../../../custom-theme' },
      { ...defaultThemes, script: 'alert(1)' },
    ]) {
      const result = await putPreferences({ body: { themePreferences }, context: context() })
      expect(String(result.error).toLowerCase()).toContain('theme')
    }
  })

  it('rejects unsafe remote-image preference values', async () => {
    const policy = await putPreferences({
      body: { remoteImagePolicy: 'sender-decides' },
      context: context(),
    })
    expect(policy.error).toContain('block, allow')

    const malformed = await putPreferences({
      body: { trustedImageSenders: ['not-an-address'] },
      context: context(),
    })
    expect(malformed.error).toContain('email address')

    const oversized = await putPreferences({
      body: { trustedImageSenders: Array.from({ length: 501 }, (_, index) => `sender-${index}@other.test`) },
      context: context(),
    })
    expect(oversized.error).toContain('at most 500')
  })

  it('uses an empty name to restore the domain label', async () => {
    await putPreferences({ body: { mailboxNames: { 'example.com': 'cadu' } }, context: context() })
    const result = await putPreferences({ body: { mailboxNames: { 'example.com': '   ' } }, context: context() })
    expect(result.mailboxNames).toEqual({})
  })

  it('rejects names for a mailbox outside the signed-in user domains', async () => {
    const result = await putPreferences({
      body: { mailboxNames: { 'other.example.com': 'other' } },
      context: context(),
    })
    expect(result.forbidden).toBe(true)
  })

  it('rejects duplicate visible mailbox names', async () => {
    const result = await putPreferences({
      body: { mailboxNames: { 'example.com': 'Work', 'work.example.com': ' work ' } },
      context: context(),
    })
    expect(result.error).toContain('unique')
  })

  it('rejects unsafe and overlong mailbox names', async () => {
    const unsafe = await putPreferences({
      body: { mailboxNames: { 'example.com': 'bad\nname' } },
      context: context(),
    })
    expect(unsafe.error).toContain('printable')

    const overlong = await putPreferences({
      body: { mailboxNames: { 'example.com': 'x'.repeat(65) } },
      context: context(),
    })
    expect(overlong.error).toContain('64')
  })
})
