import { afterEach, describe, expect, it } from 'bun:test'
import {
  DEFAULT_MAIL_PREFERENCES as CLIENT_DEFAULTS,
  applyMailPreferences,
  normalizeMailPreferences as normalizeClient,
} from '../../client/shared/scripts/mail-preferences.js'
import {
  DEFAULT_MAIL_PREFERENCES as SERVER_DEFAULTS,
  normalizeMailPreferences as normalizeServer,
  validateMailPreferencePatch,
} from '../../server/shared/users/mail-preferences.js'

const originalDocument = globalThis.document

afterEach(() => { globalThis.document = originalDocument })

describe('mail preferences', () => {
  const user = {
    email: 'alice@example.com', aliases: ['sales@example.com'], domains: ['example.com'], role: 'admin',
  }

  it('keeps browser and server defaults in lockstep', () => {
    expect(CLIENT_DEFAULTS).toEqual(SERVER_DEFAULTS)
    expect(normalizeClient({})).toEqual(normalizeServer({}, user))
  })

  it('normalizes hostile browser values and rejects unknown server fields atomically', () => {
    const hostile = {
      workspaceLayout: 'url(evil)', interfaceScale: 99, backspaceAction: 'erase-disk',
      autoCcBySender: { ' Sales@Example.com ': [' Person@Other.test ', 'Person@Other.test'] },
      findInMailDisabledDomains: [' EXAMPLE.COM ', '', 'example.com'],
    }
    expect(normalizeClient(hostile)).toMatchObject({
      workspaceLayout: 'split', interfaceScale: 1, backspaceAction: 'archive',
      autoCcBySender: { 'sales@example.com': ['person@other.test'] },
      findInMailDisabledDomains: ['example.com'],
    })
    expect(validateMailPreferencePatch({ workspaceLayout: 'list', injectedCss: 'body{}' }, user)).toEqual({
      ok: false, error: 'Unknown mail preference: injectedCss',
    })
  })

  it('applies only fixed root attributes and a cataloged scale', () => {
    const style = new Map()
    globalThis.document = {
      documentElement: { dataset: {}, style: { setProperty: (name, value) => style.set(name, value) } },
    }
    const applied = applyMailPreferences({
      workspaceLayout: 'split-vertical', interfaceScale: 1.2, restrictMessageWidth: true,
      emailRenderMode: 'dark', conversationOrder: 'descending', displayAttachmentPreviews: false,
      richText: false, swipeEnabled: false,
    })
    expect(globalThis.document.documentElement.dataset).toEqual({
      mailLayout: 'split-vertical', messageWidth: 'restricted', messageTheme: 'dark',
      conversationOrder: 'descending', attachmentPreviews: 'disabled', richComposer: 'disabled', mailSwipe: 'disabled',
    })
    expect(style.get('--caduceus-interface-scale')).toBe('1.2')
    expect(applied).toMatchObject({ richText: false, displayAttachmentPreviews: false, swipeEnabled: false })
  })
})
