import { afterEach, describe, expect, it } from 'bun:test'
import EmailDetail from '../../client/components/email/detail/tac.js'
import { formatPreferenceDate } from '../../client/shared/scripts/mail-preferences.js'
import { applyInterfaceLocale } from '../../client/shared/scripts/localization.js'

const originalWindow = globalThis.window

afterEach(() => {
  globalThis.window = originalWindow
  applyInterfaceLocale('auto', ['en'])
})

describe('mail preference behavior', () => {
  it('orders conversation messages newest-first and uses detailed sender names', async () => {
    globalThis.window = {
      _caduceus: {
        mailPreferences: { conversationOrder: 'descending', detailedNames: true },
        apiFetch: async () => new Response(JSON.stringify([{
          emails: [
            { id: 'older', sender: 'old@example.test', headerFrom: 'Old Name <old@example.test>', receivedAt: '2026-01-01T00:00:00.000Z' },
            { id: 'newer', sender: 'new@example.test', headerFrom: 'New Name <new@example.test>', receivedAt: '2026-02-01T00:00:00.000Z' },
          ],
        }]), { status: 200 }),
      },
    }
    const detail = new EmailDetail()
    await detail.loadConversation('older')
    expect(detail.$conversationMessages.map(message => message.id)).toEqual(['newer', 'older'])
    expect(detail.$conversationMessages[0].visibleConversationSender).toBe('New Name <new@example.test>')
  })

  it('never marks a message read when manual read policy is selected', async () => {
    let requests = 0
    globalThis.window = {
      _caduceus: {
        mailPreferences: { markAsReadDelay: -1 },
        apiFetch: async () => { requests += 1; return new Response('{}', { status: 200 }) },
      },
    }
    const detail = new EmailDetail()
    detail.$email = { id: 'message-1', read: false }
    detail.scheduleMarkRead('message-1')
    await new Promise(resolve => setTimeout(resolve, 5))
    expect(requests).toBe(0)
    expect(detail.$email.read).toBe(false)
  })

  it('combines the selected interface locale with the account clock preference', () => {
    globalThis.window = { _caduceus: { mailPreferences: { use24HourClock: true } } }
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
