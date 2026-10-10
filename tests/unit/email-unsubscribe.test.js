import { afterEach, beforeEach, describe, expect, it, mock } from 'bun:test'

globalThis.onMount = value => value
const { default: EmailDetail } = await import('../../client/components/email/detail/tac.js')

const originalWindow = globalThis.window
const originalConfirm = globalThis.confirm
const originalFetch = globalThis.fetch

let composed
let toast

beforeEach(() => {
  composed = null
  toast = ''
  globalThis.confirm = () => true
  globalThis.window = {
    _caduceus: {
      compose: value => { composed = value },
      toast: value => { toast = value },
    },
    open: mock(() => null),
  }
})

afterEach(() => {
  globalThis.window = originalWindow
  globalThis.confirm = originalConfirm
  globalThis.fetch = originalFetch
})

describe('EmailDetail unsubscribe action', () => {
  it('sends an RFC 8058 POST without credentials and reports success', async () => {
    let request
    globalThis.fetch = mock(async (url, options) => {
      request = { url, options }
      return new Response(null, { status: 204 })
    })
    const detail = new EmailDetail()
    detail.$email = {
      unsubscribeUrl: 'https://lists.example.com/unsubscribe/abc',
      unsubscribeOneClick: true,
    }

    await detail.unsubscribe()

    expect(request.url).toBe('https://lists.example.com/unsubscribe/abc')
    expect(request.options).toMatchObject({
      method: 'POST',
      mode: 'no-cors',
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
      body: 'List-Unsubscribe=One-Click',
    })
    expect(detail.$unsubscribeState).toBe('success')
    expect(toast).toBe('Unsubscribe request sent.')
  })

  it('opens a mailto action in the CADUCEUS composer for review', async () => {
    const detail = new EmailDetail()
    detail.$email = {
      unsubscribeMailto: 'mailto:leave@example.com?subject=Remove%20me',
    }

    await detail.unsubscribe()

    expect(composed).toEqual({
      to: ['leave@example.com'],
      subject: 'Remove me',
      body: '',
    })
    expect(detail.$unsubscribeState).toBe('idle')
  })

  it('exposes a retryable error when the one-click request fails', async () => {
    globalThis.fetch = mock(async () => { throw new Error('offline') })
    const detail = new EmailDetail()
    detail.$email = {
      unsubscribeUrl: 'https://lists.example.com/unsubscribe/abc',
      unsubscribeOneClick: true,
    }

    await detail.unsubscribe()

    expect(detail.$unsubscribeState).toBe('error')
    expect(detail.$unsubscribeError).toBe('offline')
    expect(toast).toBe('Could not send the unsubscribe request.')
  })
})

describe('EmailDetail reply threading', () => {
  it('carries the selected Message-ID and complete reference chain into the composer', () => {
    const detail = new EmailDetail()
    detail.$email = {
      sender: 'sender@other.test',
      subject: 'Threaded message',
      rfcMessageId: '<parent@other.test>',
      references: ['<root@other.test>'],
    }

    detail.reply()

    expect(composed).toMatchObject({
      to: 'sender@other.test',
      subject: 'Re: Threaded message',
      inReplyTo: '<parent@other.test>',
      references: ['<root@other.test>', '<parent@other.test>'],
    })
    expect(composed.body).toContain('data-caduceus-quote="reply"')
    expect(composed.body).toContain('sender@other.test wrote:')
  })

  it('bounds a pathological chain while preserving its root and newest ancestors', () => {
    const detail = new EmailDetail()
    detail.$email = {
      rfcMessageId: '<parent@other.test>',
      references: Array.from({ length: 100 }, (_, index) => `<m${index}@other.test>`),
    }
    const metadata = detail.replyThreadMetadata()
    expect(metadata.references).toHaveLength(100)
    expect(metadata.references[0]).toBe('<m0@other.test>')
    expect(metadata.references.at(-1)).toBe('<parent@other.test>')
  })
})

describe('EmailDetail calendar invite presentation', () => {
  it('presents single and multi-day all-day dates without timezone drift', () => {
    const detail = new EmailDetail()
    expect(detail.formatInviteWhen({
      start: '2026-03-01T00:00:00.000Z', end: '2026-03-02T00:00:00.000Z', allDay: true,
    })).toMatch(/Mar 1.*All day/)
    expect(detail.formatInviteWhen({
      start: '2026-03-01T00:00:00.000Z', end: '2026-03-04T00:00:00.000Z', allDay: true,
    })).toMatch(/Mar 1.*Mar 3.*All day/)
  })

  it('distinguishes the current user response from a reply sender response', () => {
    const detail = new EmailDetail()
    expect(detail.inviteDisplayStatus({ myStatus: 'tentative' })).toBe('tentative')
    const reply = { senderResponse: { name: 'Carol', email: 'carol@example.net', status: 'declined' }, myStatus: 'accepted' }
    expect(detail.inviteDisplayStatus(reply)).toBe('declined')
    expect(detail.inviteSenderResponseText(reply)).toBe('Carol declined this invitation.')
    expect(detail.inviteSenderResponseText({ senderResponse: { name: 'Carol', status: 'delegated' } })).toBe('Carol delegated this invitation.')
    expect(detail.inviteSenderResponseText({ senderResponse: { name: 'Carol', status: 'completed' } })).toBe('Carol completed this invitation.')
  })
})
