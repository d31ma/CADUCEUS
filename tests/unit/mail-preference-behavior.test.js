import { afterEach, describe, expect, it } from 'bun:test'
import EmailDetailService from '../../client/services/EmailDetailService.js'
import { MailPreferencesUtils } from '../../client/shared/utils/MailPreferencesUtils.js'
import { LocalizationUtils } from '../../client/shared/utils/LocalizationUtils.js'

const formatPreferenceDate = (...args) => MailPreferencesUtils.formatPreferenceDate(...args)
const applyInterfaceLocale = (...args) => LocalizationUtils.applyInterfaceLocale(...args)

const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document')

/** The reader is now EmailDetailService; the old `window._caduceus` globals come from its injected shell. */
function emailDetail(shell) {
  return Object.assign(new EmailDetailService(), { shell, taskScheduler: { render: async () => {} } })
}

/** Locale and clock format now live on `<html>` attributes instead of module/window state. */
function fakeDocument() {
  const attributes = new Map()
  return {
    documentElement: {
      dataset: {},
      style: { setProperty() {} },
      setAttribute: (name, value) => attributes.set(name, String(value)),
      getAttribute: name => attributes.get(name) ?? null,
    },
  }
}

afterEach(() => {
  applyInterfaceLocale('auto', ['en'])
  if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument)
  else delete globalThis.document
})

describe('mail preference behavior', () => {
  it('orders conversation messages newest-first and uses detailed sender names', async () => {
    const detail = emailDetail({
      mailPreferences: { conversationOrder: 'descending', detailedNames: true },
      apiFetch: async () => new Response(JSON.stringify([{
        emails: [
          { id: 'older', sender: 'old@example.test', headerFrom: 'Old Name <old@example.test>', receivedAt: '2026-01-01T00:00:00.000Z' },
          { id: 'newer', sender: 'new@example.test', headerFrom: 'New Name <new@example.test>', receivedAt: '2026-02-01T00:00:00.000Z' },
        ],
      }]), { status: 200 }),
    })
    await detail.loadConversation('older')
    expect(detail.conversationMessages.map(message => message.id)).toEqual(['newer', 'older'])
    expect(detail.conversationMessages[0].visibleConversationSender).toBe('New Name <new@example.test>')
  })

  it('never marks a message read when manual read policy is selected', async () => {
    let requests = 0
    const detail = emailDetail({
      mailPreferences: { markAsReadDelay: -1 },
      apiFetch: async () => { requests += 1; return new Response('{}', { status: 200 }) },
    })
    detail.email = { id: 'message-1', read: false }
    detail.scheduleMarkRead('message-1')
    await new Promise(resolve => setTimeout(resolve, 5))
    expect(requests).toBe(0)
    expect(detail.email.read).toBe(false)
  })

  it('combines the selected interface locale with the account clock preference', () => {
    globalThis.document = fakeDocument()
    MailPreferencesUtils.applyMailPreferences({ use24HourClock: true })
    applyInterfaceLocale('fr')
    const value = '2026-01-01T18:30:00.000Z'
    const options = {
      timeZone: 'UTC', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit',
    }
    expect(formatPreferenceDate(value, options)).toBe(
      new Intl.DateTimeFormat('fr', { ...options, hour12: false }).format(new Date(value)),
    )
  })
})
